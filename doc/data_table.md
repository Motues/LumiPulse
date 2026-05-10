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
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 创建时间 |
| `updated_at` | DATETIME | DEFAULT (datetime('now')) | 最后更新时间 |

---

## 表：`Heartbeat`

用于记录每次健康检查的结果（历史数据）。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `service_id` | INTEGER | REFERENCES `Service`(`id`) ON DELETE CASCADE | 关联的服务 ID |
| `status` | INTEGER | NOT NULL | 状态码（如 200）或布尔值（1/0） |
| `latency` | INTEGER | — | 响应延迟（单位：毫秒） |
| `message` | TEXT | DEFAULT '' | 错误详情或响应摘要 |
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 检查时间 |

索引：`idx_heartbeat_service_time(service_id, created_at)`

---

## 表：`ServiceDaily`

用于缓存每日健康检查的汇总统计，避免每次查询都从头计算。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `service_id` | INTEGER | REFERENCES `Service`(`id`) ON DELETE CASCADE | 关联的服务 ID |
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
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `service_id` | INTEGER | REFERENCES `Service`(`id`) ON DELETE CASCADE | 关联的服务 ID |
| `title` | TEXT | NOT NULL | 事件标题 |
| `impact` | TEXT | NOT NULL | 影响等级：`minor`, `major`, `critical` |
| `status` | TEXT | DEFAULT 'investigating' | 事件状态：`investigating`, `identified`, `monitoring`, `resolved` |
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 事件开始时间 |
| `updated_at` | DATETIME | DEFAULT (datetime('now')) | 最后更新时间 |

索引：`idx_incident_status(status)`

---

## 表：`Incident_Update`

用于记录某个故障事件的处理进度更新。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `incident_id` | INTEGER | REFERENCES `Incident`(`id`) ON DELETE CASCADE | 关联的事件 ID |
| `status` | TEXT | NOT NULL | 该阶段状态（同 Incident 状态枚举） |
| `content` | TEXT | NOT NULL | 更新的内容描述 |
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
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 创建时间 |

索引：`idx_maintenance_status(status)`

---

## 表：`User`

用于管理状态页后台的管理员账号。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | 自增 ID |
| `username` | TEXT | UNIQUE NOT NULL | 用户名 |
| `password_hash` | TEXT | NOT NULL | 加密后的密码 |
| `email` | TEXT | UNIQUE NOT NULL | 管理员邮箱 |
| `last_login` | DATETIME | — | 最后登录时间 |
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 创建时间 |

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
| `created_at` | DATETIME | DEFAULT (datetime('now')) | 创建时间 |

索引：`idx_apikey_key(key)`
