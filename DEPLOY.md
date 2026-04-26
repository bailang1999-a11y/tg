# TG 营销助手部署说明

## Docker Compose 部署

1. 进入项目部署目录：

   ```bash
   cd deploy
   ```

2. 复制环境变量模板，并替换所有 `replace-with-*` 值：

   ```bash
   cp .env.production.example .env.production
   ```

3. 构建并启动：

   ```bash
   docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build
   ```

4. 查看状态：

   ```bash
   docker compose --env-file .env.production -f docker-compose.prod.yml ps
   docker compose --env-file .env.production -f docker-compose.prod.yml logs -f gateway
   ```

5. 默认前端端口由 `WEB_PORT` 控制。模板里是 `8088`，浏览器访问：

   ```text
   http://服务器IP:8088
   ```

## 上线前必须替换

- `JWT_SECRET`：至少 32 位随机字符串。
- `ADMIN_PASSWORD`：后台初始管理员密码，不能使用默认值。
- `POSTGRES_PASSWORD` / `REDIS_PASSWORD`：数据库和缓存密码。
- `CORS_ORIGINS`：生产域名，例如 `https://ops.example.com`。

`APP_ENV=production` 时，如果 `JWT_SECRET` 或 `ADMIN_PASSWORD` 仍是默认/占位值，后端会拒绝启动。

## 包内不包含

- 本地上传数据：`backend/storage`
- 本地 Python 虚拟环境：`backend/.venv`
- 前端依赖和构建产物：`frontend/node_modules`、`frontend/dist`
- 本地数据库和运行日志：`codex3.db`、`.runtime`
