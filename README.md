# TG 营销助手

## 手动部署

不建议没基础的用户直接公网使用。以下是 `docker-compose.yml` 配置示例，仓库根目录已经内置同款配置。

如果你在 1Panel 里创建编排，可以直接复制下面的 `docker-compose.yml` 内容粘贴进去。构建上下文已经指向 GitHub 仓库，不要求 1Panel 的编排目录里提前存在 `backend/` 或 `frontend/`。

### 1. 拉取项目

```bash
git clone https://github.com/bailang1999-a11y/TG-Marketing-Assistant.git
cd tg
```

### 2. docker-compose.yml 配置示例

如需自定义端口、密码、域名白名单，可以编辑仓库根目录的 `docker-compose.yml`。

```yaml
services:
  frontend:
    build:
      context: "https://github.com/bailang1999-a11y/TG-Marketing-Assistant.git#main"
      dockerfile: frontend/Dockerfile
      args:
        APP_VERSION: "1.0.8"
    container_name: tg-frontend
    restart: unless-stopped
    ports:
      # 修改左侧 36666 为你想对外访问的端口，例如 "80:80" 或 "18888:80"
      - "36666:80"
    depends_on:
      gateway:
        condition: service_healthy
    networks:
      - tg_marketing

  gateway:
    build:
      context: "https://github.com/bailang1999-a11y/TG-Marketing-Assistant.git#main:backend"
      args:
        APP_VERSION: "1.0.8"
    container_name: tg-gateway
    restart: unless-stopped
    ulimits:
      nofile:
        soft: 100000
        hard: 100000
    volumes:
      - tg_storage:/app/storage
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      APP_VERSION: "1.0.8"
      APP_ENV: "production"
      APP_PORT: "8080"
      # 数据库连接配置；如果修改 postgres.POSTGRES_PASSWORD，这里的 password 也要同步修改
      DATABASE_DSN: "host=postgres user=tg_marketing password=tg_postgres_change_this_9f8a3f6b9f2d4c1a dbname=tg_marketing port=5432 sslmode=disable TimeZone=Asia/Shanghai"
      REDIS_ADDR: "redis:6379"
      # Redis 密码；如果修改 redis 服务里的密码，这里也要同步修改
      REDIS_PASSWORD: "tg_redis_change_this_8b6a4e2f9c1d7a3b"
      REDIS_DB: "0"
      NATS_URL: "nats://nats:4222"
      # JWT 密钥；公网部署前请改成至少 32 位随机字符串
      JWT_SECRET: "tg_jwt_change_this_to_a_long_random_value_3b17a62f1b4a4e88a0b2e1c7f9d6a5c4"
      AUTO_MIGRATE: "true"
      # 后台初始管理员账号
      ADMIN_USERNAME: "admin"
      # 后台初始管理员密码；公网部署前务必修改
      ADMIN_PASSWORD: "TG_Admin_Change_This_2026!"
      # 后台管理员邮箱，可按需修改
      ADMIN_EMAIL: "admin@example.com"
      # 前端访问域名白名单；部署到域名后改成你的域名，例如 "https://tg.example.com"
      CORS_ORIGINS: "http://localhost:36666,http://127.0.0.1:36666"
      DB_MAX_IDLE_CONNS: "10"
      DB_MAX_OPEN_CONNS: "40"
      DB_CONN_MAX_LIFETIME_SECONDS: "1800"
      DB_CONN_MAX_IDLE_TIME_SECONDS: "300"
      HTTP_MAX_IN_FLIGHT: "2000"
      TASK_RUN_LOCK_STALE_SECONDS: "93600"
      # 一键更新配置；依赖下面的 tg-updater 辅助容器
      APP_UPDATE_ENABLED: "true"
      APP_UPDATE_DOCKER_SOCKET: "/var/run/docker.sock"
      APP_UPDATE_DOCKER_CONTAINER: "tg-updater"
      APP_UPDATE_COMMAND: "cd /workspace && docker compose pull || true; docker compose up -d --build --remove-orphans"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
      nats:
        condition: service_started
    healthcheck:
      test:
        - CMD-SHELL
        - python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8080/health', timeout=3)"
      interval: 10s
      timeout: 5s
      retries: 20
      start_period: 20s
    networks:
      - tg_marketing

  worker:
    build:
      context: "https://github.com/bailang1999-a11y/TG-Marketing-Assistant.git#main:backend"
      args:
        APP_VERSION: "1.0.8"
    container_name: tg-worker
    restart: unless-stopped
    command: ["/app/worker"]
    ulimits:
      nofile:
        soft: 100000
        hard: 100000
    volumes:
      - tg_storage:/app/storage
    environment:
      APP_ENV: "production"
      # 数据库连接配置；必须和 gateway、postgres 的数据库密码保持一致
      DATABASE_DSN: "host=postgres user=tg_marketing password=tg_postgres_change_this_9f8a3f6b9f2d4c1a dbname=tg_marketing port=5432 sslmode=disable TimeZone=Asia/Shanghai"
      REDIS_ADDR: "redis:6379"
      # Redis 密码；必须和 gateway、redis 服务里的密码保持一致
      REDIS_PASSWORD: "tg_redis_change_this_8b6a4e2f9c1d7a3b"
      NATS_URL: "nats://nats:4222"
      # JWT 密钥；必须和 gateway 保持一致
      JWT_SECRET: "tg_jwt_change_this_to_a_long_random_value_3b17a62f1b4a4e88a0b2e1c7f9d6a5c4"
      DB_MAX_IDLE_CONNS: "5"
      DB_MAX_OPEN_CONNS: "20"
      DB_CONN_MAX_LIFETIME_SECONDS: "1800"
      DB_CONN_MAX_IDLE_TIME_SECONDS: "300"
      WORKER_CONCURRENCY: "8"
      TASK_RUN_LOCK_STALE_SECONDS: "93600"
      TASK_QUEUE_ACK_WAIT_SECONDS: "93600"
      TASK_QUEUE_MAX_DELIVER: "5"
    depends_on:
      - gateway
    networks:
      - tg_marketing

  scheduler:
    build:
      context: "https://github.com/bailang1999-a11y/TG-Marketing-Assistant.git#main:backend"
      args:
        APP_VERSION: "1.0.8"
    container_name: tg-scheduler
    restart: unless-stopped
    command: ["/app/scheduler"]
    volumes:
      - tg_storage:/app/storage
    environment:
      APP_ENV: "production"
      # 数据库连接配置；必须和 gateway、postgres 的数据库密码保持一致
      DATABASE_DSN: "host=postgres user=tg_marketing password=tg_postgres_change_this_9f8a3f6b9f2d4c1a dbname=tg_marketing port=5432 sslmode=disable TimeZone=Asia/Shanghai"
      REDIS_ADDR: "redis:6379"
      # Redis 密码；必须和 gateway、redis 服务里的密码保持一致
      REDIS_PASSWORD: "tg_redis_change_this_8b6a4e2f9c1d7a3b"
      # JWT 密钥；必须和 gateway 保持一致
      JWT_SECRET: "tg_jwt_change_this_to_a_long_random_value_3b17a62f1b4a4e88a0b2e1c7f9d6a5c4"
      DB_MAX_IDLE_CONNS: "2"
      DB_MAX_OPEN_CONNS: "5"
      DB_CONN_MAX_LIFETIME_SECONDS: "1800"
      DB_CONN_MAX_IDLE_TIME_SECONDS: "300"
      LOG_RETENTION_DAYS: "30"
      SCHEDULER_INTERVAL_SECONDS: "3600"
      IMPORT_STAGE_RETENTION_HOURS: "24"
    depends_on:
      - gateway
    networks:
      - tg_marketing

  postgres:
    image: postgres:15-alpine
    container_name: tg-postgres
    restart: unless-stopped
    ulimits:
      nofile:
        soft: 100000
        hard: 100000
    volumes:
      - tg_postgres:/var/lib/postgresql/data
    environment:
      POSTGRES_DB: "tg_marketing"
      POSTGRES_USER: "tg_marketing"
      # PostgreSQL 数据库密码；修改后要同步修改 gateway/worker/scheduler 的 DATABASE_DSN
      POSTGRES_PASSWORD: "tg_postgres_change_this_9f8a3f6b9f2d4c1a"
      PGDATA: "/var/lib/postgresql/data"
      TZ: "Asia/Shanghai"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U tg_marketing -d tg_marketing"]
      interval: 10s
      timeout: 5s
      retries: 20
    networks:
      - tg_marketing

  redis:
    image: redis:7-alpine
    container_name: tg-redis
    restart: unless-stopped
    ulimits:
      nofile:
        soft: 100000
        hard: 100000
    volumes:
      - tg_redis:/data
    command:
      - sh
      - -c
      # Redis 启动密码；修改后要同步修改 gateway/worker/scheduler 的 REDIS_PASSWORD
      - redis-server --appendonly yes --appendfsync everysec --requirepass "tg_redis_change_this_8b6a4e2f9c1d7a3b"
    environment:
      # Redis 密码；修改后要同步修改上面的 requirepass 和其它服务的 REDIS_PASSWORD
      REDIS_PASSWORD: "tg_redis_change_this_8b6a4e2f9c1d7a3b"
      # redis-cli 使用的密码；保持和 REDIS_PASSWORD 一致
      REDISCLI_AUTH: "tg_redis_change_this_8b6a4e2f9c1d7a3b"
      TZ: "Asia/Shanghai"
    networks:
      - tg_marketing

  nats:
    image: nats:2.12-alpine
    container_name: tg-nats
    restart: unless-stopped
    command: ["-js", "-sd", "/data"]
    volumes:
      - tg_nats:/data
    networks:
      - tg_marketing

  updater:
    image: docker:28-cli
    container_name: tg-updater
    restart: unless-stopped
    command: ["sh", "-c", "sleep infinity"]
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - .:/workspace
    networks:
      - tg_marketing

networks:
  tg_marketing:
    driver: bridge

volumes:
  tg_storage:
  tg_postgres:
  tg_redis:
  tg_nats:
```

### 3. 启动服务

```bash
docker compose up -d --build
```

### 4. 访问后台

```text
http://服务器IP:36666
```

默认管理员：

```text
账号：admin
密码：TG_Admin_Change_This_2026!
```

## 常用命令

查看运行状态：

```bash
docker compose ps
```

查看日志：

```bash
docker compose logs -f gateway
docker compose logs -f worker
docker compose logs -f scheduler
```

停止服务：

```bash
docker compose down
```

停止并清理全部数据卷：

```bash
docker compose down -v
```

## 上线提醒

公网部署前建议修改 `docker-compose.yml` 里的示例密码和密钥，重点替换：

- `POSTGRES_PASSWORD`
- `REDIS_PASSWORD`
- `JWT_SECRET`
- `ADMIN_PASSWORD`
- `CORS_ORIGINS`

更多部署说明见 [DEPLOY.md](DEPLOY.md)。
