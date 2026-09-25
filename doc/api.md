# LumiPulse API 文档

**基础信息**

- **Public Base URL**: `/api/v1`
- **Feed Base URL**: `/feed`
- **Admin Base URL**: `/api/v1/admin`
- **格式**: JSON

**状态码**

| 状态码 | 说明 | 典型场景 |
| --- | --- | --- |
| 200 | 请求成功 | 操作成功 |
| 201 | 创建成功 | 资源创建成功 |
| 400 | 请求参数错误 | 缺少必填字段、格式不正确等 |
| 401 | 未授权 | 未携带 Token 或 Token 失效 |
| 403 | 禁止访问 | IP 被封禁、权限不足 |
| 404 | 资源不存在 | 资源不存在场景 |
| 429 | 请求过多 | 登录频繁被限流 |
| 500 | 服务器内部错误 | 未捕获异常、数据库错误等 |

**通用响应格式**

```json
{
  "code": 200,
  "message": "ok",
  "data": { ... }
}
```

**通用约定**

- `/api/*` 与 `/feed/*` 的响应在客户端声明 `Accept-Encoding: gzip` 时会自动 gzip 压缩，
  并返回 `Content-Encoding: gzip` 与 `Vary: Accept-Encoding`。静态资源不在此列。
- 未匹配的 `/api/*` 路径返回 JSON 格式的 `404`（`{"code":404,"message":"Not found"}`），
  不会返回 SPA 的 `index.html`；其余未知路径返回 `index.html` 以支持前端路由。
- `/assets/*` 为带内容哈希的构建产物，返回 `Cache-Control: public, max-age=31536000, immutable`；
  SPA 入口与前端路由路径返回 `Cache-Control: no-cache`。
- 公开总览（`/api/v1/summary`）有 25 秒缓存，但任何服务 / 事件 / 维护计划的写操作
  （含检查器自动创建或解决事件、自动流转维护状态）都会立即让它失效。
- 延迟与每日统计接口的 `days` 会被夹紧到 `HEARTBEAT_RETENTION_DAYS` /
  `DAILY_RETENTION_DAYS`（见 `backend/README.md`），避免请求已被清理的时间范围却看不出原因。
- 月度 SLA 报告（`/api/v1/admin/sla-report`）不受每日明细保留窗口限制：
  已结束的月份会在后台归档到 `ServiceMonthly` 表，即使 `ServiceDaily` 被清理仍可查询。
- `/robots.txt` 与 `/sitemap.xml` 在服务端动态生成（站点名、可收录 URL 都是运行时数据），
  不依赖前端构建产物。
- **公开页面不出现自增 ID**：服务与事件的详情页 URL 分别是
  `/services/:hash` 与 `/incidents/:hash`，均使用随机生成的 `publicHash`。
  响应体中的 `id` / `serviceId` 仅用于前端与本地列表匹配，**不要用于拼接 URL**。
- **服务端生成内容的语言**由配置文件 `LANG`（`zh-CN` / `en-US`）决定，
  影响 `/feed/*` 的标题与字段名、告警邮件、月度 SLA 报告邮件，以及检查器
  自动创建的事件标题与进展文案。前端界面语言由浏览器决定（可在页脚手动切换），
  与 `LANG` 相互独立。
- **管理接口的两种凭据权限不同**：会话 Token（`/admin/login` 获得）代表管理员本人，
  不受限制；API 密钥受 `scope`（`read` / `write`）限制，且**高危操作一律不可用**
  （删除服务 `DELETE /admin/services/:id`、导入覆盖 `POST /admin/import`、
  改管理员账号 `PUT /admin/profile`，用密钥调用返回 403）。
  详见「API 密钥管理」一节。

---

## 公共接口 (Public API)

用于状态页前端展示，无需鉴权。

### 健康检查

```
GET /api/v1/health
```

用于负载均衡和容器编排探针。只表示进程存活（liveness），不检查数据库。

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "status": "healthy",
    "version": "0.1.4"
  }
}
```

---

### 就绪检查

```
GET /api/v1/ready
```

readiness 探针：会真正检查数据库连通性。数据库不可用时返回 **503**，
编排系统应据此停止向该实例导流。

**响应** (200)

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "status": "ready",
    "version": "0.1.4"
  }
}
```

**响应** (503)

```json
{
  "code": 503,
  "message": "database unavailable",
  "data": {
    "status": "unavailable"
  }
}
```

---

### 状态徽章

```
GET /badge/:hash.svg
```

返回 shields.io 风格的 SVG 状态徽章，**无需鉴权**，可直接嵌入 README / 文档站：

```markdown
![status](https://status.example.com/badge/<publicHash>.svg)
```

| 服务状态 | 徽章配色 |
| --- | --- |
| `operational` | 绿色 `#34a761` |
| `degraded` | 橙色 `#fda305` |
| `outage` | 红色 `#df2d2a` |

状态判定与公开状态页一致：活跃事件（`investigating` / `identified`）会覆盖
服务自身的 `status`。`hash` 不存在时返回 404。

**Content-Type**: `image/svg+xml; charset=utf-8`
**Cache-Control**: `public, max-age=60`

---

### 订阅通知

```
POST /api/v1/subscribe
```

使用邮箱订阅服务状态通知，可选指定订阅哪些服务。

**请求**

```json
{
  "email": "user@example.com",
  "services": [1, 2, 3]
}
```

`services` 可选，留空或省略则订阅所有服务。

**响应**

```json
{
  "code": 201,
  "message": "订阅成功"
}
```

邮箱已订阅时返回 200，message 为 `"该邮箱已订阅"`（同时更新订阅的服务列表）。

**投递行为**：订阅立即生效（不做双重确认，`verified` 直接为 1）。服务发生异常 / 恢复时，
除发给 `notify_emails` 外，也会**按订阅的服务列表**发给订阅者：`services` 留空的订阅者
会收到全部服务的通知（含将来新增的服务）。每封订阅者邮件底部都带退订 / 偏好管理链接。

---

### 订阅自助管理（退订 / 修改偏好）

```
GET    /api/v1/subscription?email=...&token=...
PUT    /api/v1/subscription
DELETE /api/v1/subscription?email=...&token=...
```

邮件底部的退订链接指向前端页面 `/unsubscribe?email=...&token=...`，页面通过这三个公开接口
读取偏好、保存偏好或退订。**无需鉴权**，安全性由签名令牌保证：

- 令牌 = `HMAC-SHA256(unsubscribe_secret, 小写邮箱)`，`unsubscribe_secret` 首次使用时自动生成并存库；
- 令牌与邮箱绑定，因此拿到别人的链接也无法退订他人邮箱（跨邮箱使用返回 403）；
- 校验失败统一返回 403，不区分「邮箱不存在」与「签名错误」，避免被用来探测订阅关系。

**查询 / 更新参数**

| 参数 | 位置 | 说明 |
| --- | --- | --- |
| `email` | query（GET/DELETE）/ body（PUT） | 订阅邮箱 |
| `token` | query（GET/DELETE）/ body（PUT） | 退订令牌 |
| `services` | body（PUT） | 订阅的服务 ID 数组；**空数组表示订阅全部服务** |

**响应** (200)

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "email": "user@example.com",
    "services": [1, 2],
    "siteName": "LumiPulse"
  }
}
```

| 状态码 | 说明 |
| --- | --- |
| 400 | 缺少 `email` |
| 403 | 令牌缺失或校验失败 |
| 404 | 该邮箱没有订阅记录（已退订或从未订阅） |
| 500 | 保存 / 删除失败 |

> **邮件里的绝对链接**：退订链接使用设置项 `site_url`（例如 `https://status.example.com`）；
> 未配置时退回 `allow_origin` 的第一个来源；两者都为空时邮件不附带退订区块（不会给出打不开的链接）。

---

### RSS/Atom 订阅源

```
GET /feed/rss
GET /feed/atom
```

返回最近 20 条事件的状态更新订阅源（RSS 2.0 / Atom 1.0 格式），可直接在 RSS 阅读器中订阅。

**Content-Type**: `application/rss+xml` / `application/atom+xml`

无需鉴权。订阅源的标题、描述与字段名（状态 / 影响）按配置文件 `LANG`
的语言渲染，并在 `<language>` / `xml:lang` 中声明对应标签。

---

### 爬虫文件

```
GET /robots.txt
GET /sitemap.xml
```

`robots.txt` 允许收录公开状态页，屏蔽 `/api/`、`/admin`、`/login`、`/setup`，
并在末尾给出 `Sitemap:` 绝对地址。

`sitemap.xml` 由服务端根据运行时数据生成，包含：

- 首页 `/`
- 首页展示中的服务详情 `/services/:hash`
- 最近 50 条事件详情 `/incidents/:hash`

> 站点根地址取自 `X-Forwarded-Host` / `X-Forwarded-Proto`（反向代理后）或请求的 `Host`，
> 因此部署在代理后面时请确保这两个头被正确透传，否则 sitemap 里的 URL 会是内网地址。
> 两个接口都返回 `Cache-Control: public, max-age=3600`。

---

### 获取站点配置

```
GET /api/v1/site-config
```

获取站点名称、图标及邮件通知开关状态。

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "site_name": "LumiPulse",
    "site_icon": "https://example.com/icon.png",
    "email_enabled": "true",
    "show_admin_footer_button": "true",
    "custom_footer": "",
    "sub_enable_email": "true",
    "sub_enable_rss": "true",
    "sub_enable_atom": "true"
  }
}
```

| 字段 | 说明 |
| --- | --- |
| `email_enabled` | SMTP 是否已配置并启用邮件通知 |
| `show_admin_footer_button` | 页脚是否显示"管理后台"链接 |
| `custom_footer` | 自定义页脚 HTML 内容（空则显示默认页脚） |
| `sub_enable_email` | 邮件订阅方式是否开启 |
| `sub_enable_rss` | RSS 订阅方式是否开启 |
| `sub_enable_atom` | Atom 订阅方式是否开启 |

---

### 获取系统总览

```
GET /api/v1/summary
```

获取系统整体健康状况、所有服务状态、当前活跃故障及维护计划。

服务状态由活跃事件的严重程度综合计算得出（`reconcileStatus`）：
- 存在 `identified` 事件 → 服务状态为 `outage`（故障）
- 存在 `investigating` 事件 → 服务状态为 `degraded`（异常）
- 仅 `monitoring` 或 `resolved` 事件 → 服务状态为 `operational`（正常）

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "overallStatus": "operational",
    "services": [
      {
        "id": 1,
        "publicHash": "3f8a1c0d9b2e7f4a6c5d8e1b0a9f3c72",
        "folderId": 2,
        "name": "API 服务",
        "status": "operational",
        "url": "https://api.example.com",
        "uptime": 99.49,
        "interval": 60,
        "certExpiresAt": "2026-12-01T00:00:00Z",
        "homepageBlocks": "metrics,interval,cert,latency,heatmap,history"
      }
    ],
    "folders": [
      {
        "id": 2,
        "name": "核心集群",
        "description": "网关与认证服务",
        "status": "degraded",
        "uptime": 99.12,
        "latency": 86,
        "services": [
          {
            "id": 1,
            "publicHash": "3f8a1c0d9b2e7f4a6c5d8e1b0a9f3c72",
            "folderId": 2,
            "name": "API 服务",
            "status": "operational",
            "url": "https://api.example.com",
            "uptime": 99.49,
            "latency": 42
          }
        ]
      }
    ],
    "activeIncidents": [
      {
        "id": 1,
        "publicHash": "9f2c1d7ab34e56c8f01d2b7a45e9c310",
        "serviceId": 1,
        "title": "API 服务中断",
        "impact": "critical",
        "status": "investigating",
        "createdAt": "2026-05-09T10:00:00Z",
        "updatedAt": "2026-05-09T10:00:00Z",
        "updates": [
          {
            "id": 1,
            "incidentId": 1,
            "status": "investigating",
            "content": "我们正在调查此问题",
            "createdAt": "2026-05-09T10:00:00Z"
          }
        ]
      }
    ],
    "maintenances": [
      {
        "id": 1,
        "title": "数据库升级",
        "description": "主节点版本升级",
        "scheduledStart": "2026-05-15T02:00:00Z",
        "scheduledEnd": "2026-05-15T04:00:00Z",
        "status": "scheduled",
        "affectedServices": "1,2",
        "createdAt": "2026-05-08T00:00:00Z"
      }
    ]
  }
}
```

`overallStatus` 取值：`operational`（正常）、`degraded`（部分故障）、`outage`（严重故障）

> 服务项里的 `certExpiresAt` 仅 HTTPS 服务有值（对端证书到期时间，RFC3339）；
> 纯 HTTP 服务与尚未探测成功时该字段省略，可直接用于展示证书剩余天数。

**首页展示内容（`homepageBlocks`）**

每个服务可以单独配置公开状态页要展示哪些内容块，管理后台始终展示全部内容。
取值是逗号分隔的 key 列表，含义见下表；**空串表示全部展示**（历史数据默认值），
`none` 表示全部不展示（「一个都不显示」不能用空串表达，否则与默认语义冲突）。

| key | 内容 |
| --- | --- |
| `metrics` | 在线率 / 响应时间 |
| `interval` | 探测频率 |
| `cert` | 证书到期 |
| `latency` | 最近 24 小时延迟曲线（含均值 / P95 / P99） |
| `heatmap` | 响应时间热力图 |
| `history` | 服务历史矩阵 |

> 首页列表上的 90 天状态矩阵不受该配置影响，始终展示。


**服务分组（服务聚合文件夹）**

`folders` 把同一分组下的服务融合成一个条目，前端可以用它替代这些服务的独立展示，
展开后再从 `folders[].services`（结构与 `services` 完全一致）读取各服务明细。

| 字段 | 说明 |
| --- | --- |
| `status` | 聚合状态：分组内**全部** `operational` → `operational`，**全部** `outage` → `outage`，其余 → `degraded` |
| `uptime` | 融合可用率，**按探测次数加权**（用每日汇总的 up/down 计数），不是各服务可用率的简单平均 |
| `latency` | 融合响应时间（ms），只统计成功样本并同样按次数加权 |
| `services` | 分组内服务的完整明细 |

可见性规则：

- 分组有独立的 `showOnHomepage` 开关。**分组可见时，其下服务即便自身 `showOnHomepage=false`
  也会随分组展示**（聚合条目需要完整明细）；分组不可见时，其下服务完全不出现在公开接口。
- `services` 里属于可见分组的服务仍会返回（带 `folderId`），未分组的服务保持原有位置与顺序。
- 没有任何可见分组时 `folders` 省略。

---

### 获取服务列表

```
GET /api/v1/services
```

获取所有监控服务的当前状态及在线率。状态同样经过 `reconcileStatus` 计算。
仅包含在首页展示（`showOnHomepage`）的服务。

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "publicHash": "3f8a1c0d9b2e7f4a6c5d8e1b0a9f3c72",
      "name": "API 服务",
      "status": "operational",
      "url": "https://api.example.com",
      "uptime": 99.49
    }
  ]
}
```

`id` 为数据库自增主键，**仅供前端与本地列表匹配，不得用于拼接 URL**；
公开页面的服务详情请使用 `publicHash`。

---

### 获取服务历史

```
GET /api/v1/services/:hash/history?days=90
```

通过 `publicHash` 获取特定服务的历史可用性数据。

**路径参数**

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `hash` | string | 服务的 `publicHash`（不再接受自增 `id`） |

**查询参数**

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `days` | int | 90 | 历史天数，上限为 `HEARTBEAT_RETENTION_DAYS`（默认 30） |

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "service": {
      "id": 1,
      "name": "API 服务",
      "description": "主 API 网关",
      "url": "https://api.example.com",
      "type": "http",
      "interval": 60,
      "status": "operational",
      "isActive": true,
      "sortOrder": 0,
      "certExpiresAt": "2026-12-01T00:00:00Z",
      "createdAt": "2026-01-01T00:00:00Z",
      "updatedAt": "2026-05-09T00:00:00Z"
    },
    "uptime": 99.49,
    "heartbeats": [
      {
        "id": 1,
        "serviceId": 1,
        "status": 200,
        "latency": 120,
        "message": "OK",
        "createdAt": "2026-05-09T00:00:00Z"
      }
    ]
  }
}
```

---

### 获取服务延迟

```
GET /api/v1/services/:hash/latency?days=1
```

通过 `publicHash` 获取服务延迟数据，返回按 5 分钟聚合的紧凑格式。前端可根据 `start` 和 `interval` 还原每个数据点的时间。

聚合在 SQLite 内以 `GROUP BY` 完成，不会把窗口内的原始心跳读进内存。

**查询参数**

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `days` | int | 1 | 天数，上限为 `HEARTBEAT_RETENTION_DAYS`（默认 30，配置文件可调） |

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "start": "2026-05-24T00:00:00Z",
    "interval": 5,
    "latencies": [120, 115, 0, 130],
    "statuses": [0, 0, -1, 1],
    "stats": {
      "samples": 61,
      "avg": 143.31,
      "p95": 219,
      "p99": 260,
      "max": 260
    }
  }
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `start` | string | 起始时间（ISO 8601） |
| `interval` | int | 数据间隔（分钟），固定为 5 |
| `latencies` | int[] | 延迟数组（毫秒），无数据时为 0 |
| `statuses` | int[] | 状态数组，取值见下 |
| `stats` | object \| null | 整个查询窗口的分位数汇总，窗口内没有成功样本时为 `null` |
| `stats.samples` | int | 参与统计的成功样本数 |
| `stats.avg` | float | 平均延迟（毫秒） |
| `stats.p95` / `stats.p99` | float | 95 / 99 分位延迟（毫秒），近邻插值 |
| `stats.max` | int | 窗口内最大延迟（毫秒） |

每个数据点的时间 = `start + index * interval` 分钟。

`statuses` 取值：`0`=正常、`1`=故障、`2`=维护窗口内的故障、`-1`=无数据

> `statuses = 2` 表示该 5 分钟桶里的失败探测发生在维护计划的时间窗口内
> （计划内停机），前端用蓝色与真实故障的红色区分。已取消的维护计划不参与该标记。

> 失败判定：一次探测失败的条件是 **不满足** `200 ≤ status < 400` 或 `status = 1`（TCP 成功）。
> 即 TCP 探测返回 `status = 0` 同样计入故障，与日志页口径一致。

> `stats` 只统计成功样本：失败请求的耗时往往是超时上限，混进分位数会把 p95/p99
> 直接顶到超时值，反而看不出正常请求的长尾。分位数在 SQLite 内用窗口函数一次算出，
> 不会把窗口内的原始心跳搬到 Go 里排序。

---

### 获取响应时间热力图

```
GET /api/v1/services/:hash/latency-heatmap?days=7
```

按「日期 × 小时」（北京时间）聚合平均响应时间，用于服务详情页的热力图。
聚合在 SQLite 内用 `GROUP BY` 完成：7 天原始数据是 2016 个 5 分钟桶，
只下发聚合后的格子（最多 `days × 24` 个）。

**查询参数**

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `days` | int | 7 | 统计天数（含当天），上限为 `HEARTBEAT_RETENTION_DAYS` |

窗口为 `[今天 - (days-1), 今天]`（北京时间，含首尾）。

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "from": "2026-09-13",
    "to": "2026-09-19",
    "days": 7,
    "maxAvg": 300,
    "cells": [
      { "day": "2026-09-19", "hour": 10, "avg": 200, "samples": 2, "failures": 1, "maintenance": true },
      { "day": "2026-09-18", "hour": 23, "avg": 300, "samples": 1, "failures": 0 }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `from` / `to` | string | 窗口首尾日期（`YYYY-MM-DD`，北京时间） |
| `maxAvg` | int | 所有格子的最大平均延迟，前端据此确定色阶上限 |
| `cells` | array | **只含有数据的小时**（稀疏），前端按 `from`~`to` 补齐空格子 |
| `cells[].avg` | int | 该小时内**成功样本**的平均延迟（毫秒） |
| `cells[].samples` | int | 成功样本数；为 `0` 且 `failures > 0` 表示该小时全部失败 |
| `cells[].failures` | int | 该小时内的失败探测次数 |
| `cells[].maintenance` | bool | 该小时是否落在维护窗口内（仅在有影响本服务的维护计划时出现）。为 `true` 时失败探测属于计划内停机，前端用蓝色展示 |

> 成功 / 失败判定与延迟分桶、分位数接口完全一致（成功 = `200 ≤ status < 400` 或 TCP `status = 1`）。
> 失败探测的耗时不计入 `avg`。

> 前端在窄屏（<768px）下会把每小时数据按 3 小时聚合成 8 个格子展示，
> 聚合时按成功样本数加权，并保持 `maintenance` 的或运算结果；接口本身始终按小时返回。

---

### 获取服务每日统计

```
GET /api/v1/services/:hash/daily-stats?days=90
```

通过 `publicHash` 获取特定服务每天的健康检查汇总数据，用于前端矩阵展示。每个元素为 `[upCount, downCount, statusCode, maintenanceCount]` 四元组。

**查询参数**

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `days` | int | 90 | 天数，上限为 `DAILY_RETENTION_DAYS`（默认 90，配置文件可调） |

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "serviceId": 1,
    "publicHash": "3f8a1c0d9b2e7f4a6c5d8e1b0a9f3c72",
    "days": [
      [48, 0, 0, 0],
      [-1, -1, -1, 0],
      [46, 2, 1, 0],
      [40, 0, 0, 5]
    ]
  }
}
```

`statusCode` 取值：`-1`=无数据、`0`=正常、`1`=调查中、`2`=已确认、`3`=监控中、`4`=已解决

`maintenanceCount` 是落在维护计划时间窗口内的**失败探测次数**：这类失败属于计划内停机，
**不计入 `downCount`**（可用率口径不变），前端把它显示为蓝色以便和真实故障的红色区分。
同一天既有真实故障又有维护期停机时，格子按真实故障着色，维护期的部分在悬浮提示里单独列出。
已取消的维护计划不参与该统计。

---

### 批量获取服务每日统计

```
GET /api/v1/daily-stats?days=90
```

一次性返回**所有首页可见服务**的每日统计。首页需要为每个服务渲染状态矩阵，
逐个服务调用上面的接口会产生 N 次串行往返，本接口把它压缩成 1 次请求。

**查询参数**

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `days` | int | 90 | 天数，上限为 `DAILY_RETENTION_DAYS`（默认 90） |

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "days": 90,
    "services": [
      {
        "serviceId": 1,
        "publicHash": "3f8a1c0d9b2e7f4a6c5d8e1b0a9f3c72",
        "days": [
          [48, 0, 0, 0],
          [-1, -1, -1, 0],
          [46, 2, 1, 0]
        ]
      }
    ]
  }
}
```

`serviceId` 为服务的自增 ID（仅用于前端匹配本地服务列表，不作为 URL 使用），
`publicHash` 为可安全用于 URL 的公开标识，`days` 数组格式与单服务接口完全一致
（`[upCount, downCount, statusCode, maintenanceCount]` 四元组）。

---

### 获取故障事件列表

```
GET /api/v1/incidents?page=1&limit=20
```

获取故障事件列表（分页）。

**查询参数**

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `page` | int | 1 | 页码 |
| `limit` | int | 20 | 每页数量（最大 50） |

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "incidents": [
      {
        "id": 1,
        "publicHash": "9f2c1d7ab34e56c8f01d2b7a45e9c310",
        "serviceId": 1,
        "title": "API 服务中断",
        "impact": "critical",
        "status": "resolved",
        "createdAt": "2026-05-08T18:00:00Z",
        "updatedAt": "2026-05-08T18:45:00Z",
        "updates": [
          {
            "id": 1,
            "incidentId": 1,
            "status": "investigating",
            "content": "检测到 API 服务异常，正在调查",
            "createdAt": "2026-05-08T18:00:00Z"
          },
          {
            "id": 2,
            "incidentId": 1,
            "status": "monitoring",
            "content": "已定位问题并实施修复",
            "createdAt": "2026-05-08T18:15:00Z"
          },
          {
            "id": 3,
            "incidentId": 1,
            "status": "resolved",
            "content": "服务已恢复正常",
            "createdAt": "2026-05-08T18:45:00Z"
          }
        ]
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 20,
      "totalPage": 1
    }
  }
}
```

`impact` 取值：`minor`（轻微）、`major`（较大）、`critical`（严重）

`status` 取值：`investigating`（调查中）、`identified`（已确认）、`monitoring`（监控中）、`resolved`（已解决）

`resolvedAt`：事件首次标记为已解决的时间（RFC3339 格式），仅当 `status` 为 `resolved` 时存在。编辑事件不会改变此值。旧数据可能不包含此字段。

`publicHash`：事件的公开访问标识（32 位随机十六进制）。**公开页面必须使用该值而不是自增 `id`**，以避免对外暴露数据库主键与记录规模。

---

### 获取故障事件详情

```
GET /api/v1/incidents/:hash
```

通过 `publicHash` 获取单个故障事件的详情（含对外的更新时间线，内部备注不会返回）。

**路径参数**

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `hash` | string | 事件的 `publicHash`（不再接受自增 `id`） |

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "id": 1,
    "publicHash": "9f2c1d7ab34e56c8f01d2b7a45e9c310",
    "serviceId": 1,
    "title": "API 服务中断",
    "impact": "critical",
    "status": "resolved",
    "affectedServices": "1",
    "resolvedAt": "2026-05-08T18:45:00Z",
    "createdAt": "2026-05-08T18:00:00Z",
    "updatedAt": "2026-05-08T18:45:00Z",
    "rootCause": "数据库连接池被慢查询耗尽",
    "resolution": "扩容连接池并给慢查询补上索引",
    "postmortemUrl": "https://example.com/postmortems/2026-05-08",
    "postmortemPublic": true,
    "updates": []
  }
}
```

> **事后复盘（Postmortem）**：`rootCause` / `resolution` / `postmortemUrl` 三个字段仅在
> 该事件勾选了「对外公开复盘」（`postmortemPublic: true`）时才会返回；未勾选时后端会
> 把这三个字段清空（本接口、`/incidents` 列表、`/summary` 的活跃事件都一致处理），
> 因此内部根因描述不会因为接口疏漏外泄。前端只需判断这三个字段是否存在即可决定是否渲染。
> `postmortemPublic` 本身总会返回，便于前端展示「仅内部可见」的提示。

`hash` 不存在时返回 `404`。

---

### 获取维护计划

```
GET /api/v1/maintenances
```

获取当前进行中和计划中的维护任务。

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "title": "数据库版本升级",
      "description": "主节点版本升级",
      "scheduledStart": "2026-05-15T02:00:00Z",
      "scheduledEnd": "2026-05-15T04:00:00Z",
      "status": "scheduled",
      "affectedServices": "1,2",
      "createdAt": "2026-05-08T00:00:00Z"
    }
  ]
}
```

`status` 取值：`scheduled`（计划中）、`in_progress`（进行中）、`completed`（已完成）、`cancelled`（已取消）

---

## 管理接口 (Admin API)

用于后台管理，需携带 `Authorization: Bearer <token>` 头。支持两种认证方式：

- **会话 Token**: 通过登录接口获取，有效期 20 分钟
- **API Key**: 永久有效（或按过期时间），可在密钥管理页面创建

---

### 首次设置

```
POST /api/v1/admin/setup
```

首次部署时设置管理员用户名和密码（仅在默认账户状态下可用）。

**请求**

```json
{
  "username": "admin",
  "password": "password"
}
```

**响应**

```json
{
  "code": 200,
  "message": "Setup completed, please login again"
}
```

---

### 登录

```
POST /api/v1/admin/login
```

**请求**

```json
{
  "username": "admin",
  "password": "password"
}
```

**响应**

```json
{
  "code": 200,
  "message": "Login successful",
  "data": {
    "token": "xxxxx",
    "needsSetup": false
  }
}
```

`needsSetup` 指示是否为默认账户，用于前端判断是否需要跳转设置页。

Token 有效期为 20 分钟，需在 `Authorization: Bearer <token>` 头中传递。

---

### 控制台概览

```
GET /api/v1/admin/stats
```

获取控制台仪表盘所需的统计数据。

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "totalServices": 5,
    "operationalCount": 3,
    "degradedCount": 1,
    "outageCount": 1,
    "activeIncidents": 2,
    "activeMaintenances": 1,
    "services": [
      {
        "id": 1,
        "name": "API 服务",
        "status": "operational",
        "url": "https://api.example.com",
        "type": "http",
        "uptime": 99.49,
        "latency": 120,
        "interval": 60
      }
    ],
    "recentIncidents": [
      { "id": 1, "serviceId": 1, "title": "...", "impact": "critical", "status": "investigating", "updates": [...] }
    ]
  }
}
```

注：`activeIncidents` 计数已排除 `monitoring` 状态的事件（仅统计 `investigating` 和 `identified`）。

---

### 批量获取服务每日统计（管理用）

```
GET /api/v1/admin/daily-stats?days=90
```

与公开接口 `GET /api/v1/daily-stats` 格式一致，但**包含未在首页展示的服务**。
管理端首页需要为每个服务渲染状态矩阵，逐个服务请求会产生 N 次串行往返。

**查询参数**

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `days` | int | 90 | 天数，上限为 `DAILY_RETENTION_DAYS`（默认 90） |

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "days": 90,
    "services": [
      { "serviceId": 1, "days": [[48, 0, 0], [-1, -1, -1]] }
    ]
  }
}
```

---

### 月度 SLA 报告

按月汇总可用率、事件与响应时间，供管理端「SLA 报告」页使用。

```
GET /api/v1/admin/sla-report?month=2026-05
GET /api/v1/admin/sla-report/trend?months=6
```

**查询参数（sla-report）**

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `month` | string | 当前月 | 月份，格式 `YYYY-MM` |

**查询参数（sla-report/trend）**

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `months` | int | 6 | 趋势窗口月数，上限 24 |

**响应（sla-report）**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "month": "2026-05",
    "label": "2026 年 05 月",
    "days": 31,
    "final": true,
    "summary": {
      "uptime": 99.87,
      "totalProbes": 8640,
      "downtimeProbes": 11,
      "incidents": 2,
      "downtimeSeconds": 5400,
      "avgLatency": 138.4
    },
    "services": [
      {
        "serviceId": 1,
        "publicHash": "9f2c…",
        "name": "API 服务",
        "uptime": 99.94,
        "totalProbes": 4320,
        "downtimeProbes": 3,
        "incidents": 1,
        "downtimeSeconds": 3600,
        "avgLatency": 120.5,
        "coveredDays": 31
      }
    ]
  }
}
```

**响应（sla-report/trend）**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "months": [
      { "month": "2026-04", "label": "2026 年 04 月", "uptime": 99.99, "incidents": 0, "avgLatency": 121.2, "totalProbes": 8640 }
    ],
    "retainedDays": 90
  }
}
```

| 字段 | 说明 |
| --- | --- |
| `final` | 月份已结束且数据已固化；为 `false` 表示当月数据仍在累积 |
| `uptime` | 可用率，按探测次数加权（成功探测 / 总探测 × 100） |
| `downtimeSeconds` | 事件造成的不可用时长（秒），跨月事件只计入落在本月的那一段 |
| `coveredDays` | 该月内有探测数据的天数，为 0 表示该月没有数据（前端显示「无数据」） |
| `retainedDays` | 每日明细保留天数，超出该范围的月份依赖已固化的月报 |

> **留存策略**：`ServiceDaily` 会被 `DAILY_RETENTION_DAYS`（默认 90）清理，
> 月报需要长期可查，因此已结束月份会在后台归档到 `ServiceMonthly` 表：
> 检查器每天清理前，以及首次查询某个已结束月份时，都会把当月数据固化下来
> （`final: true`）。之后即使每日明细被删除，月报仍可查询。
> 归档最多回溯 6 个月，趋势最多 24 个月。
>
> 平均延迟按全部探测次数均摊 `total_latency`（`ServiceDaily` 的累计口径，
> 成功与失败的探测耗时都计入，其中失败探测通常等于超时上限）。
> 如果只想看成功请求的长尾，请使用延迟接口的 `stats`（只统计成功样本的分位数）。

---

### 监控日志

```
GET /api/v1/admin/logs?page=1&limit=50&serviceId=0&status=all
```

获取健康检查日志记录（分页）。

**查询参数**

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `page` | int | 1 | 页码 |
| `limit` | int | 50 | 每页数量（最大 200） |
| `serviceId` | int | 0 | 按服务筛选（0=全部） |
| `status` | string | "all" | 筛选：`all`、`success`、`failure`。`success`/`failure` 按各服务自己的判定口径过滤（期望状态码 + 期望关键字），与服务探测时的结论一致 |

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "logs": [
      {
        "id": 1,
        "serviceId": 1,
        "serviceName": "API 服务",
        "status": 200,
        "latency": 120,
        "message": "HTTP 200",
        "isSuccess": true,
        "createdAt": "2026-05-09T00:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 50,
      "totalPage": 10
    }
  }
}
```

> `isSuccess` 由后端按服务的期望状态码与期望关键字给出，前端直接使用即可。
> `message` 会带上状态码与截断后的响应体片段（最多 200 字符），失败时额外说明原因
> （不在期望状态码范围内 / 响应内容未包含期望关键字），便于排查关键字不匹配的问题。

---

### 系统设置

#### 获取设置

```
GET /api/v1/admin/settings
```

**响应**

```json
{
  "code": 200,
  "message": "Settings fetched",
  "data": {
    "site_name": "LumiPulse",
    "site_icon": "",
    "admin_email": "",
    "admin_name": "admin",
    "allow_origin": "http://localhost:5173",
    "site_url": "https://status.example.com",
    "smtp_host": "",
    "smtp_port": "",
    "smtp_user": "",
    "smtp_encryption": "",
    "email_enabled": "true",
    "notify_services": "",
    "notify_emails": "",
    "alert_cooldown_minutes": "",
    "quiet_hours_start": "",
    "quiet_hours_end": "",
    "alert_escalation_enabled": "false",
    "alert_escalation_minutes": "15",
    "alert_escalation_repeat_minutes": "",
    "webhook_enabled": "false",
    "webhook_type": "generic",
    "webhook_url": "",
    "webhook_secret": "",
    "webhook_events": "down,up",
    "webhook_telegram_chat_id": "",
    "sla_report_enabled": "false",
    "sla_report_emails": "",
    "sla_report_language": "",
    "weekly_report_enabled": "false",
    "weekly_report_emails": "",
    "weekly_report_weekday": "1",
    "last_sla_report_month": "",
    "weekly_report_last_sent": "",
    "show_admin_footer_button": "true",
    "custom_footer": "",
    "sub_enable_email": "true",
    "sub_enable_rss": "true",
    "sub_enable_atom": "true"
  }
}
```

#### 更新设置

```
PUT /api/v1/admin/settings
```

**请求**

```json
{
  "site_name": "LumiPulse",
  "email_enabled": "true",
  "allow_origin": "http://localhost:5173,https://status.example.com"
}
```

只接受白名单内的键，出现未知键会整请求返回 400（`Setting "xxx" is not allowed`）。
`admin_password` / `smtp_pass` / `webhook_secret` 属敏感项：读取时返回空字符串，
提交时空值表示「保持原值不变」。`last_sla_report_month` 与 `weekly_report_last_sent`
是服务端自己维护的去重标记，可读但会被写入请求忽略。

#### 报告邮件（月报 / 周报）

在「通知管理 → 月度 SLA 报告邮件 / 周报摘要邮件」中配置：

| 设置项 | 说明 |
| --- | --- |
| `sla_report_enabled` | 是否发送月度 SLA 报告 |
| `sla_report_emails` | 月报收件邮箱（逗号分隔）；留空则复用 `notify_emails` |
| `sla_report_language` | 报告语言：`zh-CN` / `en-US`，留空跟随服务端 `LANG`。**月报与周报共用** |
| `weekly_report_enabled` | 是否发送周报摘要 |
| `weekly_report_emails` | 周报收件邮箱（逗号分隔）；留空则复用 `notify_emails` |
| `weekly_report_weekday` | 周报发送日：`1`=周一 … `7`=周日，默认 `1` |

触发与统计口径：

- 月报在检测到「进入新月份」时发送上一个自然月的报告；周报在配置的星期几发送，
  统计窗口固定为**发送日往前 7 天**（即截止到昨天，不含发送当天）。
- 两者都由检查器的每日任务驱动，并各自记录去重标记（`last_sla_report_month` /
  `weekly_report_last_sent`），**同一天内重启进程不会重复发送**；发送失败不记录标记，
  下个周期会重试。
- 收件人为空、窗口内没有任何探测数据时直接跳过（日志中会看到跳过原因）。
- 周报与月报共用同一套版式与统计口径（可用率按探测次数加权），周报额外附带
  「较上周：可用率 ±x.xxx%，事件 ±n」的环比行；上一周期无数据时不展示环比。

#### 通知模板

在「通知管理 → 通知模板」中配置。四类通知各自可自定义**主题**与**正文**，留空则使用内置的
中/英文案（跟随 `LANG`）。**邮件与 webhook 共用同一份渲染结果**（webhook 取正文作为 `message`）。

| 设置项 | 说明 |
| --- | --- |
| `tpl_alert_subject` / `tpl_alert_body` | 服务异常通知 |
| `tpl_resolved_subject` / `tpl_resolved_body` | 服务恢复通知 |
| `tpl_maintenance_subject` / `tpl_maintenance_body` | 维护计划即将开始提醒 |
| `tpl_cert_subject` / `tpl_cert_body` | HTTPS 证书即将到期提醒 |

变量写成 `{{name}}`（两侧空格可有可无）：

| 通知类型 | 可用变量 |
| --- | --- |
| 异常 | `{{service}}` `{{url}}` `{{time}}` `{{site}}` |
| 恢复 | `{{service}}` `{{url}}` `{{duration}}` `{{time}}` `{{site}}` |
| 维护提醒 | `{{title}}` `{{start}}` `{{countdown}}` `{{site}}` |
| 证书到期 | `{{service}}` `{{expires}}` `{{remaining}}` `{{site}}` |

渲染规则：

- 模板按**纯文本**处理，不能写 HTML：写入邮件时会先做 HTML 转义，再把换行转成 `<br>`，
  因此变量值（服务名、URL 等）不会破坏邮件结构，也不存在注入问题。
- 已知变量按其值替换（值为空即替换为空）；**未知变量原样保留**，便于发现拼写错误。
- 恢复通知中 `{{duration}}` 在无可用时长时为空字符串。

#### 告警静默与免打扰

在「通知管理 → 告警静默与免打扰」中配置：

| 设置项 | 说明 |
| --- | --- |
| `alert_cooldown_minutes` | 同一服务的告警冷却窗口（分钟，1~1440）。窗口内重复产生的异常事件仍会创建，但**只通知一次**；留空或 `0` 表示不限制 |
| `quiet_hours_start` / `quiet_hours_end` | 免打扰时段（`HH:mm`，北京时间）。两者都填写才生效，`start > end` 表示跨零点（如 `23:00` - `07:00`）；起止相同视为不启用 |

行为约定：

- 免打扰时段内**不发告警邮件**（异常与恢复都不发），**webhook 照常投递**——夜间仍能通过群机器人/手机收到，只是不往邮箱堆。
- `critical` 级别（事件 `impact`）的异常**穿透免打扰**，无论何时都发邮件。自动创建的事件目前固定为 `major`。
- 冷却窗口只作用于「异常」通知；恢复通知始终发送，避免出现「恢复了却没人知道」。
- 维护计划提前提醒与证书到期提醒属于一次性通知，**不受免打扰影响**（否则会被丢掉且不再补发）。
- 维护窗口内的服务不会自动创建事件（检查器在 `trackProbeState` 里已跳过），因此也不会产生告警。

#### 告警升级

在「通知管理 → 告警升级」中配置。事件创建后如果一直没人确认，超过时限会再次通知，
避免告警被忽略；在「事件管理」里点击事件行的「未确认」即可确认（见下方 `acknowledged` 字段）。

| 设置项 | 说明 |
| --- | --- |
| `alert_escalation_enabled` | 是否启用告警升级（默认 `false`）。关闭时完全不扫描，不会产生任何升级通知 |
| `alert_escalation_minutes` | 升级时限（分钟，1~10080，默认 15）。事件创建后超过该时长仍未确认则升级通知 |
| `alert_escalation_repeat_minutes` | 重复升级间隔（分钟，0~10080）。留空或 `0` 表示**只升级一次**；填写后会在事件仍未确认时按该间隔反复提醒 |

行为约定：

- 只扫描**未解决且未确认**的顶层事件（`parent_id IS NULL`）；合并事件的子事件不单独升级。
- 升级通知走邮件 + webhook + 订阅者，文案模板为 `tpl_escalation_subject` / `tpl_escalation_body`。
- 升级邮件同样受免打扰时段影响，`critical` 级别事件穿透；webhook 不受影响。
- 每次升级会把 `escalationCount + 1` 并记录 `lastEscalatedAt`，因此**重启服务不会补发历史升级**。
- 取消确认会重置 `escalationCount` 与 `lastEscalatedAt`，让事件重新走一遍未确认 → 升级的流程。

---

### 管理员账号

#### 获取当前用户

```
GET /api/v1/admin/current-user
```

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "username": "admin"
  }
}
```

#### 修改个人资料

```
PUT /api/v1/admin/profile
```

**请求**

```json
{
  "oldPassword": "current_password",
  "newUsername": "new_admin",
  "newPassword": "new_password"
}
```

`newUsername` 和 `newPassword` 可选，留空则不修改对应项。

**响应**

```json
{
  "code": 200,
  "message": "已更新"
}
```

---

### 测试邮件

```
POST /api/v1/admin/test-email
```

发送测试邮件验证 SMTP 配置。

**请求**

```json
{
  "to": "admin@example.com"
}
```

**响应**

```json
{
  "code": 200,
  "message": "测试邮件发送成功"
}
```

---

### 测试 Webhook

```
POST /api/v1/admin/test-webhook
```

使用**已保存的** webhook 设置发送一条测试消息（服务名固定为「LumiPulse 测试通知」，
事件类型为 `down`），用于验证地址、渠道格式与签名配置。前端需先保存再测试。

**响应**

```json
{
  "code": 200,
  "message": "测试 webhook 发送成功"
}
```

地址未配置时返回 400；接收端返回非 2xx 或请求失败时返回 500，message 中带上游返回的摘要。

#### Webhook 通知配置

在「系统设置 → 通知管理」中配置，对应以下设置项：

| 设置项 | 说明 |
| --- | --- |
| `webhook_enabled` | 是否启用，默认 `false` |
| `webhook_type` | 渠道：`generic`（默认）/ `slack` / `discord` / `telegram` |
| `webhook_url` | 接收地址。Telegram 为 `https://api.telegram.org/bot<token>/sendMessage` |
| `webhook_secret` | 仅 `generic`：启用 HMAC-SHA256 签名 |
| `webhook_events` | 订阅的事件，逗号分隔：`down`（异常）/ `up`（恢复，正文带本次不可用时长）/ `maintenance`（维护计划即将开始提醒，见 `MAINTENANCE_REMIND_MINUTES`）/ `cert_expiring`（HTTPS 证书剩余 < 30 天、< 7 天）。留空视为都订阅 |
| `webhook_telegram_chat_id` | 仅 `telegram`：目标会话 ID |

> `webhook_secret` 与 `smtp_pass` 属于敏感项：`GET /admin/settings` 只返回空字符串，
> `PUT` 时传空值表示「保持原值不变」，不会被清空。

**请求体形状**

- `generic`：

```json
{
  "event": "down",
  "service": "API 服务",
  "url": "https://api.example.com/health",
  "message": "服务 API 服务 (...) 连续检测失败，已自动创建故障事件。\n\n检测时间: ...",
  "timestamp": "2026-05-09T00:00:00Z"
}
```

- `slack`：`{"text": "...", "attachments": [{"color": "#df2d2a", "title": "...", "text": "...", "fields": [...]}]}`
- `discord`：`{"embeds": [{"title": "...", "description": "...", "color": 14626090, "timestamp": "...", "fields": [...]}]}`
- `telegram`：`{"chat_id": "...", "text": "...", "parse_mode": "Markdown"}`

**签名校验（仅 generic）**

配置 `webhook_secret` 后，请求会额外带上两个头：

| 请求头 | 说明 |
| --- | --- |
| `X-LumiPulse-Timestamp` | Unix 时间戳（秒） |
| `X-LumiPulse-Signature` | `sha256=<hex>`，见下 |

签名算法：

```
signature = HMAC-SHA256(secret, timestamp + "." + raw_request_body)
```

接收端伪代码：

```python
expected = hmac.new(secret.encode(), f"{ts}.".encode() + raw_body, hashlib.sha256).hexdigest()
if not hmac.compare_digest(expected, signature.removeprefix("sha256=")):
    reject()
if abs(time.time() - int(ts)) > 300:   # 建议校验时间窗口，防重放
    reject()
```

> 时间戳纳入签名，因此只有同时拿到 secret 才能伪造请求；建议接收端额外校验时间窗口。
> 每次投递的超时上限为 10 秒，且投递在探测协程内同步进行，接收端过慢会拖慢探测节奏。
> `X-LumiPulse-Event` 头会带上事件类型（`down` / `up`），方便接收端在解析请求体前分流。

#### 月度 SLA 报告邮件配置

在「系统设置 → 通知管理」中配置。启用后，服务端会在检测到「进入新月份」时
（每日清理任务里判断一次）把**上一个自然月**的报告发给指定邮箱，内容与
`GET /api/v1/admin/sla-report` 一致。

| 设置项 | 说明 |
| --- | --- |
| `sla_report_enabled` | 是否启用，默认 `false` |
| `sla_report_emails` | 收件邮箱，逗号分隔；**留空则复用 `notify_emails`** |
| `sla_report_language` | 报告语言：留空跟随配置文件 `LANG`，也可固定 `zh-CN` / `en-US` |
| `last_sla_report_month` | 只读，记录最近一次发送的月份（`YYYY-MM`），用于避免重复投递 |

> 发送失败不会写入 `last_sla_report_month`，下一个清理周期会自动重试。
> 上一个月完全没有探测数据时会跳过（不会发空报告）。
> 发送依赖 SMTP 配置，且不受 `email_enabled` 影响（该开关只控制告警邮件）。

---

### 服务管理

#### 创建服务

```
POST /api/v1/admin/services
```

**请求**

```json
{
  "name": "API 服务",
  "description": "主 API 网关",
  "url": "https://api.example.com/health",
  "type": "http",
  "interval": 60,
  "timeoutSeconds": 10,
  "sortOrder": 0,
  "showOnHomepage": true,
  "insecureSkipVerify": false,
  "httpMethod": "POST",
  "httpHeaders": "{\"Authorization\":\"Bearer xxx\",\"Accept\":\"application/json\"}",
  "httpBody": "{\"probe\":true}",
  "expectStatus": "200-299",
  "expectKeyword": "\"status\":\"ok\"",
  "homepageBlocks": "metrics,interval,cert,latency,heatmap,history"
}
```

`type` 取值：`http`（默认）、`tcp`、`ping`

| 字段 | 说明 |
| --- | --- |
| `interval` | 探测间隔（秒），取值范围 **10 ~ 3600**，默认 60。检查器按该值调度，不再是固定 1 分钟 |
| `timeoutSeconds` | 单次探测超时（秒），取值范围 **1 ~ 300**，默认 10（传 `0` 或省略即用默认值）。慢接口调大，避免被误判为故障 |
| `showOnHomepage` | 是否在公开状态页展示 |
| `insecureSkipVerify` | 仅对该服务跳过 HTTPS 证书校验（默认 `false`），用于自签证书的内网服务。优先使用它，而不是全局的 `INSECURE_SKIP_VERIFY` |
| `httpMethod` | HTTP 请求方法，支持 `GET`/`HEAD`/`POST`/`PUT`/`PATCH`/`DELETE`/`OPTIONS`，**留空等价于 `GET`**（保证向后兼容）。仅 `type=http` 生效 |
| `httpHeaders` | 自定义请求头，**JSON 对象字符串**（如 `{"Authorization":"Bearer xxx"}`）。格式非法时接口返回 400；默认自动带 `User-Agent: LumiPulse`，可被同名自定义头覆盖 |
| `httpBody` | 请求体纯文本，配合 `POST`/`PUT` 的健康检查接口使用；仅 `http` 类型且方法允许携带正文时发送 |
| `expectStatus` | 期望状态码模式，支持单值 `200`、列表 `200,301`、区间 `200-299`、通配 `2xx`（逗号分隔可混用）。**留空按默认 `200 ≤ status < 400` 判定** |
| `expectKeyword` | 期望响应关键字。非空时响应体必须包含该字符串才算成功，用于识别「返回 200 但内容是错误页」；留空表示不校验内容。上限 200 字符 |
| `homepageBlocks` | 公开状态页要展示的内容块，逗号分隔（`metrics` / `interval` / `cert` / `latency` / `heatmap` / `history`）。**省略 = 保持原值（创建时即全部展示）**，传空串 = 全部不展示（落库 `none`），传列表 = 只展示列出的内容块。只影响公开页面，管理后台始终展示全部 |

> **成功判定统一口径**：一次 HTTP 探测成功 = 状态码符合 `expectStatus`（或默认 200~399）
> **且**（若配置了）响应体包含 `expectKeyword`。心跳统计、每日统计、延迟分桶、热力图与
> 延迟分位数全部使用同一判定，监控日志接口还会在每条日志上返回 `isSuccess` 字段，
> 前端不再自行按状态码猜测。
>
> **敏感信息**：`httpHeaders` / `httpBody` 属于敏感信息，管理端读接口（列表、创建/更新响应）
> 与数据导出**一律返回空串**，不会回显已保存的值。写入时空值表示「保持原值」，
> 因此编辑表单留空提交不会覆盖已保存的请求头/请求体。相应地，导出文件重新导入后
> 这两个字段为空，需要重新填写。

> **HTTPS 证书监控**：HTTPS 服务探测成功时，检查器会把对端叶子证书的到期时间写回
> `certExpiresAt`（RFC3339，UTC），公开总览 / 服务接口都会返回该字段；纯 HTTP 服务与
> TCP 探测（不做 TLS 握手）不会写回，字段省略。剩余 **< 30 天**、**< 7 天**（含已过期）
> 各发送一次通知（邮件 + webhook `cert_expiring`），同一等级不会重复提醒，证书续期后重新计数。

**响应** (201)

```json
{
  "code": 201,
  "message": "Service created",
  "data": {
    "id": 1,
    "name": "API 服务",
    "description": "主 API 网关",
    "url": "https://api.example.com/health",
    "type": "http",
    "interval": 60,
    "timeoutSeconds": 10,
    "status": "operational",
    "isActive": true,
    "sortOrder": 0,
    "showOnHomepage": true,
    "insecureSkipVerify": false,
    "homepageBlocks": "metrics,interval,cert,latency,heatmap,history",
    "createdAt": "2026-05-09T00:00:00Z",
    "updatedAt": "2026-05-09T00:00:00Z"
  }
}
```

#### 获取服务列表（管理用）

```
GET /api/v1/admin/services
```

返回含在线率和延迟的详细服务列表。

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "name": "API 服务",
      "description": "主 API 网关",
      "url": "https://api.example.com",
      "type": "http",
      "interval": 60,
      "status": "operational",
      "isActive": true,
      "sortOrder": 0,
      "httpMethod": "POST",
      "httpHeaders": "",
      "httpBody": "",
      "expectStatus": "200-299",
      "expectKeyword": "\"status\":\"ok\"",
      "homepageBlocks": "metrics,interval,cert,latency,heatmap,history",
      "createdAt": "...",
      "updatedAt": "...",
      "uptime": 99.49,
      "latency": 120
    }
  ]
}
```

> `httpHeaders` / `httpBody` 恒为空串（敏感信息不回显），其余探测配置原样返回。

#### 更新服务

```
PUT /api/v1/admin/services/:id
```

**请求**

```json
{
  "name": "API 服务 v2",
  "url": "https://api-v2.example.com/health",
  "status": "operational",
  "isActive": true,
  "httpMethod": "GET",
  "expectStatus": "",
  "expectKeyword": "",
  "httpHeaders": "",
  "httpBody": ""
}
```

探测高级匹配字段的写入语义：

| 字段 | 未传（省略） | 传空串 | 传非空值 |
| --- | --- | --- | --- |
| `httpMethod` | 保持原值 | 归一化为 `GET` | 使用传入值 |
| `expectStatus` | 保持原值 | 清空，回到默认 200~399 判定 | 使用传入值（格式非法返回 400） |
| `expectKeyword` | 保持原值 | 清空，不校验响应内容 | 使用传入值 |
| `httpHeaders` | 保持原值 | **保持原值**（敏感字段不回显，留空即不变） | 使用传入值（JSON 非法返回 400） |
| `httpBody` | 保持原值 | **保持原值**（同上） | 使用传入值 |
| `folderId` | 保持原值 | **清空分组**（回到未分组） | 归入指定分组（分组不存在返回 400） |
| `homepageBlocks` | 保持原值 | **全部不展示** | 只展示列出的内容块（未知 key 自动忽略；全是不认识的 key 时等同空串） |

> 创建服务时的 `folderId` 传 `0` 或省略即「未分组」。
> `homepageBlocks` 省略时保持原值，创建时即「全部展示」。

#### 服务分组（服务聚合文件夹）

服务分组把多个服务聚合在一起，公开首页把它们融合成一个条目展示；点击展开后再看各服务明细
（融合信息消失）。分组只影响展示口径，探测、事件与 SLA 统计仍然按服务计算。

```
GET    /api/v1/admin/service-folders          # 列表（带 serviceCount）
GET    /api/v1/admin/service-folders/summary  # 各分组的融合结果（含未在首页展示的分组）
POST   /api/v1/admin/service-folders          # 创建
PUT    /api/v1/admin/service-folders/:id      # 修改
DELETE /api/v1/admin/service-folders/:id      # 删除（服务不会被删除，只是变回未分组）
PUT    /api/v1/admin/services/:id/folder      # 把服务移动到分组
```

创建 / 修改请求：

```json
{
  "name": "核心集群",
  "description": "网关与认证服务",
  "showOnHomepage": true,
  "sortOrder": 0
}
```

| 字段 | 说明 |
| --- | --- |
| `name` | 分组名称（必填，≤100 字符） |
| `showOnHomepage` | 是否在公开首页展示。关闭后该分组及其下所有服务都不会出现在公开首页；创建时省略默认 `true` |
| `sortOrder` | 排序值（升序，同值按 ID） |

修改接口的 `name` / `description` / `showOnHomepage` / `sortOrder` 均为**指针语义**：不传表示不修改。

移动服务到分组：

```json
{ "folderId": 2 }
```

`folderId` 传 `null` 或 `0` 表示取消分组；分组不存在返回 400。
`GET /admin/services` 返回的服务对象会带 `folderId` 与 `folderName`，可直接用于表单回显。

`GET /admin/service-folders/summary` 的响应结构与公开总览的 `folders` 完全一致
（融合口径见「状态总览」一节）。

#### 删除服务

```
DELETE /api/v1/admin/services/:id
```

**响应**

```json
{
  "code": 200,
  "message": "Service deleted"
}
```

#### 服务排序

```
PUT /api/v1/admin/services/reorder
```

批量更新服务的排序顺序。

**请求**

```json
{
  "services": [
    { "id": 1, "sortOrder": 0 },
    { "id": 3, "sortOrder": 1 },
    { "id": 2, "sortOrder": 2 }
  ]
}
```

**响应**

```json
{
  "code": 200,
  "message": "Services reordered"
}
```

---

### 事件与更新管理

#### 创建故障事件

```
POST /api/v1/admin/incidents
```

创建故障事件时会自动更新关联服务的状态：
- `critical` → 服务状态设为 `outage`
- `major` / `minor` → 服务状态设为 `degraded`

**请求**

```json
{
  "serviceId": 1,
  "title": "API 服务响应超时",
  "impact": "critical",
  "status": "investigating"
}
```

#### 获取事件列表（管理用）

```
GET /api/v1/admin/incidents?page=1&limit=20
```

同公共接口 `/api/v1/incidents`，返回格式一致。事件对象同时包含 `publicHash`，可用于拼接公开详情页地址。

#### 获取事件详情（管理用）

```
GET /api/v1/admin/incidents/:id
```

返回指定事件的完整信息，包含所有进展更新（含内部记录）和子事件列表。管理接口仍使用自增 `id`；公开页面请改用 `/api/v1/incidents/:hash`。

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "id": 1,
    "publicHash": "9f2c1d7ab34e56c8f01d2b7a45e9c310",
    "serviceId": 1,
    "title": "API 服务中断",
    "impact": "critical",
    "status": "investigating",
    "affectedServices": "1,2,3",
    "parentId": null,
    "createdAt": "2026-05-08T18:00:00Z",
    "updatedAt": "2026-05-08T18:45:00Z",
    "updates": [
      { "id": 1, "incidentId": 1, "status": "investigating", "content": "...", "isInternal": false, "createdAt": "..." }
    ],
    "children": [
      { "id": 2, "serviceId": 2, "title": "子事件", "impact": "major", "status": "identified", ... }
    ]
  }
}
```

`children` 为已合并到该事件的子事件列表。`updates` 中包含 `isInternal` 字段，标记是否为系统内部记录。

---

#### 添加事件进展更新

```
POST /api/v1/admin/incidents/:id/updates
```

当 `status` 设置为 `resolved` 时，会自动将对应事件标记为已解决，并将关联服务状态恢复为 `operational`。

**请求**

```json
{
  "status": "monitoring",
  "content": "已定位问题并实施修复，正在监控恢复情况"
}
```

#### 更新事件进展

```
PUT /api/v1/admin/incidents/:id/updates/:updateId
```

**请求**

```json
{
  "status": "monitoring",
  "content": "已更新进展内容"
}
```

**响应**

```json
{
  "code": 200,
  "message": "Incident update modified"
}
```

#### 删除事件进展

```
DELETE /api/v1/admin/incidents/:id/updates/:updateId
```

**响应**

```json
{
  "code": 200,
  "message": "Incident update deleted"
}
```

---

#### 更新事件

```
PATCH /api/v1/admin/incidents/:id
```

**请求**

```json
{
  "impact": "major",
  "status": "resolved",
  "title": "API 服务中断（已恢复）",
  "rootCause": "数据库连接池被慢查询耗尽",
  "resolution": "扩容连接池并给慢查询补上索引",
  "postmortemUrl": "https://example.com/postmortems/2026-05-08",
  "postmortemPublic": true,
  "acknowledged": true
}
```

| 字段 | 说明 |
| --- | --- |
| `title` / `impact` / `status` | 与历史行为一致：**空字符串表示不修改** |
| `rootCause` / `resolution` / `postmortemUrl` | 事后复盘内容。与上面三个字段不同，这几个字段是**指针语义**：传空字符串表示清空，不传表示保持原值 |
| `postmortemPublic` | 是否在公开页面展示复盘内容（默认 `false`，即仅内部可见） |
| `acknowledged` | 人工确认开关（指针语义）。置 `true` 记录 `acknowledgedAt` / `acknowledgedBy` 并**停止告警升级**；置 `false` 会清空确认信息并重置升级计数，重新进入未确认流程 |

当 `status` 设为 `resolved` 时，自动恢复关联服务状态为 `operational`。

确认 / 取消确认都会在同一事务之外追加一条 `status=system` 的进展记录，用于回溯谁在何时接手。

**事件对象新增字段**

| 字段 | 说明 |
| --- | --- |
| `acknowledged` | 是否已被人工确认 |
| `acknowledgedAt` / `acknowledgedBy` | 确认时间与确认人（单管理员模型下为管理员用户名）。公开接口一律清空这两个字段 |
| `escalationCount` | 已发送的升级通知次数 |
| `lastEscalatedAt` | 上次升级通知时间（RFC3339） |

#### 删除事件

```
DELETE /api/v1/admin/incidents/:id
```

删除未解决事件时自动恢复关联服务状态为 `operational`。

**响应**

```json
{
  "code": 200,
  "message": "Incident deleted"
}
```

---

#### 合并事件

```
POST /api/v1/admin/incidents/:id/merge
```

将指定事件合并为当前事件的子事件。被合并的事件状态会跟随主事件同步变更。

**请求**

```json
{
  "sourceId": 2
}
```

#### 拆分事件

```
POST /api/v1/admin/incidents/:id/split
```

将指定子事件从主事件中拆分为独立事件。

**响应**

```json
{
  "code": 200,
  "message": "Incident split"
}
```

---

### 维护计划管理

`scheduledStart` / `scheduledEnd` 均为带时区的 RFC3339 时间字符串（例如 `2026-05-15T02:00:00+08:00`）。
更新计划时若某个时间字段传空字符串，则保留数据库中的原值；前端 `datetime-local` 输入框只接受
`YYYY-MM-DDTHH:mm` 形式，因此编辑回填时需先把存储值转换成该形式，避免时间被清空或写成非法值。

**周期维护**

维护计划支持周期重复。同一个计划始终代表「当前这次窗口 + 下一次窗口」的时间：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `recurrence` | string | `daily` / `weekly` / `monthly`；空字符串表示一次性窗口 |
| `recurrenceInterval` | int | 重复间隔，默认 1（每 N 天 / N 周 / N 月），上限 365 |
| `recurrenceWeekday` | int | 仅 `weekly`：1=周一 … 7=周日。仅用于回显，实际重复日期由 `scheduledStart` 的星期决定 |
| `recurrenceMonthday` | int | 仅 `monthly`：1~31，留空按 `scheduledStart` 的日期；超出当月天数时取当月最后一天（如 31 号 → 2 月 28 日） |
| `recurrenceUntil` | string | 重复截止日期（含当天，`YYYY-MM-DD`），空值表示一直重复 |

重复流转由检查器自动完成：窗口结束（`scheduledEnd` 已过）且状态为 `in_progress` 时，
把 `scheduledStart` / `scheduledEnd` 推进到下一个窗口并把状态置回 `scheduled`；
若已超过 `recurrenceUntil`，则按一次性维护处理，状态置为 `completed`。
服务停机导致一次跨过多个窗口时，会连续推进到第一个尚未结束的未来窗口。

> 更新接口里周期字段使用指针语义：字段缺席表示保持不变，显式传 `"recurrence": ""`
> 才会把周期维护改回一次性窗口（同时清空其它周期字段）。

#### 获取维护计划列表（管理用）

```
GET /api/v1/admin/maintenances
```

返回所有维护计划（不限状态）。

#### 创建维护计划

```
POST /api/v1/admin/maintenances
```

**请求**

```json
{
  "title": "数据库版本升级",
  "description": "主节点版本升级，预计停机 2 小时",
  "scheduledStart": "2026-05-15T02:00:00Z",
  "scheduledEnd": "2026-05-15T04:00:00Z",
  "affectedServices": "1,2",
  "recurrence": "weekly",
  "recurrenceInterval": 1,
  "recurrenceUntil": "2026-12-31"
}
```

**响应** (201)

```json
{
  "code": 201,
  "message": "Maintenance created",
  "data": {
    "id": 1,
    "title": "数据库版本升级",
    "description": "主节点版本升级，预计停机 2 小时",
    "scheduledStart": "2026-05-15T02:00:00Z",
    "scheduledEnd": "2026-05-15T04:00:00Z",
    "status": "scheduled",
    "affectedServices": "1,2",
    "recurrence": "weekly",
    "recurrenceInterval": 1,
    "recurrenceWeekday": 0,
    "recurrenceMonthday": 0,
    "recurrenceUntil": "2026-12-31",
    "createdAt": "2026-05-09T00:00:00Z"
  }
}
```

#### 更新维护计划

```
PUT /api/v1/admin/maintenances/:id
```

**请求**

```json
{
  "scheduledStart": "2026-05-16T02:00:00Z",
  "scheduledEnd": "2026-05-16T04:00:00Z",
  "status": "in_progress",
  "recurrence": "",
  "recurrenceUntil": ""
}
```

上例同时把该计划从周期维护改回一次性窗口（`recurrence` 显式传空）。

#### 删除维护计划

```
DELETE /api/v1/admin/maintenances/:id
```

**响应**

```json
{
  "code": 200,
  "message": "Maintenance deleted"
}
```

---

### API 密钥管理

所有密钥管理接口需携带 `Authorization: Bearer <token>` 头（支持会话 Token 和 API Key 两种认证方式）。

> **凭据差异**：会话 Token 不受限制；API 密钥受下面的 `scope` 限制，且三类高危操作
> （删除服务 / 导入覆盖数据 / 修改管理员账号）**一律不接受 API 密钥**，只能用会话登录调用，
> 否则返回 403。

#### 权限范围与限流

| 字段 | 取值 | 说明 |
| --- | --- | --- |
| `scope` | `read`（默认）/ `write` | `read` 只允许 `GET` / `HEAD` / `OPTIONS`；写方法返回 403。创建时留空按 `read` 处理（最小权限） |
| `rateLimitPerMinute` | `0` ~ `6000` | 每分钟请求上限，`0`（默认）表示不限制；超出返回 **429**，并带 `Retry-After: 60` |

> 升级说明：历史密钥在数据库迁移时统一置为 `read`。如果某个已有密钥本来用于写操作，
> 需要在「API 密钥管理」里把它的范围改成 `write`。

**403 响应**

```json
{
  "code": 403,
  "message": "该 API 密钥为只读范围，不允许写操作"
}
```

```json
{
  "code": 403,
  "message": "该操作仅允许在管理后台登录后执行，API 密钥不可调用"
}
```

**429 响应**

```json
{
  "code": 429,
  "message": "API 密钥请求过于频繁，请稍后重试"
}
```

#### 获取密钥列表

```
GET /api/v1/admin/api-keys
```

返回所有 API 密钥（不返回完整密钥，仅展示掩码后的密钥）。

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "name": "开发环境密钥",
      "maskedKey": "lp_d3b0****a7f3",
      "expiresAt": "",
      "lastUsedAt": "2026-05-09T12:00:00Z",
      "lastUsedIP": "192.168.1.100",
      "isActive": true,
      "scope": "read",
      "rateLimitPerMinute": 0,
      "createdAt": "2026-01-01T00:00:00Z"
    }
  ]
}
```

#### 创建密钥

```
POST /api/v1/admin/api-keys
```

完整密钥仅在此接口返回，请立即保存。

**请求**

```json
{
  "name": "开发环境密钥",
  "expiresAt": "2027-01-01T00:00:00Z",
  "scope": "read",
  "rateLimitPerMinute": 120
}
```

`expiresAt` 为空表示永久有效。推荐使用 RFC3339（如 `2027-01-01T00:00:00+08:00`）；
为兼容手工调用，无时区的 `YYYY-MM-DDTHH:mm[:ss]` 也会被接受，并按北京时间（UTC+8）解释。
无法解析的值会被视为已过期。

`scope` 省略时按 `read` 处理；`rateLimitPerMinute` 省略或为 `0` 表示不限制。

**响应** (201)

```json
{
  "code": 201,
  "message": "密钥创建成功",
  "data": {
    "id": 1,
    "name": "开发环境密钥",
    "key": "lp_d3b0f29a1c8e4f7b2a5d9c3e6f8b0a1d2c4e6f8a0b1c3d5e7f9a0b2c4d6e8f",
    "keyPrefix": "lp_d3b0f2",
    "expiresAt": "2027-01-01T00:00:00Z",
    "scope": "read",
    "rateLimitPerMinute": 120,
    "createdAt": "2026-05-09T00:00:00Z"
  }
}
```

#### 更新密钥

```
PUT /api/v1/admin/api-keys/:id
```

用于改名称、调整权限范围（例如把迁移后的历史密钥改成 `write`）与限流。
`scope` / `rateLimitPerMinute` 不传表示保持原值。

**请求**

```json
{
  "name": "生产环境密钥",
  "scope": "write",
  "rateLimitPerMinute": 0
}
```

#### 删除密钥

```
DELETE /api/v1/admin/api-keys/:id
```

**响应**

```json
{
  "code": 200,
  "message": "密钥已删除"
}
```

---

### 服务器管理

#### 获取服务器列表

```
GET /api/v1/admin/servers
```

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "name": "Web 服务器组",
      "description": "前端 Web 集群",
      "autoMerge": true,
      "autoMergeThreshold": 2,
      "createdAt": "2026-01-01T00:00:00Z",
      "updatedAt": "2026-05-09T00:00:00Z"
    }
  ]
}
```

#### 创建服务器

```
POST /api/v1/admin/servers
```

**请求**

```json
{
  "name": "Web 服务器组",
  "description": "前端 Web 集群",
  "autoMerge": true,
  "autoMergeThreshold": 2
}
```

#### 更新服务器

```
PUT /api/v1/admin/servers/:id
```

#### 删除服务器

```
DELETE /api/v1/admin/servers/:id
```

---

### 探测任务管理

#### 获取探测任务列表

```
GET /api/v1/admin/probe-tasks
```

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "serviceId": 1,
      "serverId": 1,
      "triggerCount": 5,
      "isActive": true,
      "serviceName": "API 服务",
      "serverName": "Web 服务器组",
      "createdAt": "2026-01-01T00:00:00Z",
      "updatedAt": "2026-05-09T00:00:00Z"
    }
  ]
}
```

#### 创建探测任务

```
POST /api/v1/admin/probe-tasks
```

**请求**

```json
{
  "serviceId": 1,
  "serverId": 1,
  "triggerCount": 5
}
```

服务创建时自动创建默认探测任务（triggerCount=5）。

#### 更新探测任务

```
PUT /api/v1/admin/probe-tasks/:id
```

**请求**

```json
{
  "serverId": 1,
  "triggerCount": 3,
  "isActive": true
}
```

#### 删除探测任务

```
DELETE /api/v1/admin/probe-tasks/:id
```

---

### 数据导入导出

#### 导出数据

```
GET /api/v1/admin/export
```

导出服务、事件、事件更新、维护计划和系统设置（不含日志和探测记录）。

**响应** (文件下载)

Content-Disposition 头中包含文件名 `lumipulse-export-YYYY-MM-DD.json`。

```json
{
  "version": 1,
  "exportedAt": "2026-05-30T12:00:00Z",
  "services": [...],
  "incidents": [...],
  "incidentUpdates": [...],
  "maintenances": [...],
  "settings": {
    "site_name": "LumiPulse",
    ...
  }
}
```

#### 导入数据

```
POST /api/v1/admin/import?confirm=true
```

导入将覆盖现有服务、事件、维护计划和系统设置，此操作不可撤销。请求必须包含 `?confirm=true` 参数确认。

**请求体**: 使用导出生成的 JSON 文件内容。

**响应**

```json
{
  "code": 200,
  "message": "导入成功"
}
```

---

### 订阅者管理

#### 获取订阅者列表

```
GET /api/v1/admin/subscribers
```

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "email": "user@example.com",
      "verified": true,
      "subscribedServices": "1,2,3",
      "createdAt": "2026-05-09T00:00:00Z",
      "updatedAt": "2026-05-09T00:00:00Z"
    }
  ]
}
```

`subscribedServices`：逗号分隔的服务 ID 列表，空字符串表示订阅所有服务。

#### 删除订阅者

```
DELETE /api/v1/admin/subscribers/:id
```

**响应**

```json
{
  "code": 200,
  "message": "删除成功"
}
```
