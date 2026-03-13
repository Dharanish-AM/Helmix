package deploy

import (
	"context"
	"strings"

	eventsdk "github.com/your-org/helmix/libs/event-sdk"
	"github.com/nats-io/nats.go"
)

type natsPublisher struct {
	nc *nats.Conn
}

// NewPublisher returns a NATS-backed publisher when configured, otherwise a no-op publisher.
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