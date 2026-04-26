# TG 营销助手

TG 营销助手是一套基于 Vue、Go、PostgreSQL、Redis、NATS 的运营控制台，仓库已内置根目录 `docker-compose.yml`，可以一条命令启动完整服务。

## 一键部署

服务器需要先安装 Docker 和 Docker Compose v2。然后执行：

```bash
git clone https://github.com/bailang1999-a11y/tg.git
cd tg
docker compose up -d --build
```

默认访问地址：

```text
http://服务器IP:8088
```

根目录 [docker-compose.yml](docker-compose.yml) 已经写好 gateway、worker、scheduler、frontend、PostgreSQL、Redis、NATS、数据卷和网络。

公网生产环境上线前，请先编辑 `docker-compose.yml`，替换里面带有 `change_this` / `Change_This` 的示例密码和密钥：

- `POSTGRES_PASSWORD`
- `REDIS_PASSWORD`
- `JWT_SECRET`
- `ADMIN_PASSWORD`
- `CORS_ORIGINS`

## 可选：环境变量版部署脚本

如果你更想把密码放在 `.env.production`，也可以使用仓库根目录的 [deploy.sh](deploy.sh)：

```bash
./deploy.sh
```

首次运行会自动生成 `deploy/.env.production` 并停止部署。编辑这个文件，把所有 `replace-with-*` 占位值替换成真实密码/密钥后，再执行一次 `./deploy.sh`。

`deploy.sh` 内容如下：

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

### 根目录一键版 `docker-compose.yml`

```bash
docker compose up -d --build
docker compose ps
docker compose logs -f gateway
docker compose down
```

清理数据卷：

```bash
docker compose down -v
```

### 环境变量生产版 `deploy/docker-compose.prod.yml`

```bash
docker compose --env-file deploy/.env.production -f deploy/docker-compose.prod.yml ps
docker compose --env-file deploy/.env.production -f deploy/docker-compose.prod.yml logs -f gateway
docker compose --env-file deploy/.env.production -f deploy/docker-compose.prod.yml down
```

### 开发版 `deploy/docker-compose.yml`

仓库也保留了 [deploy/docker-compose.yml](deploy/docker-compose.yml)，适合本地开发或内网临时测试。它带有开发默认密码和调试端口，不建议直接用于公网生产。

启动：

```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

查看状态和日志：

```bash
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml logs -f gateway
```

停止：

```bash
docker compose -f deploy/docker-compose.yml down
```

默认访问地址：

```text
http://服务器IP:8088
```

## 生产安全

上线前必须替换：

- `JWT_SECRET`：至少 32 位随机字符串。
- `ADMIN_PASSWORD`：后台初始管理员密码，不能使用默认值。
- `POSTGRES_PASSWORD` / `REDIS_PASSWORD`：数据库和缓存密码。
- `CORS_ORIGINS`：生产域名，例如 `https://ops.example.com`。

后端在 `APP_ENV=production` 时会拒绝使用默认/占位的 `JWT_SECRET` 或 `ADMIN_PASSWORD` 启动。

不要提交真实生产密钥。真实配置只保存在服务器的 `deploy/.env.production`。
