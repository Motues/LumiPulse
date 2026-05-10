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
```

可通过 `PORT` 环境变量覆盖端口。

## API 文档

参见 [doc/api.md](../doc/api.md)。
