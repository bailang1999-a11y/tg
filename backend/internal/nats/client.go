package nats

import (
	"time"

	"codex3/backend/internal/config"

	"github.com/nats-io/nats.go"
)

type Client struct {
	Conn *nats.Conn
	JS   nats.JetStreamContext
}

func Connect(cfg config.Config) (*Client, error) {
	conn, err := nats.Connect(cfg.NATSURL, nats.Timeout(2*time.Second))
	if err != nil {
		return nil, err
	}
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := js.StreamInfo("TASKS"); err != nil {
		if _, addErr := js.AddStream(&nats.StreamConfig{
			Name:     "TASKS",
			Subjects: []string{"tenant.*.task.*"},
			Storage:  nats.FileStorage,
		}); addErr != nil {
			conn.Close()
			return nil, addErr
		}
	}
	return &Client{Conn: conn, JS: js}, nil
}
