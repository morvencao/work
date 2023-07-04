package mqclients

import (
	"context"

	"k8s.io/client-go/tools/cache"
	workv1 "open-cluster-management.io/api/work/v1"

	"open-cluster-management.io/work/pkg/clients/watcher"
)

type MessageQueueClient interface {
	Connect(ctx context.Context) error
	PublishSpec(ctx context.Context, work *workv1.ManifestWork) error
	PublishStatus(ctx context.Context, work *workv1.ManifestWork) error
	Subscribe(ctx context.Context, receiver watcher.Receiver) error
	SetStore(store cache.Store)
}
