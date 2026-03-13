package observability

import (
	"context"
	"strings"

	eventsdk "github.com/your-org/helmix/libs/event-sdk"
	"github.com/nats-io/nats.go"
)

type Publisher interface {
	Publish(ctx context.Context, event any) error
	Close() error
}

type natsPublisher struct {
	nc *nats.Conn
}

func NewPublisher(natsURL string) (Publisher, error) {
	if strings.TrimSpace(natsURL) == "" {
		return noopPublisher{}, nil
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, err
	}
	return &natsPublisher{nc: nc}, nil
}

func (p *natsPublisher) Publish(_ context.Context, event any) error {
	return eventsdk.Publish(p.nc, event)
}

func (p *natsPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	p.nc.Close()
	return nil
}

type noopPublisher struct{}

func (noopPublisher) Publish(context.Context, any) error { return nil }

func (noopPublisher) Close() error { return nil }