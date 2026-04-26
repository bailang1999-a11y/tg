# TG 营销助手部署说明

## 根目录 `docker-compose.yml` 一键部署

最简单的方式是在仓库根目录使用内置的 `docker-compose.yml`：

```bash
git clone https://github.com/bailang1999-a11y/TG-Marketing-Assistant.git
cd tg
docker compose up -d --build
```

如果你使用 1Panel 的「容器」->「编排」->「创建编排」，可以直接复制根目录 `docker-compose.yml` 内容粘贴进去。这个文件的构建上下文已经指向 GitHub 仓库，不依赖 1Panel 本地存在 `backend/` 或 `frontend/` 目录。

默认访问地址：

```text
http://服务器IP:36666
```

这个文件已经包含：

- `frontend`
- `gateway`
- `worker`
- `scheduler`
- `postgres`
- `redis`
- `nats`
- `updater`
- 网络 `tg_marketing`
- 数据卷 `tg_storage`、`tg_postgres`、`tg_redis`、`tg_nats`

`updater` 是一键更新辅助容器。管理员登录后台后，可以在「系统设置」->「版本更新」里点击一键更新，系统会重新拉取 GitHub 最新代码并执行 Docker Compose 重建。

公网生产环境上线前，请先编辑根目录 `docker-compose.yml`，替换里面带有 `change_this` / `Change_This` 的示例密码和密钥：

- `POSTGRES_PASSWORD`
- `REDIS_PASSWORD`
- `JWT_SECRET`
- `ADMIN_PASSWORD`
- `CORS_ORIGINS`

常用命令：

```bash
docker compose ps
docker compose logs -f gateway
docker compose logs -f worker
docker compose logs -f scheduler
docker compose down
```

清理全部数据卷：

```bash
docker compose down -v
```

## 环境变量版 `deploy.sh`

如果你希望把密码放到 `deploy/.env.production`，可以使用仓库根目录的 `deploy.sh`。

首次运行：

```bash
./deploy.sh
```

脚本会自动生成 `deploy/.env.production` 并提示你先替换占位值。编辑完成后再次运行：

```bash
./deploy.sh
```

脚本会执行：

```bash
docker compose --env-file deploy/.env.production -f deploy/docker-compose.prod.yml up -d --build
```

## 手动部署环境变量版

### 生产版 `docker-compose.prod.yml`

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

5. 默认前端端口由 `WEB_PORT` 控制。模板里是 `36666`，浏览器访问：

   ```text
   http://服务器IP:36666
   ```

### 本地/开发版 `docker-compose.yml`

`deploy/docker-compose.yml` 是本地开发/内网测试用的 compose 文件，包含 PostgreSQL、PgBouncer、Redis、NATS、MinIO、gateway、worker、scheduler、frontend。

它默认会暴露数据库、Redis、NATS、MinIO 等端口，并且使用开发密码，例如 `admin123456`、`local-dev-secret-change-me`、`codex3`。公网生产环境请使用 `docker-compose.prod.yml` 和 `.env.production`。

启动开发版：

```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

查看状态：

```bash
docker compose -f deploy/docker-compose.yml ps
```

查看日志：

```bash
docker compose -f deploy/docker-compose.yml logs -f gateway
docker compose -f deploy/docker-compose.yml logs -f worker
docker compose -f deploy/docker-compose.yml logs -f scheduler
```

停止：

```bash
docker compose -f deploy/docker-compose.yml down
```

清理开发版数据卷：

```bash
docker compose -f deploy/docker-compose.yml down -v
```

默认访问地址：

```text
http://服务器IP:36666
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
