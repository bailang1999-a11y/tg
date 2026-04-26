# 环境变量

## 后端

| 名称 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_ENV` | `development` | 运行环境 |
| `APP_PORT` | `8080` | 网关端口 |
| `DATABASE_DSN` | 本地 PostgreSQL | GORM PostgreSQL DSN |
| `REDIS_ADDR` | `localhost:6379` | Redis 地址 |
| `REDIS_PASSWORD` | 空 | Redis 密码 |
| `REDIS_DB` | `0` | Redis DB |
| `NATS_URL` | `nats://localhost:4222` | NATS 地址 |
| `JWT_SECRET` | `change-me-in-production` | JWT 签名密钥 |
| `AUTO_MIGRATE` | `true` | 启动时自动迁移 |
| `ADMIN_USERNAME` | `admin` | 初始管理员 |
| `ADMIN_PASSWORD` | `admin123456` | 初始管理员密码 |
| `ADMIN_EMAIL` | `admin@example.com` | 初始管理员邮箱 |
| `CORS_ORIGINS` | `http://localhost:5173,http://127.0.0.1:5173` | 允许的前端来源 |
| `DB_MAX_IDLE_CONNS` | `10` | 后端进程数据库最大空闲连接数 |
| `DB_MAX_OPEN_CONNS` | `50` | 后端进程数据库最大打开连接数；高副本部署时需结合 PgBouncer 总连接数下调 |
| `DB_CONN_MAX_LIFETIME_SECONDS` | `3600` | 数据库连接最大生命周期，单位秒 |
| `DB_CONN_MAX_IDLE_TIME_SECONDS` | `300` | 数据库连接最大空闲时间，单位秒 |
| `HTTP_MAX_IN_FLIGHT` | `2000` | 单个 gateway 进程允许同时处理的最大请求数，超过返回 503 |
| `WORKER_CONCURRENCY` | `8` | 单个 worker 进程同时执行的任务数 |
| `TASK_RUN_LOCK_STALE_SECONDS` | `93600` | 任务运行锁过期时间，默认 26 小时，避免长任务被重复执行 |
| `TASK_QUEUE_ACK_WAIT_SECONDS` | `93600` | NATS 消息等待 ACK 的时间，默认 26 小时 |
| `TASK_QUEUE_MAX_DELIVER` | `5` | NATS 任务消息最大投递次数 |
| `LOG_RETENTION_DAYS` | `30` | scheduler 清理任务日志的保留天数 |
| `SCHEDULER_INTERVAL_SECONDS` | `3600` | scheduler 执行维护任务的间隔秒数 |
| `IMPORT_STAGE_RETENTION_HOURS` | `24` | 导入暂存目录保留时长，超过后由 scheduler 清理 |

生产环境必须替换 `JWT_SECRET`、数据库密码、MinIO 密码，并关闭不需要的调试端口。
当 `APP_ENV=production` 时，网关会拒绝使用默认或占位的 `JWT_SECRET` / `ADMIN_PASSWORD` 启动；`JWT_SECRET` 建议至少 32 位随机字符串。
