package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	if _, err := appredis.Connect(cfg); err != nil {
		log.Printf("redis unavailable: %v", err)
	}
	natsClient, err := appnats.Connect(cfg)
	if err != nil {
		log.Fatalf("connect nats: %v", err)
	}
	defer natsClient.Conn.Drain()

	server := handlers.NewServer(cfg, db, services.NewAuthService(db, cfg.JWTSecret), nil)
	concurrency := cfg.WorkerConcurrency
	if concurrency <= 0 {
		concurrency = 8
	}
	sem := make(chan struct{}, concurrency)
	sub, err := taskqueue.SubscribeWithOptions(natsClient, taskqueue.SubscribeOptions{
		AckWait:    cfg.TaskQueueAckWait,
		MaxDeliver: cfg.TaskQueueMaxDeliver,
	}, func(ctx context.Context, msg taskqueue.TaskMessage) error {
		sem <- struct{}{}
		defer func() { <-sem }()
		log.Printf("worker received task type=%s id=%s tenant=%s action=%s", msg.Type, msg.TaskID, msg.TenantID, msg.Action)
		switch msg.Type {
		case "mass_messaging":
			server.RunMassMessagingTask(msg.TaskID)
			return nil
		case "direct_messages":
			server.RunDirectMessagesTask(msg.TaskID)
			return nil
		case "join_targets":
			server.RunJoinTargetsTask(msg.TaskID)
			return nil
		case "listener_join_targets":
			server.RunListenerJoinTargetsTask(msg.TaskID)
			return nil
		case "account_status_check":
			server.RunCheckTerminalsTask(msg.TaskID)
			return nil
		case "target_membership_refresh":
			server.RunRefreshTargetMembershipsTask(msg.TaskID)
			return nil
		case "profile_modification":
			server.RunProfileModificationTask(msg.TaskID)
			return nil
		case "import_validation", "import_session", "import_tdata":
			server.RunImportTask(msg.TaskID)
			return nil
		default:
			log.Printf("unsupported task type=%s id=%s", msg.Type, msg.TaskID)
			return nil
		}
	})
	if err != nil {
		log.Fatalf("subscribe tasks: %v", err)
	}
	defer sub.Unsubscribe()

	log.Printf("worker started: consuming NATS task subjects, concurrency=%d", concurrency)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("worker stopping")
}
