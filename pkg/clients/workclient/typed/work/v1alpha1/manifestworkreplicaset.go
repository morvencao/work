package v1

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
	workv1alpha1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1alpha1"
	workv1alpha1 "open-cluster-management.io/api/work/v1alpha1"
)

// MQManifestWorkReplicaSets implements ManifestWorkReplicaSetInterface
type MQManifestWorkReplicaSets struct {
	client    *MQWorkV1alpha1Client
	namespace string
}

var _ workv1alpha1client.ManifestWorkReplicaSetInterface = &MQManifestWorkReplicaSets{}

// newMQManifestWorkReplicaSets returns a MQManifestWorkReplicaSets
func newMQManifestWorkReplicaSets(client *MQWorkV1alpha1Client, namespace string) *MQManifestWorkReplicaSets {
	return &MQManifestWorkReplicaSets{
		client:    client,
		namespace: namespace,
	}
}

func (mw *MQManifestWorkReplicaSets) Create(ctx context.Context, manifestWork *workv1alpha1.ManifestWorkReplicaSet, opts metav1.CreateOptions) (*workv1alpha1.ManifestWorkReplicaSet, error) {
	klog.Infof("creating manifestworkreplicaset")
	return nil, nil
}

func (mw *MQManifestWorkReplicaSets) Update(ctx context.Context, manifestWork *workv1alpha1.ManifestWorkReplicaSet, opts metav1.UpdateOptions) (*workv1alpha1.ManifestWorkReplicaSet, error) {
	klog.Infof("updating manifestworkreplicaset")
	return nil, nil
}

func (mw *MQManifestWorkReplicaSets) UpdateStatus(ctx context.Context, manifestWork *workv1alpha1.ManifestWorkReplicaSet, opts metav1.UpdateOptions) (*workv1alpha1.ManifestWorkReplicaSet, error) {
	klog.Infof("updating status for manifestworkreplicaset")
	return nil, nil
}

func (mw *MQManifestWorkReplicaSets) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	klog.Infof("deleting manifestworkreplicaset")
	return nil
}

func (mw *MQManifestWorkReplicaSets) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	klog.Infof("creating manifestworkreplicaset collection")
	return nil
}

func (mw *MQManifestWorkReplicaSets) Get(ctx context.Context, name string, opts metav1.GetOptions) (*workv1alpha1.ManifestWorkReplicaSet, error) {
	klog.Infof("getting manifestworkreplicaset")
	return nil, nil
}

func (mw *MQManifestWorkReplicaSets) List(ctx context.Context, opts metav1.ListOptions) (*workv1alpha1.ManifestWorkReplicaSetList, error) {
	klog.Infof("listing manifestworkreplicaset")
	return &workv1alpha1.ManifestWorkReplicaSetList{}, nil
}

func (mw *MQManifestWorkReplicaSets) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return mw.client.watcher, nil
}

func (mw *MQManifestWorkReplicaSets) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *workv1alpha1.ManifestWorkReplicaSet, err error) {
	klog.Infof("patching manifestworkreplicaset")
	return nil, nil
}
