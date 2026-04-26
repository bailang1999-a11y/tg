# TG 营销助手

TG 营销助手是一套基于 Vue、Go、PostgreSQL、Redis、NATS 的运营控制台，仓库已内置 Docker Compose 生产部署文件和一键部署脚本。

## 一键部署

服务器需要先安装 Docker 和 Docker Compose v2。然后执行：

```bash
git clone https://github.com/bailang1999-a11y/tg.git
cd tg
./deploy.sh
```

首次运行会自动生成 `deploy/.env.production` 并停止部署。编辑这个文件，把所有 `replace-with-*` 占位值替换成真实密码/密钥后，再执行一次：

```bash
./deploy.sh
```

默认前端端口由 `deploy/.env.production` 里的 `WEB_PORT` 控制，模板默认是 `8088`。

```text
http://服务器IP:8088
```

## 一键部署脚本

仓库根目录已包含 [deploy.sh](deploy.sh)，内容如下：

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

## 常用命令

```bash
docker compose --env-file deploy/.env.production -f deploy/docker-compose.prod.yml ps
docker compose --env-file deploy/.env.production -f deploy/docker-compose.prod.yml logs -f gateway
docker compose --env-file deploy/.env.production -f deploy/docker-compose.prod.yml down
```

## 生产安全

上线前必须替换：

- `JWT_SECRET`：至少 32 位随机字符串。
- `ADMIN_PASSWORD`：后台初始管理员密码，不能使用默认值。
- `POSTGRES_PASSWORD` / `REDIS_PASSWORD`：数据库和缓存密码。
- `CORS_ORIGINS`：生产域名，例如 `https://ops.example.com`。

后端在 `APP_ENV=production` 时会拒绝使用默认/占位的 `JWT_SECRET` 或 `ADMIN_PASSWORD` 启动。

不要提交真实生产密钥。真实配置只保存在服务器的 `deploy/.env.production`。
