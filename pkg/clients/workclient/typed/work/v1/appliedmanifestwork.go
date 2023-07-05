package v1

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"
)

// MQAppliedManifestWorks implements AppliedManifestWorkInterface
type MQAppliedManifestWorks struct {
	client *MQWorkV1Client
}

var _ workv1client.AppliedManifestWorkInterface = &MQAppliedManifestWorks{}

// newMQAppliedManifestWorks returns a MQAppliedManifestWorks
func newMQAppliedManifestWorks(client *MQWorkV1Client) *MQAppliedManifestWorks {
	return &MQAppliedManifestWorks{
		client: client,
	}
}

func (amw *MQAppliedManifestWorks) Create(ctx context.Context, manifestWork *workv1.AppliedManifestWork, opts metav1.CreateOptions) (*workv1.AppliedManifestWork, error) {
	klog.Infof("creating appliedmanifestwork")
	return nil, nil
}

func (amw *MQAppliedManifestWorks) Update(ctx context.Context, manifestWork *workv1.AppliedManifestWork, opts metav1.UpdateOptions) (*workv1.AppliedManifestWork, error) {
	klog.Infof("updating appliedmanifestwork")
	return nil, nil
}

func (amw *MQAppliedManifestWorks) UpdateStatus(ctx context.Context, manifestWork *workv1.AppliedManifestWork, opts metav1.UpdateOptions) (*workv1.AppliedManifestWork, error) {
	klog.Infof("updating status for appliedmanifestwork")
	return nil, nil
}

func (amw *MQAppliedManifestWorks) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	klog.Infof("deleting appliedmanifestwork")
	return nil
}

func (amw *MQAppliedManifestWorks) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	klog.Infof("deleting appliedmanifestwork collection")
	return nil
}

func (amw *MQAppliedManifestWorks) Get(ctx context.Context, name string, opts metav1.GetOptions) (*workv1.AppliedManifestWork, error) {
	klog.Infof("getting appliedmanifestwork")
	return nil, nil
}

func (amw *MQAppliedManifestWorks) List(ctx context.Context, opts metav1.ListOptions) (*workv1.AppliedManifestWorkList, error) {
	klog.Infof("listing appliedmanifestwork")
	return &workv1.AppliedManifestWorkList{}, nil
}

func (amw *MQAppliedManifestWorks) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return amw.client.watcher, nil
}

func (amw *MQAppliedManifestWorks) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *workv1.AppliedManifestWork, err error) {
	klog.Infof("patching appliedmanifestwork")
	return nil, nil
}
