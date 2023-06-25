package mqclients

import (
	"context"

	"open-cluster-management.io/work/pkg/clients/decoder"
	"open-cluster-management.io/work/pkg/clients/watcher"
)

type MessageQueueClient interface {
	Connect(ctx context.Context) error
	Publish(ctx context.Context) error
	Subscribe(ctx context.Context, decoder decoder.Decoder, receiver watcher.Receiver) error
}
