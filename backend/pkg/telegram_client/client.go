package telegram_client

import (
	"context"
	"errors"
)

var ErrDryRunOnly = errors.New("telegram client is in dry-run mode; configure an authorized adapter before external actions")

type AccountProfile struct {
	Nickname  string
	AvatarURL string
	Bio       string
	Online    bool
}

type Client interface {
	GetFullUser(ctx context.Context, terminalID string) (AccountProfile, error)
	CheckAuthorization(ctx context.Context, terminalID string) error
}

type DryRunClient struct{}

func (DryRunClient) GetFullUser(ctx context.Context, terminalID string) (AccountProfile, error) {
	return AccountProfile{}, ErrDryRunOnly
}

func (DryRunClient) CheckAuthorization(ctx context.Context, terminalID string) error {
	return ErrDryRunOnly
}
