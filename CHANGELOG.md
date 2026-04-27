# 更新日志

## v1.0.12 - 2026-04-27

### 新增

- 上传监听号后自动为本次新增监听号分配空闲代理，优先使用绑定数量最少的代理。
- 代理列表延迟列增加图标和颜色状态，区分未检测、低延迟、中等延迟、失败和超时。
- 代理检测时自动查询出口 IP 国家，并在代理列表展示国家信息。

### 优化

- 代理延迟检测升级为真实代理检测：SOCKS5 会执行握手并验证账号密码，HTTP 会执行 CONNECT 探测。
- 自动代理分配会跳过检测失败或超时的代理，减少无效出口绑定。

### 验证

- `go test ./...` 通过。
- `npm run build` 通过。
- 本地 Docker 重建并确认 `tg-gateway` healthy。

## v1.0.11 - 2026-04-27

### 新增

- 监听矩阵新增「自动加监听群」任务，支持按监听号分组、监听群分组批量加入。
- 自动加群支持设置每号每日上限、加群间隔和本次最多加群数量，降低风控风险。
- 自动加群会优先处理还没有监听号覆盖的目标群，并跳过账号已加入过的目标。

### 修复

- 修复 1Panel 一键更新时 gateway 无权限访问 Docker socket 导致更新无法启动的问题。
- 优化一键更新命令，自动读取 GitHub 最新 tag，按最新 tag 强制重建并重启服务。

### 验证

- `go test ./...` 通过。
- `npm run build` 通过。
- `docker compose -f docker-compose.yml config` 通过。
- 本地 Docker 重建并确认 `tg-gateway` healthy。

## v1.0.10 - 2026-04-27

### 新增

- 新增全局操作审计日志，覆盖登录以及所有后台 `POST` / `PUT` / `PATCH` / `DELETE` 动作。
- 日志会记录接口、状态码、耗时、输入摘要和失败原因，并自动脱敏密码、token、secret、key、license 等敏感字段。

### 修复

- 修复监听矩阵导入代理失败时无日志的问题，逐行记录成功、重复、格式错误和自动分配警告。
- 优化导入代理返回逻辑，自动分配失败不再掩盖实际导入结果。

### 验证

- `go test ./...` 通过。
- `npm run build` 通过。
- 本地 Docker 重建并确认 `tg-gateway` healthy。

## v1.0.9 - 2026-04-27

### 修复

- 修复版本更新页只读取 GitHub Release 导致最新版本停留在旧版本的问题。
- 版本检查现在会同时读取 GitHub Release 和 tags，并选择语义版本号更高的版本。
- 「有新版本」状态改为仅在远端版本高于当前版本时提示，避免当前版本高于旧 Release 时误判。

### 验证

- `go test ./...` 通过。

## v1.0.8 - 2026-04-27

### 优化

- 重构登录页 UI，首屏品牌标题改为「TG 营销助手」。
- 后台侧边栏品牌名同步改为「TG 营销助手」。
- 左上角品牌标识由 `C3` 文字改为机器人头像样式。
- 同步提升前端、后端和 Docker Compose 默认版本号到 `1.0.8`。

### 验证

- `npm run build` 通过。
- 本地 Docker 前端容器重建并完成页面验收。

## v1.0.7 - 2026-04-27

### 优化

- 按上线批注调整左侧菜单和页面标题：导入账号、目标群组、私信群发、一键修改资料、监听设置。
- 私信群发页面将面向用户的「终端」文案统一改为「账号」，降低运营使用时的理解成本。
- 同步提升前端、后端和 Docker Compose 默认版本号到 `1.0.7`。

### 验证

- `npm run build` 通过。

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
