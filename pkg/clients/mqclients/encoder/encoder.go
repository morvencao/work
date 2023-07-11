package encoder

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/api/meta"
	workv1 "open-cluster-management.io/api/work/v1"

	"open-cluster-management.io/work/pkg/clients/mqclients/payload"
)

type Encoder interface {
	EncodeSpec(*workv1.ManifestWork) ([]byte, error)
	EncodeStatus(*workv1.ManifestWork) ([]byte, error)
}

type MQTTEncoder struct {
	clusterName string
}

func NewMQTTEncoder(clusterName string) *MQTTEncoder {
	return &MQTTEncoder{
		clusterName: clusterName,
	}
}

func (e *MQTTEncoder) EncodeSpec(work *workv1.ManifestWork) ([]byte, error) {
	resourceVersion, err := strconv.ParseInt(work.ResourceVersion, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse resource version, %v", err)
	}

	if !work.DeletionTimestamp.IsZero() {
		manifestPayload := &payload.ManifestPayload{
			ResourceName:      work.Name,
			ResourceID:        string(work.UID),
			ResourceVersion:   resourceVersion,
			DeletionTimestamp: work.DeletionTimestamp,
		}

		return json.Marshal(manifestPayload)
	}

	manifestPayload := &payload.ManifestPayload{
		ResourceName:    work.Name,
		ResourceID:      string(work.UID),
		ResourceVersion: resourceVersion,
		Manifests:       work.Spec.Workload.Manifests,
		DeleteOption:    work.Spec.DeleteOption,
		ManifestConfigs: work.Spec.ManifestConfigs,
	}

	return json.Marshal(manifestPayload)
}

func (e *MQTTEncoder) EncodeStatus(work *workv1.ManifestWork) ([]byte, error) {
	status, err := ToAggregatedStatus(work)
	if err != nil {
		return nil, err
	}

	status.ResourceStatusHash = work.Annotations["statushash"]
	return json.Marshal(status)
}

func ToAggregatedStatus(work *workv1.ManifestWork) (*payload.ManifestStatus, error) {
	resourceVersion, err := strconv.ParseInt(work.ResourceVersion, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse resource version, %v", err)
	}

	statusPayload := &payload.ManifestStatus{
		ClusterName:     work.Namespace,
		ResourceName:    work.Name,
		ResourceID:      string(work.UID),
		ResourceVersion: resourceVersion,
	}

	switch {
	case !work.DeletionTimestamp.IsZero() && len(work.Finalizers) == 0:
		statusPayload.ResourceCondition = payload.ResourceCondition{
			Type:    payload.ResourceConditionTypeDeleted,
			Status:  "True",
			Reason:  "Deleted",
			Message: fmt.Sprintf("The resouce is deleted from the cluster %s", work.Namespace),
		}
	case meta.IsStatusConditionTrue(work.Status.Conditions, workv1.WorkAvailable):
		statusPayload.ResourceCondition = payload.ResourceCondition{
			Type:    payload.ResourceConditionTypeAvailable,
			Status:  "True",
			Reason:  "Available",
			Message: fmt.Sprintf("The resouce is available on the cluster %s", work.Namespace),
		}
		statusPayload.ResourceStatus = work.Status.ResourceStatus.Manifests
	case meta.IsStatusConditionTrue(work.Status.Conditions, workv1.WorkApplied):
		statusPayload.ResourceCondition = payload.ResourceCondition{
			Type:    payload.ResourceConditionTypeApplied,
			Status:  "True",
			Reason:  "Applied",
			Message: fmt.Sprintf("The resouce is applied on the cluster %s", work.Namespace),
		}
	default:
		statusPayload.ResourceCondition = payload.ResourceCondition{
			Type:    payload.ResourceConditionTypeApplied,
			Status:  "False",
			Reason:  "Progressing",
			Message: fmt.Sprintf("The resouce is in the progress to be applied on the cluster %s", work.Namespace),
		}
	}

	return statusPayload, nil
}

func GetStatusHash(work *workv1.ManifestWork) (string, error) {
	status, err := ToAggregatedStatus(work)
	if err != nil {
		return "", err
	}

	statusBytes, err := json.Marshal(status)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(statusBytes)), nil
}
