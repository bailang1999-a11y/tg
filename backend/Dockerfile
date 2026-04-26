FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gateway ./cmd/gateway \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/scheduler ./cmd/scheduler

FROM python:3.12-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends gcc libc6-dev \
    && python -m venv /opt/venv \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY requirements.txt /app/requirements.txt
RUN /opt/venv/bin/pip install --no-cache-dir -r /app/requirements.txt \
    && apt-get purge -y --auto-remove gcc libc6-dev
COPY --from=build /out/gateway /app/gateway
COPY --from=build /out/worker /app/worker
COPY --from=build /out/scheduler /app/scheduler
COPY scripts /app/scripts
RUN useradd --system --home-dir /app --shell /usr/sbin/nologin app \
    && mkdir -p /app/storage \
    && chown -R app:app /app/storage
ENV CODEX3_BACKEND_ROOT=/app
ENV TELEGRAM_SYNC_PYTHON=/opt/venv/bin/python
ENV TELEGRAM_SYNC_SCRIPT=/app/scripts/telegram_profile_sync.py
ENV TELEGRAM_APPLY_SCRIPT=/app/scripts/telegram_profile_apply.py
ENV TELEGRAM_MESSAGE_SCRIPT=/app/scripts/telegram_message_send.py
ENV TELEGRAM_LISTEN_SCRIPT=/app/scripts/telegram_keyword_listen.py
USER app

CMD ["/app/gateway"]
