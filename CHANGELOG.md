# 更新日志

## v1.0.6 - 2026-04-27

### 新增

- 新增独立「版本更新」页面，路径 `/updates`。
- 左侧菜单新增「版本更新」入口，管理员可直接进入检查版本和触发一键更新。
- 保留系统设置页中的版本更新卡片。

### 验证

- `npm run build` 通过。
- `go test ./...` 通过。
- `docker compose -f docker-compose.yml config` 通过。

## v1.0.5 - 2026-04-26

### 修复

- 移除 Compose 里的本地镜像名 `tg-backend:*` / `tg-frontend:*`，避免 1Panel 将它们当作远程镜像提前拉取并报 `pull access denied`。
- 保留 `APP_VERSION` 构建参数作为缓存破坏因子，继续避免构建复用旧前端。

### 验证

- `npm run build` 通过。
- `go test ./...` 通过。
- `docker compose -f docker-compose.yml config` 通过。

## v1.0.4 - 2026-04-26

### 优化

- Docker Compose 为前端和后端镜像增加明确版本 tag：`tg-frontend:1.0.4`、`tg-backend:1.0.4`。
- 前端和后端 Dockerfile 增加 `APP_VERSION` 构建参数，版本变化时会触发新镜像构建，降低 1Panel 继续复用旧镜像的概率。
- 前端镜像内生成 `/version.json`，便于确认当前容器实际运行版本。

### 验证

- `npm run build` 通过。
- `go test ./...` 通过。
- `docker compose -f docker-compose.yml config` 通过。

## v1.0.3 - 2026-04-26

### 修复

- 修复前端 Nginx 在后端 `gateway` 重建后仍连接旧容器 IP，导致登录和接口请求 `502 Bad Gateway` 的问题。
- Nginx 改为使用 Docker 内置 DNS 动态解析 `gateway`，避免更新过程中 upstream 地址失效。
- 为 `gateway` 增加健康检查，前端容器等待后端健康后再启动。

### 验证

- `npm run build` 通过。
- `go test ./...` 通过。
- `docker compose -f docker-compose.yml config` 通过。

## v1.0.2 - 2026-04-26

### 修复

- 确认线上仍在加载旧前端资源，继续修复 `监听私信` 页面空白问题。
- 增加页面运行时错误兜底，页面异常时显示错误面板，不再只剩空白内容区。
- Nginx 对入口 HTML 禁用缓存，降低更新后继续加载旧 JS 的概率。

### 新增

- 系统设置页新增版本更新卡片，可检查当前版本和 GitHub 最新 Release。
- 新增管理员一键更新接口，通过 `tg-updater` 辅助容器触发 Docker Compose 重新构建。

### 验证

- `npm run build` 通过。
- `docker compose -f docker-compose.yml config` 通过。

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
