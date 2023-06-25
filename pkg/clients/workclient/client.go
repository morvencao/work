package workclient

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/work/pkg/clients/mqclients"
)

type MQWorkClient struct {
	mqClient mqclients.MessageQueueClient
	watcher  watch.Interface
	store    cache.Store
}

var _ workv1client.ManifestWorkInterface = &MQWorkClient{}

func NewMQWorkClient(mqClient mqclients.MessageQueueClient, watcher watch.Interface) *MQWorkClient {
	return &MQWorkClient{
		mqClient: mqClient,
		watcher:  watcher,
	}
}

func (c *MQWorkClient) Create(ctx context.Context, manifestWork *workv1.ManifestWork, opts metav1.CreateOptions) (*workv1.ManifestWork, error) {
	klog.Infof("create manifest work")
	return nil, nil
}

func (c *MQWorkClient) Update(ctx context.Context, manifestWork *workv1.ManifestWork, opts metav1.UpdateOptions) (*workv1.ManifestWork, error) {
	klog.Infof("update manifest work")
	if err := c.store.Update(manifestWork.DeepCopy()); err != nil {
		return nil, err
	}
	return manifestWork, nil
}

func (c *MQWorkClient) UpdateStatus(ctx context.Context, manifestWork *workv1.ManifestWork, opts metav1.UpdateOptions) (*workv1.ManifestWork, error) {
	klog.Infof("update manifest work status")
	//TODO:
	return manifestWork, nil
}

func (c *MQWorkClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	klog.Infof("delete manifest work")
	return nil
}

func (c *MQWorkClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	klog.Infof("delete manifest work collection")
	return nil
}

func (c *MQWorkClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*workv1.ManifestWork, error) {
	klog.Infof("get manifest work")
	return nil, nil
}

func (c *MQWorkClient) List(ctx context.Context, opts metav1.ListOptions) (*workv1.ManifestWorkList, error) {
	klog.Infof("list manifest work")
	return &workv1.ManifestWorkList{}, nil
}

func (c *MQWorkClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.watcher, nil
}

func (c *MQWorkClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *workv1.ManifestWork, err error) {
	klog.Infof("patch manifest work")
	return nil, nil
}

func (c *MQWorkClient) AddStore(store cache.Store) {
	c.store = store
}
