# 数据库设计（SQLite）

## 表：`Service`

用于存储需要监控的服务或节点信息。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `name` | TEXT | NOT NULL | 服务名称（如 "API Gateway"） |
| `description` | TEXT | DEFAULT '' | 服务简述 |
| `url` | TEXT | NOT NULL | 监控的终端地址 |
| `type` | TEXT | DEFAULT 'http' | 监控类型：`http`, `tcp`, `ping` |
| `interval` | INTEGER | DEFAULT 60 | 检查间隔（单位：秒） |
| `status` | TEXT | DEFAULT 'operational' | 当前状态：`operational`, `degraded`, `outage` |
| `is_active` | INTEGER | DEFAULT 1 | 是否启用监控（1 为启用，0 为禁用） |
| `sort_order` | INTEGER | DEFAULT 0 | 前端展示排序权重 |
| `show_on_homepage` | INTEGER | DEFAULT 1 | 是否在首页展示（1 为展示，0 为隐藏） |
| `insecure_skip_verify` | INTEGER | DEFAULT 0 | 仅对该服务跳过 HTTPS 证书校验（1 为跳过，用于自签证书的内网服务） |
| `timeout_seconds` | INTEGER | DEFAULT 10 | 单次探测超时（秒）。0 表示使用默认值 10 秒，生效范围 1~300 秒 |
| `cert_expires_at` | TEXT | DEFAULT '' | HTTPS 证书到期时间（RFC3339，UTC）。由检查器在探测成功后写回，空值表示无证书信息 |
| `cert_notify_level` | INTEGER | DEFAULT 0 | 已发送过的证书到期告警等级（0 / 30 / 7，单位：天）。只在等级变严重时通知一次，证书续期后重置为 0 |
| `public_hash` | TEXT | UNIQUE, DEFAULT '' | 公开访问标识（32 位随机十六进制）。公开详情页用它替代自增 ID |
| `http_method` | TEXT | DEFAULT '' | HTTP 探测请求方法，空值等价于 GET。支持 GET/HEAD/POST/PUT/PATCH/DELETE/OPTIONS |
| `http_headers` | TEXT | DEFAULT '' | 自定义请求头，JSON 对象字符串（如 `{"Authorization":"Bearer xxx"}`）。**敏感信息**：管理端接口不回显原值 |
| `http_body` | TEXT | DEFAULT '' | 请求体纯文本，配合 `POST`/`PUT` 的健康检查使用。**敏感信息**：管理端接口不回显原值 |
| `expect_status` | TEXT | DEFAULT '' | 期望状态码模式，支持 `200` / `200,301` / `200-299` / `2xx` 组合（逗号分隔）。空值沿用默认判定 `200 ≤ status < 400` |
| `expect_keyword` | TEXT | DEFAULT '' | 期望响应关键字。非空时响应体必须包含该字符串才算成功；空值表示不校验内容 |
| `folder_id` | INTEGER | DEFAULT NULL | 所属服务分组 ID；空值表示未分组（在首页独立展示）。分组删除时置空，服务本身不受影响 |
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 创建时间 |
| `updated_at` | DATETIME | DEFAULT (datetime('now')) | 最后更新时间 |

索引：`idx_service_public_hash(public_hash)`（唯一）、`idx_service_folder(folder_id)`

> 探测成功判定 = 状态码符合 `expect_status`（或默认 200~399）**且**（若配置了）`message` 中包含 `expect_keyword`。
> 检查器在写心跳时会把响应体片段（最多 200 字符）写进 `Heartbeat.message`，统计查询据此复核历史心跳，
> 保证日志筛选、延迟分桶、热力图与分位数使用与探测时一致的判定口径。

---

## 表：`ServiceFolder`

服务分组（服务聚合文件夹）。把多个服务聚合在一起，公开首页融合成一个条目展示；
只影响展示口径，探测、事件与 SLA 统计仍然按服务计算。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `name` | TEXT | NOT NULL | 分组名称 |
| `description` | TEXT | DEFAULT '' | 分组简述 |
| `show_on_homepage` | INTEGER | DEFAULT 1 | 是否在公开首页展示。关闭后该分组及其下所有服务都不出现在公开首页 |
| `sort_order` | INTEGER | DEFAULT 0 | 排序权重（升序，同值按 ID） |
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 创建时间 |
| `updated_at` | DATETIME | DEFAULT (datetime('now')) | 最后更新时间 |

索引：`idx_service_folder_sort(sort_order, id)`

---

## 表：`Heartbeat`

用于记录每次健康检查的结果（历史数据）。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `service_id` | INTEGER | NOT NULL REFERENCES `Service`(`id`) ON DELETE CASCADE | 关联的服务 ID |
| `status` | INTEGER | NOT NULL | 状态码（如 200）或布尔值（1/0） |
| `latency` | INTEGER | — | 响应延迟（单位：毫秒） |
| `message` | TEXT | DEFAULT '' | 状态码 + 响应体片段（最多 200 字符）；判定失败时额外说明原因（不在期望状态码范围内 / 响应内容未包含期望关键字）。统计查询按该片段复核「期望关键字」 |
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 检查时间 |

索引：`idx_heartbeat_service_time(service_id, created_at)`

---

## 表：`ServiceDaily`

用于缓存每日健康检查的汇总统计，避免每次查询都从头计算。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `service_id` | INTEGER | NOT NULL REFERENCES `Service`(`id`) ON DELETE CASCADE | 关联的服务 ID |
| `date` | TEXT | NOT NULL | 日期（格式：YYYY-MM-DD） |
| `uptime_count` | INTEGER | DEFAULT 0 | 成功检查次数 |
| `downtime_count` | INTEGER | DEFAULT 0 | 失败检查次数 |
| `total_latency` | INTEGER | DEFAULT 0 | 当日总延迟累加 |

约束：`UNIQUE(service_id, date)`

索引：`idx_daily_service_date(service_id, date)`

---

## 表：`Incident`

用于记录服务发生的故障事件或重大异常。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID（仅内部 / 管理接口使用） |
| `public_hash` | TEXT | UNIQUE, DEFAULT '' | 公开访问标识（32 位随机十六进制）。公开页面用它替代自增 ID，避免暴露主键与记录规模 |
| `service_id` | INTEGER | NOT NULL REFERENCES `Service`(`id`) ON DELETE CASCADE | 关联的服务 ID |
| `title` | TEXT | NOT NULL | 事件标题 |
| `impact` | TEXT | NOT NULL | 影响等级：`minor`, `major`, `critical` |
| `status` | TEXT | DEFAULT 'investigating' | 事件状态：`investigating`, `identified`, `monitoring`, `resolved` |
| `affected_services` | TEXT | DEFAULT '' | 合并事件影响的额外服务 ID 列表（逗号分隔） |
| `parent_id` | INTEGER | REFERENCES `Incident`(`id`) ON DELETE SET NULL | 父事件 ID（合并的子事件标识） |
| `resolved_at` | DATETIME | DEFAULT NULL | 首次标记为已解决的时间（固定不变） |
| `root_cause` | TEXT | DEFAULT '' | 事后复盘：根因描述 |
| `resolution` | TEXT | DEFAULT '' | 事后复盘：处理措施 |
| `postmortem_url` | TEXT | DEFAULT '' | 事后复盘：复盘文档链接 |
| `postmortem_public` | INTEGER | DEFAULT 0 | 是否在公开页面展示上述复盘内容（1 为公开）；为 0 时公开接口会清空这三个字段 |
| `acknowledged` | INTEGER | DEFAULT 0 | 是否已被人工确认。确认后不再发送告警升级通知 |
| `acknowledged_at` | TEXT | DEFAULT '' | 确认时间（RFC3339）。公开接口会清空该字段 |
| `acknowledged_by` | TEXT | DEFAULT '' | 确认人（管理员用户名）。公开接口会清空该字段 |
| `escalation_count` | INTEGER | DEFAULT 0 | 已发送的告警升级通知次数 |
| `last_escalated_at` | TEXT | DEFAULT '' | 上次升级通知时间（RFC3339）。取消确认时会一并重置 |
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 事件开始时间 |
| `updated_at` | DATETIME | DEFAULT (datetime('now')) | 最后更新时间 |

索引：`idx_incident_status(status)`、`idx_incident_service_status(service_id, status)`、`idx_incident_parent(parent_id)`、`idx_incident_public_hash(public_hash)`（唯一）

---

## 表：`Incident_Update`

用于记录某个故障事件的处理进度更新。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `incident_id` | INTEGER | REFERENCES `Incident`(`id`) ON DELETE CASCADE | 关联的事件 ID |
| `status` | TEXT | NOT NULL | 该阶段状态（同 Incident 状态枚举） |
| `content` | TEXT | NOT NULL | 更新的内容描述 |
| `is_internal` | INTEGER | DEFAULT 0 | 是否为系统内部记录（合并/拆分操作，1 为内部） |
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 更新时间 |

索引：`idx_incident_update_incident(incident_id)`

---

## 表：`Maintenance`

用于计划内的停机维护预告。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `title` | TEXT | NOT NULL | 维护标题 |
| `description` | TEXT | DEFAULT '' | 维护内容详细说明 |
| `scheduled_start` | DATETIME | NOT NULL | 计划开始时间 |
| `scheduled_end` | DATETIME | NOT NULL | 计划结束时间 |
| `status` | TEXT | DEFAULT 'scheduled' | 状态：`scheduled`, `in_progress`, `completed`, `cancelled` |
| `affected_services` | TEXT | DEFAULT '' | 受影响的服务 ID 列表（JSON 字符串或逗号分隔） |
| `recurrence` | TEXT | DEFAULT '' | 周期重复方式：`daily`, `weekly`, `monthly`；空值=一次性维护窗口 |
| `recurrence_interval` | INTEGER | DEFAULT 1 | 重复间隔（每 N 天 / N 周 / N 月） |
| `recurrence_weekday` | INTEGER | DEFAULT 0 | 仅 `weekly`：1=周一 … 7=周日（仅用于回显，实际重复日期由 `scheduled_start` 的星期决定） |
| `recurrence_monthday` | INTEGER | DEFAULT 0 | 仅 `monthly`：1~31，超出当月天数时取当月最后一天 |
| `recurrence_until` | TEXT | DEFAULT '' | 重复截止日期（`YYYY-MM-DD`，含当天）；空值=一直重复 |
| `reminded` | INTEGER | DEFAULT 0 | 「即将开始」提醒是否已发送（1 为已发送），保证每个维护窗口最多提醒一次 |
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 创建时间 |

索引：`idx_maintenance_status(status)`

> 周期维护只保留「当前这次窗口 + 下一次窗口」：窗口结束后检查器把
> `scheduled_start` / `scheduled_end` 推进到下一个窗口并将状态置回 `scheduled`，
> 因此同一行的时间始终代表下一次（或正在进行）的维护窗口。
> 推进窗口时会同时把 `reminded` 重置为 0，使每个窗口都能重新提醒。

---

## 表：`ServiceMonthly`

月度 SLA 汇总。`ServiceDaily` 会被 `DAILY_RETENTION_DAYS` 清理，而月报需要长期可查，
因此在已结束月份的每日数据被清理前先固化到这里。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `service_id` | INTEGER | NOT NULL REFERENCES `Service`(`id`) ON DELETE CASCADE | 关联的服务 ID |
| `month` | TEXT | NOT NULL | 月份（格式：YYYY-MM） |
| `uptime_count` | INTEGER | DEFAULT 0 | 当月成功探测次数 |
| `downtime_count` | INTEGER | DEFAULT 0 | 当月失败探测次数 |
| `total_latency` | INTEGER | DEFAULT 0 | 当月总延迟累加（与 `ServiceDaily` 口径一致） |
| `incident_total` | INTEGER | DEFAULT 0 | 当月事件数（只统计根事件） |
| `incident_downtime_seconds` | INTEGER | DEFAULT 0 | 当月事件造成的不可用时长（秒），跨月事件只计入落在本月的那一段 |
| `recorded_at` | DATETIME | DEFAULT NULL | 最近一次写入时间 |
| `closed_at` | DATETIME | DEFAULT NULL | 归档时间；空值表示月份尚未固化，数据仍可能变化 |

约束：`UNIQUE(service_id, month)`

索引：`idx_monthly_service_month(service_id, month)`

> 归档时机：检查器每天清理前会归档最近 6 个月内已结束的月份；
> 查询历史月份时若发现缺失也会即时固化。可用率按探测次数加权计算。

---

## 表：`Subscriber`

用于存储公开订阅服务状态通知的邮箱。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `email` | TEXT | UNIQUE NOT NULL | 订阅邮箱地址 |
| `verified` | INTEGER | DEFAULT 0 | 订阅是否有效（1 为有效）。本项目不做双重确认，创建时即置 1；保留该列仅用于标识订阅状态 |
| `subscribed_services` | TEXT | DEFAULT '' | 订阅的服务 ID 列表（逗号分隔，空=全部） |
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 订阅时间 |
| `updated_at` | DATETIME | DEFAULT (datetime('now')) | 最后更新时间 |

---

## 表：`Settings`

键值对存储的系统设置项。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `key` | TEXT | PRIMARY KEY | 设置键名 |
| `value` | TEXT | NOT NULL | 设置值 |
| `updated_at` | TEXT | NOT NULL | 最后更新时间 |

支持的键名：

| 键名 | 说明 | 示例 |
| --- | --- | --- |
| `site_name` | 站点名称 | LumiPulse |
| `site_icon` | 站点图标 URL | https://example.com/icon.png |
| `admin_email` | 管理员邮箱 | admin@example.com |
| `admin_name` | 管理员用户名 | admin |
| `admin_password` | bcrypt 加密的密码 | $2a$10$... |
| `allow_origin` | CORS 允许的域名列表 | http://localhost:5173 |
| `smtp_host` | SMTP 服务器地址 | smtp.example.com |
| `smtp_port` | SMTP 服务器端口 | 587 |
| `smtp_user` | SMTP 登录用户名 | user@example.com |
| `smtp_pass` | SMTP 登录密码 | （返回时留空） |
| `smtp_encryption` | SMTP 加密方式 | tls, starttls |
| `email_enabled` | 是否启用邮件通知 | true, false |
| `notify_services` | 通知关联的服务 ID | 1,2,3 |
| `notify_emails` | 通知接收邮箱列表 | a@example.com,b@example.com |
| `webhook_enabled` | 是否启用 Webhook 通知 | true, false |
| `webhook_type` | Webhook 渠道 | generic, slack, discord, telegram |
| `webhook_url` | Webhook 接收地址 | https://hooks.slack.com/services/... |
| `webhook_secret` | 通用渠道的 HMAC-SHA256 签名密钥 | （返回时留空则不展示） |
| `webhook_events` | 订阅的事件类型（逗号分隔） | down,up |
| `webhook_telegram_chat_id` | Telegram 目标会话 ID | -1001234567890 |
| `sla_report_enabled` | 是否每月自动发送 SLA 报告邮件 | true, false |
| `sla_report_emails` | 月报收件邮箱；留空复用 `notify_emails` | a@example.com |
| `sla_report_language` | 月报语言；留空跟随配置文件 `LANG` | zh-CN, en-US |
| `last_sla_report_month` | 最近一次发送的月报月份（由服务端写入，用于去重） | 2026-05 |
| `show_admin_footer_button` | 页脚是否显示管理后台链接 | true, false |
| `custom_footer` | 自定义页脚 HTML 内容 | `<a href="...">我的站点</a>` |
| `sub_enable_email` | 邮件订阅方式开关 | true, false |
| `sub_enable_rss` | RSS 订阅方式开关 | true, false |
| `sub_enable_atom` | Atom 订阅方式开关 | true, false |

---

## 表：`ApiKey`

用于 API 访问认证的密钥管理。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `name` | TEXT | NOT NULL | 密钥名称（用于区分用途） |
| `key` | TEXT | UNIQUE NOT NULL | 完整密钥（`lp_` 前缀 + 64 位 hex，共 67 字符） |
| `key_prefix` | TEXT | NOT NULL | 密钥前 8 位（用于列表展示区分） |
| `expires_at` | TEXT | DEFAULT '' | 过期时间（RFC3339 格式，空值=永久有效） |
| `last_used_at` | TEXT | DEFAULT '' | 最后使用时间 |
| `last_used_ip` | TEXT | DEFAULT '' | 最后使用的 IP 地址 |
| `is_active` | INTEGER | DEFAULT 1 | 是否启用（1 为启用，0 为禁用） |
| `scope` | TEXT | DEFAULT 'read' | 权限范围：`read` 只允许 GET/HEAD，`write` 允许写操作；历史密钥迁移后为 `read` |
| `rate_limit_per_minute` | INTEGER | DEFAULT 0 | 每分钟请求上限（0 = 不限制），超出返回 429 |
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 创建时间 |

索引：`idx_apikey_key(key)`

---

## 表：`Server`

逻辑分组，将多个探测任务归组到同一服务器下，支持自动合并事件。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `name` | TEXT | NOT NULL | 服务器名称 |
| `description` | TEXT | DEFAULT '' | 描述 |
| `auto_merge` | INTEGER | DEFAULT 1 | 是否自动合并事件（1 为启用） |
| `auto_merge_threshold` | INTEGER | DEFAULT 1 | 同时异常的服务数达到此阈值时自动合并 |
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 创建时间 |
| `updated_at` | DATETIME | DEFAULT (datetime('now')) | 更新时间 |

---

## 表：`ProbeTask`

定义对特定服务执行健康检查的探测任务，可关联到服务器。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `service_id` | INTEGER | UNIQUE REFERENCES `Service`(`id`) ON DELETE CASCADE | 关联的服务 ID（一对一） |
| `server_id` | INTEGER | REFERENCES `Server`(`id`) ON DELETE SET NULL | 关联的服务器 ID（可选） |
| `trigger_count` | INTEGER | DEFAULT 5 | 连续失败次数阈值，超过则触发事件 |
| `is_active` | INTEGER | DEFAULT 1 | 是否启用探测 |
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 创建时间 |
| `updated_at` | DATETIME | DEFAULT (datetime('now')) | 更新时间 |

索引：`idx_probe_service(service_id)`、`idx_probe_server(server_id)`
