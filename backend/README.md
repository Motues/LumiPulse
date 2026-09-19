# LumiPulse Backend

LumiPulse 状态监控系统后端服务。

## 技术栈

- Go + Gin + SQLite

## 开发

```bash
go run main.go
```

## 构建

```bash
go build -o lumipluse-backend main.go
```

## 配置

配置文件位于 `config/config.yaml`：

```yaml
PORT: 17171
# 全局跳过 HTTPS 证书校验（默认 false）。
# 建议保持关闭，改为在后台「服务管理」里对单个自签证书服务单独开启。
INSECURE_SKIP_VERIFY: false
# 心跳原始数据保留天数，同时是延迟接口 days 参数的上限（默认 30）
HEARTBEAT_RETENTION_DAYS: 30
# 每日汇总数据保留天数，同时是每日统计接口 days 参数的上限（默认 90）
DAILY_RETENTION_DAYS: 90
# 服务端生成内容（RSS / Atom 订阅源、告警邮件、月度 SLA 报告邮件、检查器
# 自动创建的事件与进展文案）的语言：zh-CN（默认）或 en-US。
# 前端界面语言由浏览器决定，不受该项影响。
LANG: zh-CN
# 单轮探测的最大并发数（默认 8，上限 64）。服务多且容易同时超时时可调大。
MAX_PROBE_CONCURRENCY: 8
# 维护计划开始前的提前提醒量（分钟，默认 30）；0 表示关闭提醒。
# 提醒通过邮件与 webhook（事件类型 maintenance）发送。
MAINTENANCE_REMIND_MINUTES: 30
```

可通过 `PORT` 环境变量覆盖端口。

单次探测超时不再硬编码：每个服务可在后台「服务管理」里单独设置
`timeout_seconds`（1~300 秒，留空/0 使用默认 10 秒），慢接口不会被误判为故障。

## 订阅者通知与退订

公开状态页提交的订阅**立即生效**（不做双重确认）。服务异常 / 恢复时，除 `notify_emails`
外还会按订阅者选择的服务列表投递：订阅列表为空表示订阅全部服务（含将来新增的）。

每封订阅者邮件底部带退订 / 偏好管理链接：`<site_url>/unsubscribe?email=...&token=...`。
令牌是 `HMAC-SHA256(unsubscribe_secret, 小写邮箱)`，密钥首次使用时自动生成并存库，
因此链接无法被猜到，也无法用来退订他人邮箱。退订页可一键退订，也可调整订阅的服务范围。
`site_url` 未配置时退回 `allow_origin` 的第一个来源；两者都为空则该邮件不带退订区块。

## 报告邮件（月报 / 周报）

在后台「通知管理」里配置，开关与收件人都存数据库：

- **月度 SLA 报告**：检测到进入新月份时发送上一个自然月的报告。
- **周报摘要**：在选定的星期几（默认周一）发送「往前 7 天」的摘要，版式与月报一致，
  并附带与上一周的环比。

两者都由检查器的每日任务驱动（`handler.SendScheduledReportsIfDue`），各自记录去重标记，
同一天内重启不会重复发送；报告语言由 `sla_report_language` 控制，留空跟随 `LANG`。

## 告警静默与免打扰

告警策略在后台「通知管理 → 告警静默与免打扰」里配置（存数据库，改完立即生效，无需重启）：

- **冷却窗口**（`alert_cooldown_minutes`）：同一服务在窗口内只通知一次，用于抑制服务在阈值
  附近抖动时的重复轰炸。事件照常创建，只是不重复通知。
- **免打扰时段**（`quiet_hours_start` / `quiet_hours_end`，北京时间，支持跨零点）：时段内不发
  告警邮件（异常与恢复），webhook 照常投递；`critical` 级别穿透。维护计划与证书到期提醒是
  一次性通知，不受免打扰影响。

## 通知模板

「通知管理 → 通知模板」可为四类通知（异常 / 恢复 / 维护提醒 / 证书到期）分别自定义主题与正文，
留空则用内置的中/英文案。变量写成 `{{service}}`、`{{duration}}` 这种形式，支持哪些变量见
`doc/api.md` 的「通知模板」一节，也可在设置页的变量说明里直接看到。

实现要点：模板是**纯文本**（见 `i18n/template.go` 的 `RenderTemplate`），写入邮件时统一做
HTML 转义与换行处理（`utils.textToHTML`），因此同一份模板既能用于邮件正文，也能原样作为
webhook 的 `message`，且变量值不会破坏邮件结构。邮件与 webhook 共用同一份渲染结果。

## API 密钥的权限控制

会话 Token（`/admin/login`）代表管理员本人，不受限制；API 密钥受两层限制：

1. **范围** `scope`：`read`（默认，只允许 GET/HEAD/OPTIONS）或 `write`。创建时留空按 `read` 处理；
   历史密钥在迁移时统一置为 `read`，需要写的集成请在「API 密钥管理」里改成 `write`。
2. **限流** `rate_limit_per_minute`：每分钟请求上限（0 = 不限制），超出返回 429 + `Retry-After: 60`。
   限流状态只存内存（进程内按密钥计每分钟），重启即重新计数。

此外，**删除服务 / 导入覆盖数据 / 修改管理员账号**三类高危操作一律不接受 API 密钥
（返回 403），只能用会话登录调用——密钥泄漏的影响面通常远大于会话泄漏。

## HTTPS 证书到期监控

HTTPS 服务探测成功时会读取对端叶子证书的 `NotAfter`，写入 `Service.cert_expires_at`，
公开状态页与管理端服务详情都会展示到期时间与剩余天数。剩余不足 30 天 / 7 天
（含已过期）各发送一次通知（邮件 + webhook `cert_expiring`），同一等级不重复提醒，
证书续期后重新计数。纯 HTTP 服务与 TCP 探测不参与（后者需要额外的 TLS 握手）。

## 端点

- `GET /api/v1/health` — liveness，只表示进程存活
- `GET /api/v1/ready` — readiness，会检查数据库连通性，不可用时返回 503
- `GET /badge/:hash.svg` — 对外状态徽章（无需鉴权，`Cache-Control: public, max-age=60`）
- `GET /api/v1/services/:hash/latency-heatmap?days=7` — 按「日期 × 小时」聚合的响应时间热力图

服务收到 `SIGINT` / `SIGTERM` 后会停机：先停止接受新请求并等待在途请求结束，
再停止检查器并关闭数据库连接。

数据库连接启用了 WAL 与 `busy_timeout`，检查器与接口可并发读写；备份时请
一并处理 `data.db-wal` / `data.db-shm`，或先执行一次 checkpoint。

## API 文档

参见 [doc/api.md](../doc/api.md)。
