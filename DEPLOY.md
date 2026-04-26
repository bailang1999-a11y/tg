# TG 营销助手部署说明

## Docker Compose 一键部署

最简单的方式是在仓库根目录运行：

```bash
./deploy.sh
```

首次运行会自动生成 `deploy/.env.production` 并提示你先替换占位值。编辑完成后再次运行：

```bash
./deploy.sh
```

脚本会执行：

```bash
docker compose --env-file deploy/.env.production -f deploy/docker-compose.prod.yml up -d --build
```

## 手动部署

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

## 一键部署脚本内容

仓库根目录的 `deploy.sh` 内容如下：

```sh
#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ENV_FILE="$ROOT_DIR/deploy/.env.production"
ENV_TEMPLATE="$ROOT_DIR/deploy/.env.production.example"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.prod.yml"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is not installed or not in PATH." >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose v2 is not available. Install Docker Desktop or the docker compose plugin." >&2
  exit 1
fi

if [ ! -f "$ENV_FILE" ]; then
  cp "$ENV_TEMPLATE" "$ENV_FILE"
  echo "Created deploy/.env.production from template."
  echo "Edit deploy/.env.production and replace every replace-with-* value, then run ./deploy.sh again."
  exit 1
fi

if grep -q "replace-with-" "$ENV_FILE"; then
  echo "deploy/.env.production still contains replace-with-* placeholders. Replace them before deploying." >&2
  exit 1
fi

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --build
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps
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
