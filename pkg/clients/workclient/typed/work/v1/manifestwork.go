package v1

import (
	"context"
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/google/uuid"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	strategicpatch "k8s.io/apimachinery/pkg/util/strategicpatch"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/work/pkg/clients/mqclients/encoder"
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

	resourceVersion, ok := manifestWork.Annotations["manifestworkreplicaset-generation"]
	if !ok {
		return nil, fmt.Errorf("manifestworkreplicaset generation is required")
	}

	addedObj := manifestWork.DeepCopy()
	// generate with UUID v5 based on work name and namespace to make sure uid is not changed across restart
	// TODO: replace uuid.NameSpaceOID with UUID of managed cluster
	uid := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s-%s-%s", addedObj.GroupVersionKind().String(), addedObj.Namespace, addedObj.Name))).String()
	addedObj.UID = types.UID(uid)
	addedObj.ResourceVersion = resourceVersion

	if err := mw.client.mqClient.Publish(ctx, addedObj); err != nil {
		klog.Errorf("failed to publish manifestwork spec, %v", err)
		return nil, err
	}

	klog.Infof("Creating manifest work %v", addedObj.ObjectMeta)
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

	// the manifest work is deleting and its finalizers are removed, delete it safely
	if !updatedObj.DeletionTimestamp.IsZero() && len(updatedObj.Finalizers) == 0 {
		// update status hash to make sure hub can receive the delete status
		hash, err := encoder.GetStatusHash(updatedObj)
		if err != nil {
			return nil, err
		}
		updatedObj.Annotations = map[string]string{"statushash": hash}

		if err := mw.client.mqClient.Publish(ctx, updatedObj); err != nil {
			// TODO think about how to handle this error
			klog.Errorf("failed to update status, %v", err)
			return manifestWork, nil
		}

		klog.Infof("Deleting manifest work, %v", updatedObj.ObjectMeta)
		mw.client.watcher.Receive(watch.Event{
			Type:   watch.Deleted,
			Object: updatedObj,
		})

		return updatedObj, nil
	}

	klog.Infof("Updating manifest work, %v", updatedObj.ObjectMeta)
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
	hash, err := encoder.GetStatusHash(updatedObj)
	if err != nil {
		return nil, err
	}

	updatedObj.Annotations = map[string]string{"statushash": hash}
	if err := mw.client.mqClient.Publish(ctx, updatedObj); err != nil {
		// TODO think about how to handle this error
		klog.Errorf("failed to update status, %v", err)
		return manifestWork, nil
	}

	klog.Infof("Updating manifest work status, %v", updatedObj.ObjectMeta)
	mw.client.watcher.Receive(watch.Event{
		Type:   watch.Modified,
		Object: updatedObj,
	})

	return updatedObj, nil
}

func (mw *MQManifestWorks) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	mw.client.Lock()
	defer mw.client.Unlock()

	// Get the existing manifest work
	manifestWork, err := mw.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// actual deletion should be done after hub receive delete status
	deletedObj := manifestWork.DeepCopy()
	now := metav1.Now()
	deletedObj.DeletionTimestamp = &now
	if err := mw.client.mqClient.Publish(ctx, deletedObj); err != nil {
		klog.Errorf("failed to publish manifestwork spec, %v", err)
		return err
	}

	return nil
}

func (mw *MQManifestWorks) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	klog.Infof("TODO deleting manifest work collection")
	return nil
}

func (mw *MQManifestWorks) Get(ctx context.Context, name string, opts metav1.GetOptions) (*workv1.ManifestWork, error) {
	klog.Infof("Getting manifest work")
	manifestWork, err := mw.client.mqClient.GetByKey(mw.namespace, name)
	if err != nil {
		return nil, err
	}

	return manifestWork, nil
}

func (mw *MQManifestWorks) List(ctx context.Context, opts metav1.ListOptions) (*workv1.ManifestWorkList, error) {
	klog.Infof("TODO Listing manifest work")
	return &workv1.ManifestWorkList{}, nil
}

func (mw *MQManifestWorks) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	klog.Infof("Watching manifest work")
	return mw.client.watcher, nil
}

func (mw *MQManifestWorks) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *workv1.ManifestWork, err error) {
	mw.client.Lock()
	defer mw.client.Unlock()

	// Get the existing manifest work
	existingManifestWork, err := mw.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Marshal the existing manifest work to JSON
	existingJSON, err := json.Marshal(existingManifestWork)
	if err != nil {
		return nil, err
	}

	var patchedManifestWorkJSON []byte
	// Apply the patch based on the patch type
	switch pt {
	case types.JSONPatchType:
		patchedData, err := jsonpatch.MergePatch(existingJSON, data)
		if err != nil {
			return nil, err
		}
		patchedManifestWorkJSON, err = strategicpatch.StrategicMergePatch(existingJSON, patchedData, workv1.ManifestWork{})
		if err != nil {
			return nil, err
		}
	case types.MergePatchType:
		patchedManifestWorkJSON, err = strategicpatch.StrategicMergePatch(existingJSON, data, workv1.ManifestWork{})
		if err != nil {
			return nil, err
		}
	case types.StrategicMergePatchType:
		patchedManifestWorkJSON, err = strategicpatch.StrategicMergePatch(existingJSON, data, workv1.ManifestWork{})
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported patch type: %s", pt)
	}

	patchedManifestWork := &workv1.ManifestWork{}
	if err := json.Unmarshal(patchedManifestWorkJSON, patchedManifestWork); err != nil {
		return nil, err
	}

	currentResoureVersion, ok := patchedManifestWork.Annotations["manifestworkreplicaset-generation"]
	if !ok {
		return nil, fmt.Errorf("manifestworkreplicaset generation is required")
	}

	patchedManifestWork.ResourceVersion = currentResoureVersion
	if err := mw.client.mqClient.Publish(ctx, patchedManifestWork); err != nil {
		klog.Errorf("failed to publish manifestwork spec, %v", err)
		return nil, err
	}

	klog.Infof("Patching manifest work %v", patchedManifestWork.ObjectMeta)
	mw.client.watcher.Receive(watch.Event{
		Type:   watch.Modified,
		Object: patchedManifestWork,
	})

	return patchedManifestWork, nil
}
