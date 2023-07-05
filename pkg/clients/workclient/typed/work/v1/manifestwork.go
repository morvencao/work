package v1

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"
)

// MQManifestWorks implements ManifestWorkInterface
type MQManifestWorks struct {
	client    *MQWorkV1Client
	namespace string
}

var _ workv1client.ManifestWorkInterface = &MQManifestWorks{}

// newMQManifestWorks returns a MQManifestWorks
func newMQManifestWorks(client *MQWorkV1Client, namespace string) *MQManifestWorks {
	return &MQManifestWorks{
		client:    client,
		namespace: namespace,
	}
}

func (mw *MQManifestWorks) Create(ctx context.Context, manifestWork *workv1.ManifestWork, opts metav1.CreateOptions) (*workv1.ManifestWork, error) {
	mw.client.Lock()
	defer mw.client.Unlock()

	addedObj := manifestWork.DeepCopy()

	// generate with UUID v5 based on work name and namespace to make sure uid is not changed across restart
	uid := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s-%s-%s", addedObj.GroupVersionKind().String(), addedObj.Namespace, addedObj.Name))).String()
	addedObj.UID = types.UID(uid)

	if err := mw.client.mqClient.PublishSpec(ctx, addedObj); err != nil {
		klog.Errorf("failed to publish manifestwork spec, %v", err)
		return nil, err
	}

	klog.Infof("creating manifest work")
	mw.client.watcher.Receive(watch.Event{
		Type:   watch.Added,
		Object: addedObj,
	})

	return addedObj, nil
}

func (mw *MQManifestWorks) Update(ctx context.Context, manifestWork *workv1.ManifestWork, opts metav1.UpdateOptions) (*workv1.ManifestWork, error) {
	mw.client.Lock()
	defer mw.client.Unlock()

	updatedObj := manifestWork.DeepCopy()
	updatedObj.ResourceVersion = mw.addResourceVersion(updatedObj.ResourceVersion)

	// the manifest work is deleting and its finalizers are removed, delete it safely
	if !updatedObj.DeletionTimestamp.IsZero() && len(updatedObj.Finalizers) == 0 {
		if err := mw.client.mqClient.PublishStatus(ctx, updatedObj); err != nil {
			// TODO think about how to handle this error
			klog.Errorf("failed to update status, %v", err)
			return manifestWork, nil
		}

		klog.Infof("deleting manifest work")
		mw.client.watcher.Receive(watch.Event{
			Type:   watch.Deleted,
			Object: updatedObj,
		})

		return updatedObj, nil
	}

	klog.Infof("updating manifest work")
	mw.client.watcher.Receive(watch.Event{
		Type:   watch.Modified,
		Object: updatedObj,
	})

	return updatedObj, nil
}

func (mw *MQManifestWorks) UpdateStatus(ctx context.Context, manifestWork *workv1.ManifestWork, opts metav1.UpdateOptions) (*workv1.ManifestWork, error) {
	mw.client.Lock()
	defer mw.client.Unlock()

	updatedObj := manifestWork.DeepCopy()
	updatedObj.ResourceVersion = mw.addResourceVersion(updatedObj.ResourceVersion)

	klog.Infof("updating manifest work status")
	if err := mw.client.mqClient.PublishStatus(ctx, updatedObj); err != nil {
		// TODO think about how to handle this error
		klog.Errorf("failed to update status, %v", err)
		return manifestWork, nil
	}

	mw.client.watcher.Receive(watch.Event{
		Type:   watch.Modified,
		Object: updatedObj,
	})

	return updatedObj, nil
}

func (mw *MQManifestWorks) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	klog.Infof("deleting manifest work")
	return nil
}

func (mw *MQManifestWorks) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	klog.Infof("deleting manifest work collection")
	return nil
}

func (mw *MQManifestWorks) Get(ctx context.Context, name string, opts metav1.GetOptions) (*workv1.ManifestWork, error) {
	klog.Infof("getting manifest work")
	return nil, nil
}

func (mw *MQManifestWorks) List(ctx context.Context, opts metav1.ListOptions) (*workv1.ManifestWorkList, error) {
	klog.Infof("listing manifest work")
	return &workv1.ManifestWorkList{}, nil
}

func (mw *MQManifestWorks) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	klog.Infof("watching manifest work")
	return mw.client.watcher, nil
}

func (mw *MQManifestWorks) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *workv1.ManifestWork, err error) {
	klog.Infof("patching manifest work")

	return nil, nil
}

func (mw *MQManifestWorks) addResourceVersion(resouceVersion string) string {
	if len(resouceVersion) == 0 {
		return fmt.Sprintf("%d", 0)
	}

	newResouceVersion, _ := strconv.ParseInt(resouceVersion, 10, 64)
	newResouceVersion = newResouceVersion + 1
	return fmt.Sprintf("%d", newResouceVersion)
}
