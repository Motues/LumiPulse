# LumiPulse API 文档

**基础信息**

- **Base URL**: `/api/v1`
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

---

## 公共接口 (Public API)

用于状态页前端展示，无需鉴权。

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
    "email_enabled": "true"
  }
}
```

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
        "name": "API 服务",
        "status": "operational",
        "url": "https://api.example.com",
        "uptime": 99.49
      }
    ],
    "activeIncidents": [
      {
        "id": 1,
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

---

### 获取服务列表

```
GET /api/v1/services
```

获取所有监控服务的当前状态及在线率。状态同样经过 `reconcileStatus` 计算。

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "name": "API 服务",
      "status": "operational",
      "url": "https://api.example.com",
      "uptime": 99.49
    }
  ]
}
```

---

### 获取服务历史

```
GET /api/v1/services/:id/history?days=90
```

获取特定服务的历史可用性数据。

**查询参数**

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `days` | int | 90 | 历史天数（如 30 或 90） |

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

### 获取服务每日统计

```
GET /api/v1/services/:id/daily-stats?days=90
```

获取特定服务每天的健康检查汇总数据，用于前端矩阵展示。每个元素为 `[upCount, downCount, statusCode]` 三元组。

**查询参数**

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `days` | int | 90 | 天数（最大 365） |

**响应**

```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "serviceId": 1,
    "days": [
      [48, 0, 0],
      [-1, -1, -1],
      [46, 2, 1]
    ]
  }
}
```

`statusCode` 取值：`-1`=无数据、`0`=正常、`1`=调查中、`2`=已确认、`3`=监控中、`4`=已解决

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
| `status` | string | "all" | 筛选：`all`、`success`、`failure` |

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
        "message": "OK",
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
    "admin_email": "admin@example.com",
    "admin_name": "admin",
    "allow_origin": "http://localhost:5173",
    "smtp_host": "",
    "smtp_port": "",
    "smtp_user": "",
    "smtp_encryption": "",
    "email_enabled": "true",
    "notify_services": "",
    "notify_emails": ""
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
  "sortOrder": 0
}
```

`type` 取值：`http`（默认）、`tcp`、`ping`

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
    "status": "operational",
    "isActive": true,
    "sortOrder": 0,
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
      "createdAt": "...",
      "updatedAt": "...",
      "uptime": 99.49,
      "latency": 120
    }
  ]
}
```

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
  "isActive": true
}
```

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

同公共接口 `/api/v1/incidents`，返回格式一致。

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

#### 更新事件

```
PATCH /api/v1/admin/incidents/:id
```

**请求**

```json
{
  "impact": "major",
  "status": "resolved",
  "title": "API 服务中断（已恢复）"
}
```

当 `status` 设为 `resolved` 时，自动恢复关联服务状态为 `operational`。

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

### 维护计划管理

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
  "affectedServices": "1,2"
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
  "status": "in_progress"
}
```

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
  "expiresAt": "2027-01-01T00:00:00Z"
}
```

`expiresAt` 为空表示永久有效。

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
    "createdAt": "2026-05-09T00:00:00Z"
  }
}
```

#### 更新密钥名称

```
PUT /api/v1/admin/api-keys/:id
```

**请求**

```json
{
  "name": "生产环境密钥"
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
