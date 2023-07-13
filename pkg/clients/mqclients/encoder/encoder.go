package encoder

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"

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

	if !work.DeletionTimestamp.IsZero() && len(work.Finalizers) == 0 {
		statusPayload.Conditions = []payload.ResourceCondition{
			{
				Type:    payload.ResourceConditionTypeDeleted,
				Status:  "True",
				Reason:  "Deleted",
				Message: fmt.Sprintf("The resouce is deleted from the cluster %s", work.Namespace),
			},
		}
	} else {
		statusPayload.Conditions = make([]payload.ResourceCondition, len(work.Status.Conditions))
		for i, cond := range work.Status.Conditions {
			statusPayload.Conditions[i] = payload.ResourceCondition{
				Type:    cond.Type,
				Status:  string(cond.Status),
				Reason:  cond.Reason,
				Message: cond.Message,
			}
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
