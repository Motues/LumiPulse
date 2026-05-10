<div align="center">
    <img src="./doc/images/logo.svg" width="84" height="84" alt="Lumi Pulse">
    <h1>Lumi Pulse</h1>
    <p><strong>现代化、简约、轻量的服务监控状态页系统</strong></p>
    <p>
        <img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" alt="Go">
        <img src="https://img.shields.io/badge/Vue-3.5+-4FC08D?logo=vue.js&logoColor=white" alt="Vue">
        <img src="https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white" alt="Docker">
    </p>
</div>


自动监测后端 API 服务和网站可用性，在前端实时展示服务状态（SLA 历史）、发布运维公告以及管理维护周期。

## 功能特性

- **服务监控** — 支持 HTTP、TCP、Ping 三种监控类型，可自定义检查间隔
- **实时状态展示** — 公共状态页实时显示所有服务运行状况，含 90 天在线率矩阵
- **故障事件管理** — 事件创建、进度更新、影响等级划分，自动关联服务状态
- **维护计划管理** — 计划内停机维护预告与展示
- **深色模式** — 支持手动切换深色/浅色模式，偏好持久化存储
- **API 密钥认证** — 支持会话 Token 和 API Key 两种认证方式
- **邮件通知** — SMTP 邮件通知，支持测试邮件发送
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

启动成功后，访问 `http://localhost:3000` 为公共状态页，`htttp://localhost:3000/login` 为登录页面，**默认用户和密码均为`lumi`，首次进入需要修改用户名和密码**。

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

启动成功后，访问 `http://localhost:3000` 为公共状态页，`htttp://localhost:3000/login` 为登录页面，**默认用户和密码均为`lumi`，首次进入需要修改用户名和密码**。


## 相关文档

*  API 文档：[doc/api.md](doc/api.md)
*  数据库设计：[doc/data_table.md](doc/data_table.md)

> Made with ❤️ by [Motues](https://www.motues.top)