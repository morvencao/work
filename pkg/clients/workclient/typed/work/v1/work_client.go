package v1

import (
	"sync"

	rest "k8s.io/client-go/rest"
	workv1 "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"

	"open-cluster-management.io/work/pkg/clients/mqclients"
	"open-cluster-management.io/work/pkg/clients/watcher"
)

type MQWorkV1Client struct {
	restClient rest.Interface

	sync.Mutex
	mqClient mqclients.MessageQueueClient
	watcher  watcher.Receiver
}

var _ workv1.WorkV1Interface = &MQWorkV1Client{}

func NewMQWorkV1Client(mqClient mqclients.MessageQueueClient, watcher watcher.Receiver) *MQWorkV1Client {
	return &MQWorkV1Client{
		mqClient: mqClient,
		watcher:  watcher,
	}
}

func (c *MQWorkV1Client) AppliedManifestWorks() workv1.AppliedManifestWorkInterface {
	return newMQAppliedManifestWorks(c)
}

func (c *MQWorkV1Client) ManifestWorks(namespace string) workv1.ManifestWorkInterface {
	return newMQManifestWorks(c, namespace)
}

func (c *MQWorkV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
