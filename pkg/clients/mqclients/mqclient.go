package mqclients

import (
	"context"

	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/work/pkg/clients/decoder"
	"open-cluster-management.io/work/pkg/clients/watcher"
)

type MessageQueueClient interface {
	Connect(ctx context.Context) error
	Publish(ctx context.Context, work *workv1.ManifestWork) error
	Subscribe(ctx context.Context, decoder decoder.Decoder, receiver watcher.Receiver) error
}
