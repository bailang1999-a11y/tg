# 更新日志

## v1.0.1 - 2026-04-26

### 修复

- 修复 HTTP/IP 访问环境下 `监听私信` 页面空白的问题。
- 兼容浏览器非安全上下文缺少 `crypto.randomUUID()` 的情况。

### 优化

- 根目录 `docker-compose.yml` 适配 1Panel 编排粘贴部署。
- 1Panel 部署时构建上下文改为 GitHub 仓库，避免缺少本地 `backend/`、`frontend/` 目录导致构建失败。
- 一键部署默认对外端口调整为 `36666`。
- 仓库地址统一为 `TG-Marketing-Assistant`。

### 验证

- `npm run build` 通过。
- `docker compose -f docker-compose.yml config` 通过。
