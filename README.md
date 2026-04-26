# TG 营销助手

TG 营销助手 is an operations console with a Vue frontend, Go backend, PostgreSQL, Redis, and NATS.

## Docker Compose One-Command Deploy

1. Copy and edit the production environment file:

   ```bash
   cp deploy/.env.production.example deploy/.env.production
   ```

2. Replace every `replace-with-*` value in `deploy/.env.production`.

3. Start the full stack:

   ```bash
   ./deploy.sh
   ```

The frontend port is controlled by `WEB_PORT` in `deploy/.env.production`. The default template uses `8088`.

## Useful Commands

```bash
docker compose --env-file deploy/.env.production -f deploy/docker-compose.prod.yml ps
docker compose --env-file deploy/.env.production -f deploy/docker-compose.prod.yml logs -f gateway
docker compose --env-file deploy/.env.production -f deploy/docker-compose.prod.yml down
```

## Production Safety

The backend refuses to start in production when `JWT_SECRET` or `ADMIN_PASSWORD` still uses a default or placeholder value.

Do not commit real production secrets. Keep them in `deploy/.env.production` on the server only.
