# TG 营销助手

## 手动部署

不建议没基础的用户直接公网使用。以下是 `docker-compose.yml` 配置示例，仓库根目录已经内置同款配置。

### 1. 拉取项目

```bash
git clone https://github.com/bailang1999-a11y/tg.git
cd tg
```

### 2. docker-compose.yml 配置示例

如需自定义端口、密码、域名白名单，可以编辑仓库根目录的 `docker-compose.yml`。

```yaml
services:
  frontend:
    build:
      context: .
      dockerfile: frontend/Dockerfile
    container_name: tg-frontend
    restart: unless-stopped
    ports:
      - "8088:80"
    depends_on:
      - gateway
    networks:
      - tg_marketing

  gateway:
    build:
      context: ./backend
    container_name: tg-gateway
    restart: unless-stopped
    ulimits:
      nofile:
        soft: 100000
        hard: 100000
    volumes:
      - tg_storage:/app/storage
    environment:
      APP_ENV: "production"
      APP_PORT: "8080"
      DATABASE_DSN: "host=postgres user=tg_marketing password=tg_postgres_change_this_9f8a3f6b9f2d4c1a dbname=tg_marketing port=5432 sslmode=disable TimeZone=Asia/Shanghai"
      REDIS_ADDR: "redis:6379"
      REDIS_PASSWORD: "tg_redis_change_this_8b6a4e2f9c1d7a3b"
      REDIS_DB: "0"
      NATS_URL: "nats://nats:4222"
      JWT_SECRET: "tg_jwt_change_this_to_a_long_random_value_3b17a62f1b4a4e88a0b2e1c7f9d6a5c4"
      AUTO_MIGRATE: "true"
      ADMIN_USERNAME: "admin"
      ADMIN_PASSWORD: "TG_Admin_Change_This_2026!"
      ADMIN_EMAIL: "admin@example.com"
      CORS_ORIGINS: "http://localhost:8088,http://127.0.0.1:8088"
      DB_MAX_IDLE_CONNS: "10"
      DB_MAX_OPEN_CONNS: "40"
      DB_CONN_MAX_LIFETIME_SECONDS: "1800"
      DB_CONN_MAX_IDLE_TIME_SECONDS: "300"
      HTTP_MAX_IN_FLIGHT: "2000"
      TASK_RUN_LOCK_STALE_SECONDS: "93600"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
      nats:
        condition: service_started
    networks:
      - tg_marketing

  worker:
    build:
      context: ./backend
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
      DATABASE_DSN: "host=postgres user=tg_marketing password=tg_postgres_change_this_9f8a3f6b9f2d4c1a dbname=tg_marketing port=5432 sslmode=disable TimeZone=Asia/Shanghai"
      REDIS_ADDR: "redis:6379"
      REDIS_PASSWORD: "tg_redis_change_this_8b6a4e2f9c1d7a3b"
      NATS_URL: "nats://nats:4222"
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
      context: ./backend
    container_name: tg-scheduler
    restart: unless-stopped
    command: ["/app/scheduler"]
    volumes:
      - tg_storage:/app/storage
    environment:
      APP_ENV: "production"
      DATABASE_DSN: "host=postgres user=tg_marketing password=tg_postgres_change_this_9f8a3f6b9f2d4c1a dbname=tg_marketing port=5432 sslmode=disable TimeZone=Asia/Shanghai"
      REDIS_ADDR: "redis:6379"
      REDIS_PASSWORD: "tg_redis_change_this_8b6a4e2f9c1d7a3b"
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
      - ./backend/migrations:/docker-entrypoint-initdb.d:ro
    environment:
      POSTGRES_DB: "tg_marketing"
      POSTGRES_USER: "tg_marketing"
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
      - redis-server --appendonly yes --appendfsync everysec --requirepass "tg_redis_change_this_8b6a4e2f9c1d7a3b"
    environment:
      REDIS_PASSWORD: "tg_redis_change_this_8b6a4e2f9c1d7a3b"
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
http://服务器IP:8088
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
