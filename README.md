<div align="center">
    <img src="./doc/images/logo.svg" width="84" height="84" alt="Lumi Pulse">
    <h1>Lumi Pulse</h1>
    <p><strong>现代化、简约、轻量的服务监控状态页系统</strong></p>
    <p>
        <img src="https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white" alt="Go">
        <img src="https://img.shields.io/badge/Vue-4FC08D?logo=vue.js&logoColor=white" alt="Vue">
        <img src="https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white" alt="Docker">
    </p>
</div>


自动监测后端 API 服务和网站可用性，在前端实时展示服务状态（SLA 历史）、发布运维公告以及管理维护周期。

> 注意，目前还处于开发版本，可能存在部分 BUG，预计 v1.0.0 发布稳定版本。

## 功能特性

- **服务监控** — 支持 HTTP、TCP、Ping 三种监控类型，可自定义检查间隔与单次探测超时
- **HTTP 探测高级匹配** — 可自定义请求方法、请求头与请求体（适配需要鉴权的接口与只接受 POST 的健康检查），并支持**期望状态码**（`200` / `200,301` / `200-299` / `2xx`）与**期望响应关键字**，识别「返回 200 但内容是错误页」的情况；请求头 / 请求体属于敏感信息，后台不回显
- **HTTPS 证书监控** — 自动记录 HTTPS 服务证书到期时间，剩余不足 30 / 7 天时分级告警
- **实时状态展示** — 公共状态页实时显示所有服务运行状况，含 90 天在线率矩阵、延迟分位数（P95 / P99）与响应时间热力图
- **服务分组（服务聚合）** — 可把多个服务放进同一分组，首页把分组**融合成一个条目**展示（状态取最严重、可用率与延迟按探测次数加权），点击下拉展开可查看分组内各服务明细
- **故障事件管理** — 事件创建、进度更新、影响等级划分，自动关联服务状态；支持事后复盘（根因 / 处理措施 / 文档链接），可逐事件选择是否对外公开
- **维护计划管理** — 计划内停机维护预告与展示，支持按天 / 周 / 月周期重复
- **月度 SLA 报告 / 周报摘要** — 按月汇总可用率、事件与响应时间，历史月份自动归档长期留存；周报按选定星期发送并附环比，可自动发邮件
- **国际化** — 公开页与管理后台均支持中文 / 英文切换（自动跟随浏览器，页头可手动切换）；RSS 与邮件模板语言由 `LANG` 配置
- **深色模式** — 支持手动切换深色/浅色模式，偏好持久化存储，首屏无闪烁
- **API 密钥认证** — 支持会话 Token 和 API Key 两种认证方式；密钥可设**只读 / 读写范围**与每分钟限流，删除服务、导入数据、改管理员密码等高危操作只能用会话登录执行
- **邮件通知** — SMTP 邮件通知，支持测试邮件发送
- **邮件订阅** — 公开状态页可提交邮箱订阅，按服务范围投递通知，邮件底部带**签名退订链接**（可退订或修改偏好）
- **告警静默** — 冷却窗口抑制抖动期间的重复告警，可设免打扰时段（夜间只发 webhook 不发邮件）
- **告警升级** — 事件长时间无人确认时按设定时限**再次通知**（可配置是否重复提醒）；在事件列表点击「未确认」即可确认，确认后停止升级
- **Webhook 通知** — Slack / Discord / Telegram / 通用 Webhook，通用渠道支持 HMAC-SHA256 签名
- **通知模板** — 异常 / 恢复 / 维护提醒 / 证书到期 / 告警升级五类通知的主题与正文均可在后台自定义，支持 `{{service}}` 等变量
- **分享与收录** — 公开页含 OG / Twitter Card 元信息，并提供 `robots.txt` 与 `sitemap.xml`
- **状态徽章** — `GET /badge/:hash.svg` 返回状态 SVG，可直接嵌入 README / 文档站
- **监控日志** — 查看详细健康检查记录，支持按服务和状态筛选

## 快速开始

### Docker 部署

LumiPulse 支持 Docker 一键部署，镜像发布在 GitHub Container Registry。

```bash
# 使用 docker-compose（推荐）
curl -fsSLO https://raw.githubusercontent.com/Motues/lumipulse/main/docker-compose.yml
docker compose up -d

# 或直接运行
docker run -d \
  --name lumipulse \
  -p 3000:3000 \
  -v momo-data:/app/data \
  ghcr.io/motues/lumipulse:latest
```

启动成功后，访问 `http://localhost:3000` 为公共状态页，`http://localhost:3000/login` 为登录页面，**默认用户和密码均为`lumi`，首次进入需要修改用户名和密码**。

### 二进制文件部署

#### 1. 下载二进制文件

从 [Release](https://github.com/Motues/lumipulse/releases/latest) 下载最新的二进制压缩包，根据你的系统选择对应的文件：

* **Linux**: `backend-linux-amd64.tar.gz` 
* **Windows**: `backend-windows-amd64.zip`

以 Linux 为例，可以使用自带的脚本进行部署：

```bash
wget https://github.com/Motues/lumipulse/releases/latest/download/backend-linux-amd64.tar.gz
tar -xzf backend-linux-amd64.tar.gz
./lumipulse-linux-amd64
```

#### 2. 设置环境变量

运行之后会生成一个 `./config/config.yaml` 文件，可以参考下面的环境变量，请根据需要修改，修改后需要重启服务。

```bash
vim ./config/config.yaml
# 根据实际情况修改环境变量
# ./config/config.yaml
# PORT: 3000  # server port
```

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `3000` | 服务监听端口，也可用同名环境变量覆盖 |
| `INSECURE_SKIP_VERIFY` | `false` | 全局跳过 HTTPS 证书校验。建议保持关闭，改为在后台「服务管理」里对单个自签证书服务单独开启 |
| `HEARTBEAT_RETENTION_DAYS` | `30` | 心跳原始数据保留天数，同时是延迟接口 `days` 参数的上限 |
| `DAILY_RETENTION_DAYS` | `90` | 每日汇总数据保留天数，同时是每日统计接口 `days` 参数的上限 |
| `LANG` | `zh-CN` | 服务端生成内容（RSS / Atom 订阅源、告警邮件、月报邮件）的语言，可选 `zh-CN` / `en-US`。前端界面语言可在页面右上角切换，不受该项影响 |
| `MAX_PROBE_CONCURRENCY` | `8` | 单轮探测的最大并发数，服务数量多时可调大 |
| `MAINTENANCE_REMIND_MINUTES` | `30` | 维护计划开始前的提前提醒量（分钟），`0` 表示关闭提醒 |

启动成功后，访问 `http://localhost:3000` 为公共状态页，`http://localhost:3000/login` 为登录页面，**默认用户和密码均为`lumi`，首次进入需要修改用户名和密码**。


## 相关文档

*  API 文档：[doc/api.md](doc/api.md)
*  数据库设计：[doc/data_table.md](doc/data_table.md)

> Made with ❤️ by [Motues](https://www.motues.top)