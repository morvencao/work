package clients

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1lister "open-cluster-management.io/api/client/work/listers/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/work/pkg/clients/mqtt"
	"open-cluster-management.io/work/pkg/helper"
)

type HubWorkClient struct {
	workClinet   workv1client.ManifestWorkInterface
	workLister   workv1lister.ManifestWorkLister
	workInformer cache.SharedIndexInformer
	hubhash      string
}

func (c *HubWorkClient) GetClinet() workv1client.ManifestWorkInterface {
	return c.workClinet
}

func (c *HubWorkClient) GetInformer() cache.SharedIndexInformer {
	return c.workInformer
}

func (c *HubWorkClient) GetLister() workv1lister.ManifestWorkLister {
	return c.workLister
}

func (c *HubWorkClient) GetHubHash() string {
	return c.hubhash
}

type HubWorkClientBuilder struct {
	clusterName string

	hubKubeconfigFile string

	mqttOptions *mqtt.MQTTClientOptions
}

func NewEventClientBuilder(clusterName string) *HubWorkClientBuilder {
	return &HubWorkClientBuilder{
		clusterName: clusterName,
	}
}

func (b *HubWorkClientBuilder) WithHubKubeconfigFile(hubKubeconfigFile string) *HubWorkClientBuilder {
	b.hubKubeconfigFile = hubKubeconfigFile
	return b
}

func (b *HubWorkClientBuilder) WithMQTTOptions(options *mqtt.MQTTClientOptions) *HubWorkClientBuilder {
	b.mqttOptions = options
	return b
}

func (b *HubWorkClientBuilder) NewHubWorkClient() (*HubWorkClient, error) {
	if b.hubKubeconfigFile != "" {
		return b.newKubeClient()
	}

	if b.mqttOptions.BrokerHost != "" {
		return b.newMQTTClient()
	}

	return nil, fmt.Errorf("")
}

func (b *HubWorkClientBuilder) newKubeClient() (*HubWorkClient, error) {
	hubRestConfig, err := clientcmd.BuildConfigFromFlags("", b.hubKubeconfigFile)
	if err != nil {
		return nil, err
	}

	hubWorkClient, err := workclientset.NewForConfig(hubRestConfig)
	if err != nil {
		return nil, err
	}

	manifestWorkClient := hubWorkClient.WorkV1().ManifestWorks(b.clusterName)

	// Only watch the cluster namespace on hub
	manifestWorkInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.FieldSelector = fields.OneTermEqualSelector("metadata.namespace", b.clusterName).String()
				return manifestWorkClient.List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.FieldSelector = fields.OneTermEqualSelector("metadata.namespace", b.clusterName).String()
				return manifestWorkClient.Watch(context.TODO(), options)
			},
		},
		&workv1.ManifestWork{},
		5*time.Minute,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	return &HubWorkClient{
		workClinet:   manifestWorkClient,
		workInformer: manifestWorkInformer,
		workLister:   workv1lister.NewManifestWorkLister(manifestWorkInformer.GetIndexer()),
		hubhash:      helper.HubHash(hubRestConfig.Host),
	}, nil
}

func (b *HubWorkClientBuilder) newMQTTClient() (*HubWorkClient, error) {
	client, err := b.mqttOptions.GetClient(b.clusterName)
	if err != nil {
		return nil, fmt.Errorf("")
	}

	workClient := &EventClient{watcher: mqtt.NewMQTTWatcher(client, b.mqttOptions.IncomingTopic, b.mqttOptions.Qos)}

	manifestWorkInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return workClient.List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return workClient.Watch(context.TODO(), options)
			},
		},
		&workv1.ManifestWork{},
		5*time.Minute,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	workClient.store = manifestWorkInformer.GetStore()

	return &HubWorkClient{
		workClinet:   workClient,
		workInformer: manifestWorkInformer,
		workLister:   workv1lister.NewManifestWorkLister(manifestWorkInformer.GetIndexer()),
		hubhash:      helper.HubHash(b.mqttOptions.BrokerHost),
	}, nil
}
