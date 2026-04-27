package taskqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appnats "codex3/backend/internal/nats"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

const (
	SubjectPrefix = "tenant"
	WorkerQueue   = "codex3-workers"
	DurableName   = "codex3-worker"
)

type TaskMessage struct {
	TaskID    uuid.UUID `json:"task_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Type      string    `json:"type"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

type Handler func(context.Context, TaskMessage) error

type SubscribeOptions struct {
	AckWait    time.Duration
	MaxDeliver int
}

type Publisher struct {
	client *appnats.Client
}

func NewPublisher(client *appnats.Client) *Publisher {
	if client == nil || client.JS == nil {
		return nil
	}
	return &Publisher{client: client}
}

func (p *Publisher) PublishTask(ctx context.Context, msg TaskMessage) error {
	if p == nil || p.client == nil || p.client.JS == nil {
		return fmt.Errorf("task queue unavailable")
	}
	if msg.TaskID == uuid.Nil || msg.Type == "" {
		return fmt.Errorf("invalid task message")
	}
	if msg.Action == "" {
		msg.Action = "run"
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = p.client.JS.PublishMsg(&nats.Msg{
		Subject: subjectFor(msg.TenantID, msg.Type),
		Data:    payload,
		Header: nats.Header{
			"Nats-Msg-Id": []string{fmt.Sprintf("%s:%s:%s", msg.TenantID, msg.Type, msg.TaskID)},
		},
	}, nats.Context(ctx))
	return err
}

func Subscribe(client *appnats.Client, handler Handler) (*nats.Subscription, error) {
	return SubscribeWithOptions(client, SubscribeOptions{}, handler)
}

func SubscribeWithOptions(client *appnats.Client, options SubscribeOptions, handler Handler) (*nats.Subscription, error) {
	if client == nil || client.JS == nil {
		return nil, fmt.Errorf("task queue unavailable")
	}
	ackWait := options.AckWait
	if ackWait <= 0 {
		ackWait = 26 * time.Hour
	}
	maxDeliver := options.MaxDeliver
	if maxDeliver <= 0 {
		maxDeliver = 5
	}
	return client.JS.QueueSubscribe(
		"tenant.*.task.*",
		WorkerQueue,
		func(msg *nats.Msg) {
			var taskMsg TaskMessage
			if err := json.Unmarshal(msg.Data, &taskMsg); err != nil {
				_ = msg.Term()
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
			defer cancel()
			if err := handler(ctx, taskMsg); err != nil {
				_ = msg.Nak()
				return
			}
			_ = msg.Ack()
		},
		nats.Durable(DurableName),
		nats.ManualAck(),
		nats.AckWait(ackWait),
		nats.MaxDeliver(maxDeliver),
	)
}

func subjectFor(tenantID uuid.UUID, taskType string) string {
	return fmt.Sprintf("%s.%s.task.%s", SubjectPrefix, tenantID.String(), taskType)
}
