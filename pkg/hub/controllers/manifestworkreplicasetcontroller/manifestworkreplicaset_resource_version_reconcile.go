package manifestworkreplicasetcontroller

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	workapiv1alpha1 "open-cluster-management.io/api/work/v1alpha1"
	"strconv"
)

// resourceVersionReconciler is to add manifestwork template resource version to the manifestworkreplicaset.
type resourceVersionReconciler struct {
	workClient workclientset.Interface
}

func (r *resourceVersionReconciler) reconcile(ctx context.Context, mwrSet *workapiv1alpha1.ManifestWorkReplicaSet) (*workapiv1alpha1.ManifestWorkReplicaSet, reconcileState, error) {
	manifestJSON, err := json.Marshal(mwrSet.Spec.ManifestWorkTemplate)
	if err != nil {
		return mwrSet, reconcileStop, fmt.Errorf("Failed to marshal manifestwork template")
	}

	manifestHash := fmt.Sprintf("%x", sha256.Sum256(manifestJSON))

	modified := false
	resourceVersion := int64(1)
	annotations := mwrSet.GetAnnotations()
	foundManifestHash, hashExisting := annotations[ManifestWorkReplicaSetControllerTemplateHashAnnotationKey]
	foundManifestResourceVersionStr, resourceVersionExisting := annotations[ManifestWorkReplicaSetControllerTemplateResourceVersionAnnotationKey]
	switch {
	case !hashExisting && !resourceVersionExisting:
		annotations[ManifestWorkReplicaSetControllerTemplateHashAnnotationKey] = manifestHash
		annotations[ManifestWorkReplicaSetControllerTemplateResourceVersionAnnotationKey] = strconv.FormatInt(resourceVersion, 10)
		modified = true
	case !hashExisting && resourceVersionExisting:
		// also update resource version when manifest template hash is not found
		// this should never happen
		foundManifestResourceVersion, err := strconv.ParseInt(foundManifestResourceVersionStr, 10, 64)
		if err != nil {
			return mwrSet, reconcileStop, fmt.Errorf("Invalid annotation value for %s", ManifestWorkReplicaSetControllerTemplateResourceVersionAnnotationKey)
		}
		resourceVersion = foundManifestResourceVersion + 1
		annotations[ManifestWorkReplicaSetControllerTemplateHashAnnotationKey] = manifestHash
		annotations[ManifestWorkReplicaSetControllerTemplateResourceVersionAnnotationKey] = strconv.FormatInt(resourceVersion, 10)
		modified = true
	case hashExisting && !resourceVersionExisting:
		// also update resource version when manifest template resource version is not found
		// this should never happen
		annotations[ManifestWorkReplicaSetControllerTemplateHashAnnotationKey] = manifestHash
		annotations[ManifestWorkReplicaSetControllerTemplateResourceVersionAnnotationKey] = strconv.FormatInt(resourceVersion, 10)
		modified = true
	case hashExisting && resourceVersionExisting:
		if manifestHash != foundManifestHash {
			foundManifestResourceVersion, err := strconv.ParseInt(foundManifestResourceVersionStr, 10, 64)
			if err != nil {
				return mwrSet, reconcileStop, fmt.Errorf("Invalid annotation value for %s", ManifestWorkReplicaSetControllerTemplateResourceVersionAnnotationKey)
			}
			resourceVersion = foundManifestResourceVersion + 1
			annotations[ManifestWorkReplicaSetControllerTemplateHashAnnotationKey] = manifestHash
			annotations[ManifestWorkReplicaSetControllerTemplateResourceVersionAnnotationKey] = strconv.FormatInt(resourceVersion, 10)
			modified = true
		}
	}

	if modified {
		_, err = r.workClient.WorkV1alpha1().ManifestWorkReplicaSets(mwrSet.Namespace).Update(ctx, mwrSet, metav1.UpdateOptions{})
		return mwrSet, reconcileStop, err
	}

	return mwrSet, reconcileContinue, err
}
