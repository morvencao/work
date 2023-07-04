package v1

import (
	"sync"

	rest "k8s.io/client-go/rest"
	workv1alpha1 "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1alpha1"

	"open-cluster-management.io/work/pkg/clients/mqclients"
	"open-cluster-management.io/work/pkg/clients/watcher"
)

type MQWorkV1alpha1Client struct {
	restClient rest.Interface

	sync.Mutex
	mqClient mqclients.MessageQueueClient
	watcher  watcher.Receiver
}

var _ workv1alpha1.WorkV1alpha1Interface = &MQWorkV1alpha1Client{}

func NewMQWorkV1alpha1Client(mqClient mqclients.MessageQueueClient, watcher watcher.Receiver) *MQWorkV1alpha1Client {
	return &MQWorkV1alpha1Client{
		mqClient: mqClient,
		watcher:  watcher,
	}
}

func (c *MQWorkV1alpha1Client) ManifestWorkReplicaSets(namespace string) workv1alpha1.ManifestWorkReplicaSetInterface {
	return newMQManifestWorkReplicaSets(c, namespace)
}

func (c *MQWorkV1alpha1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
