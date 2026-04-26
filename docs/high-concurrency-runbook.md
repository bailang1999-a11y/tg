# 高并发运行手册

## 目标

当前优化目标是让 gateway 承担轻量 API 和任务投递，让耗时任务进入 NATS 后由 worker 池执行。

## 部署建议

- gateway：至少 2 个副本，按 CPU 和 P95 延迟扩容。
- worker：至少 2 个副本，按 NATS pending 数和任务耗时扩容。
- PostgreSQL 前置 PgBouncer，使用 transaction pool。
- 单个 gateway 的 `HTTP_MAX_IN_FLIGHT` 建议从 2000 起压测调参。
- 单个 worker 的 `WORKER_CONCURRENCY` 建议从 8 起压测调参。

## 已迁移到 worker 的任务

- `mass_messaging`
- `join_targets`
- `account_status_check`
- `profile_modification`
- `import_validation`
- `import_session`
- `import_tdata`

## 数据库迁移

新库会按 `backend/migrations` 初始化。已有库需执行：

```bash
psql "$DATABASE_DSN" -f backend/migrations/000002_high_concurrency.sql
```

## 压测

先获取登录 token，再运行：

```bash
BASE_URL=http://localhost:8080 TOKEN="$TOKEN" API_VUS=4500 WS_VUS=500 k6 run deploy/loadtest/k6-api.js
```

推荐分阶段压测：

- 1000 API VUs + 100 WS VUs，确认基础健康。
- 3000 API VUs + 300 WS VUs，观察数据库连接池、PgBouncer、NATS pending。
- 4500 API VUs + 500 WS VUs，验证接近 5000 并发。

## 观察指标

- gateway：P95/P99、503 比例、goroutine、内存、CPU。
- worker：任务耗时、执行失败数、NATS pending、NATS redelivery。
- PostgreSQL：慢查询、连接数、锁等待、索引命中率。
- PgBouncer：active/waiting clients。
- NATS：stream size、consumer pending、redelivered。

## 调参顺序

1. 先控制 `DB_MAX_OPEN_CONNS`，保证所有副本总连接数不会打满 PgBouncer/PostgreSQL。
2. 再调 `HTTP_MAX_IN_FLIGHT`，避免 gateway 过载排队。
3. 再调 `WORKER_CONCURRENCY`，防止 worker 同时打爆 Telegram、代理和数据库。
4. 最后按压测结果增加 gateway/worker 副本。
