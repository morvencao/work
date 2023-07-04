package clients

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	"open-cluster-management.io/work/pkg/clients/mqclients/mqtt"
	"open-cluster-management.io/work/pkg/clients/watcher"
	"open-cluster-management.io/work/pkg/clients/workclient"
	"open-cluster-management.io/work/pkg/helper"
)

type HubWorkClient struct {
	workClientSet workclientset.Interface
	mqttClient    *mqtt.MQTTClient
	hubhash       string
}

func (c *HubWorkClient) GetClientSet() workclientset.Interface {
	return c.workClientSet
}

func (c *HubWorkClient) GetHubHash() string {
	return c.hubhash
}

func (c *HubWorkClient) SetStore(store cache.Store) {
	if c.mqttClient != nil {
		c.mqttClient.SetStore(store)
	}
}

type HubWorkClientBuilder struct {
	clusterName       string
	hubKubeconfig     *rest.Config
	hubKubeconfigFile string
	mqttOptions       *mqtt.MQTTClientOptions
	restMapper        meta.RESTMapper
}

func NewHubWorkClientBuilder() *HubWorkClientBuilder {
	return &HubWorkClientBuilder{}
}

func (b *HubWorkClientBuilder) WithClusterName(clusterName string) *HubWorkClientBuilder {
	b.clusterName = clusterName
	return b
}

func (b *HubWorkClientBuilder) WithHubKubeconfigFile(hubKubeconfigFile string) *HubWorkClientBuilder {
	b.hubKubeconfigFile = hubKubeconfigFile
	return b
}

func (b *HubWorkClientBuilder) WithHubKubeconfig(hubKubeconfig *rest.Config) *HubWorkClientBuilder {
	b.hubKubeconfig = hubKubeconfig
	return b
}

func (b *HubWorkClientBuilder) WithMQTTOptions(options *mqtt.MQTTClientOptions) *HubWorkClientBuilder {
	b.mqttOptions = options
	return b
}

func (b *HubWorkClientBuilder) WithRestMapper(restMapper meta.RESTMapper) *HubWorkClientBuilder {
	b.restMapper = restMapper
	return b
}

func (b *HubWorkClientBuilder) NewHubWorkClient(ctx context.Context) (*HubWorkClient, error) {
	// mqtt option go first
	if b.mqttOptions != nil && b.mqttOptions.BrokerHost != "" {
		return b.newMQTTClient(ctx)
	}

	if b.hubKubeconfig != nil || b.hubKubeconfigFile != "" {
		return b.newKubeClient(ctx)
	}

	return nil, fmt.Errorf("invalid options")
}

func (b *HubWorkClientBuilder) newKubeClient(ctx context.Context) (*HubWorkClient, error) {
	if b.hubKubeconfig == nil {
		var err error
		b.hubKubeconfig, err = clientcmd.BuildConfigFromFlags("", b.hubKubeconfigFile)
		if err != nil {
			return nil, err
		}
	}

	workClientSet, err := workclientset.NewForConfig(b.hubKubeconfig)
	if err != nil {
		return nil, err
	}

	return &HubWorkClient{
		workClientSet: workClientSet,
		hubhash:       helper.HubHash(b.hubKubeconfig.Host),
	}, nil
}

func (b *HubWorkClientBuilder) newMQTTClient(ctx context.Context) (*HubWorkClient, error) {
	watcher := watcher.NewMessageQueueWatcher()
	mqttClient := mqtt.NewMQTTClient(b.mqttOptions, b.clusterName, b.restMapper)

	if err := mqttClient.Connect(ctx); err != nil {
		return nil, err
	}

	// TODO publish resync message

	go func() {
		mqttClient.Subscribe(ctx, watcher)
	}()

	workClientSet := workclient.NewMQWorkClientSet(mqttClient, watcher)

	return &HubWorkClient{
		workClientSet: workClientSet,
		mqttClient:    mqttClient,
		hubhash:       helper.HubHash(b.mqttOptions.BrokerHost),
	}, nil
}
