package sink

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

type FranzPublisher struct {
	client *kgo.Client
}

func NewFranzPublisher(brokers []string, clientID string) (*FranzPublisher, error) {
	opts := []kgo.Opt{kgo.SeedBrokers(brokers...)}
	if clientID != "" {
		opts = append(opts, kgo.ClientID(clientID))
	}
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &FranzPublisher{client: client}, nil
}

func (p *FranzPublisher) Publish(ctx context.Context, topic string, key []byte, value []byte) error {
	return p.client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: key, Value: value}).FirstErr()
}

func (p *FranzPublisher) Close() {
	if p != nil && p.client != nil {
		p.client.Close()
	}
}
