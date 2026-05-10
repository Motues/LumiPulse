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

## 相关文档

*  API 文档：[doc/api.md](doc/api.md)
*  数据库设计：[doc/data_table.md](doc/data_table.md)

