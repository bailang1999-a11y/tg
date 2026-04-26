package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"codex3/backend/internal/config"
	"codex3/backend/internal/database"
	"codex3/backend/internal/handlers"
	appnats "codex3/backend/internal/nats"
	appredis "codex3/backend/internal/redis"
	"codex3/backend/internal/services"
	"codex3/backend/internal/taskqueue"
)

func main() {
	cfg := config.Load()
	if err := cfg.ValidateGateway(); err != nil {
		log.Fatalf("invalid gateway config: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	if cfg.AutoMigrate {
		if err := database.AutoMigrate(db); err != nil {
			log.Fatalf("auto migrate: %v", err)
		}
	}

	if _, err := appredis.Connect(cfg); err != nil {
		log.Printf("redis unavailable: %v", err)
	}

	var natsClient *appnats.Client
	if client, err := appnats.Connect(cfg); err != nil {
		log.Printf("nats unavailable: %v", err)
	} else {
		natsClient = client
		defer natsClient.Conn.Drain()
	}

	auth := services.NewAuthService(db, cfg.JWTSecret)
	if err := auth.EnsureBootstrapAdmin(context.Background(), cfg); err != nil {
		log.Fatalf("bootstrap admin: %v", err)
	}

	router := handlers.NewRouterWithTaskQueue(cfg, db, auth, taskqueue.NewPublisher(natsClient))
	addr := ":" + cfg.AppPort
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Printf("gateway listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("run gateway: %v", err)
	}
}
