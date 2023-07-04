package workclient

import (
	discovery "k8s.io/client-go/discovery"

	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	workv1 "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1alpha1 "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1alpha1"
	"open-cluster-management.io/work/pkg/clients/mqclients"
	"open-cluster-management.io/work/pkg/clients/watcher"

	workclientv1 "open-cluster-management.io/work/pkg/clients/workclient/typed/work/v1"
	workclientv1alpha1 "open-cluster-management.io/work/pkg/clients/workclient/typed/work/v1alpha1"
)

type MQWorkClientSet struct {
	*discovery.DiscoveryClient
	workV1       *workclientv1.MQWorkV1Client
	workV1alpha1 *workclientv1alpha1.MQWorkV1alpha1Client
}

var _ workclientset.Interface = &MQWorkClientSet{}

func NewMQWorkClientSet(mqClient mqclients.MessageQueueClient, watcher watcher.Receiver) *MQWorkClientSet {
	var mqttWorkCS MQWorkClientSet
	mqttWorkCS.workV1 = workclientv1.NewMQWorkV1Client(mqClient, watcher)
	mqttWorkCS.workV1alpha1 = workclientv1alpha1.NewMQWorkV1alpha1Client(mqClient, watcher)

	return &mqttWorkCS
}

// WorkV1 retrieves the WorkV1Client
func (c *MQWorkClientSet) WorkV1() workv1.WorkV1Interface {
	return c.workV1
}

// WorkV1alpha1 retrieves the WorkV1alpha1Client
func (c *MQWorkClientSet) WorkV1alpha1() workv1alpha1.WorkV1alpha1Interface {
	return c.workV1alpha1
}

// Discovery retrieves the DiscoveryClient
func (c *MQWorkClientSet) Discovery() discovery.DiscoveryInterface {
	if c == nil {
		return nil
	}
	return c.DiscoveryClient
}
