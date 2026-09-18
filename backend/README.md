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
```

可通过 `PORT` 环境变量覆盖端口。

数据库连接启用了 WAL 与 `busy_timeout`，检查器与接口可并发读写；备份时请
一并处理 `data.db-wal` / `data.db-shm`，或先执行一次 checkpoint。

## API 文档

参见 [doc/api.md](../doc/api.md)。
