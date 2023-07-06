package encoder

import (
	"encoding/json"
	"fmt"

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
	resourceVersion := int64(1)
	if work.Generation > 0 {
		resourceVersion = work.Generation
	}

	manifest := map[string]any{}
	// TODO: handle multiple resources in a manifestwork
	if len(work.Spec.Workload.Manifests) > 0 {
		if err := json.Unmarshal(work.Spec.Workload.Manifests[0].Raw, &manifest); err != nil {
			return nil, fmt.Errorf("Failed to unmarshal manifests %v", err)
		}
	}

	updateStrategy := &workv1.UpdateStrategy{
		Type: workv1.UpdateStrategyTypeUpdate,
	}
	statusFeedbackRules := []workv1.FeedbackRule{
		{
			Type: workv1.WellKnownStatusType,
		},
	}

	if len(work.Spec.ManifestConfigs) > 0 {
		updateStrategy = work.Spec.ManifestConfigs[0].UpdateStrategy
		if len(work.Spec.ManifestConfigs[0].FeedbackRules) > 0 {
			statusFeedbackRules = work.Spec.ManifestConfigs[0].FeedbackRules
		}
	}

	deletePolicy := workv1.DeletePropagationPolicyTypeForeground
	if work.Spec.DeleteOption != nil {
		deletePolicy = work.Spec.DeleteOption.PropagationPolicy
	}
	manifestPayload := &payload.ManifestPayload{
		ResourceName:        work.Name,
		ResourceID:          string(work.UID),
		ResourceVersion:     resourceVersion,
		Manifest:            manifest,
		StatusFeedbackRules: statusFeedbackRules,
		UpdateStrategy:      updateStrategy,
		DeletePolicy:        deletePolicy,
	}

	if !work.DeletionTimestamp.IsZero() {
		manifestPayload.DeletionTimestamp = work.DeletionTimestamp
	}

	return json.Marshal(manifestPayload)
}

func (e *MQTTEncoder) EncodeStatus(work *workv1.ManifestWork) ([]byte, error) {
	statusPayload := &payload.ManifestStatus{
		ClusterName:     e.clusterName,
		ResourceName:    work.Name,
		ResourceID:      string(work.UID),
		ResourceVersion: work.Generation,
	}

	switch {
	case !work.DeletionTimestamp.IsZero() && len(work.Finalizers) == 0:
		statusPayload.ResourceCondition = payload.ResourceCondition{
			Type:    payload.ResourceConditionTypeDeleted,
			Status:  "True",
			Reason:  "Deleted",
			Message: fmt.Sprintf("The resouce is deleted from the cluster %s", e.clusterName),
		}
	case meta.IsStatusConditionTrue(work.Status.Conditions, workv1.WorkAvailable):
		statusPayload.ResourceCondition = payload.ResourceCondition{
			Type:    payload.ResourceConditionTypeAvailable,
			Status:  "True",
			Reason:  "Available",
			Message: fmt.Sprintf("The resouce is available on the cluster %s", e.clusterName),
		}
		if len(work.Status.ResourceStatus.Manifests) != 0 && len(work.Status.ResourceStatus.Manifests[0].StatusFeedbacks.Values) != 0 {
			statusPayload.ResourceStatus = payload.ResourceStatus{
				Values: work.Status.ResourceStatus.Manifests[0].StatusFeedbacks.Values,
			}
		}
	case meta.IsStatusConditionTrue(work.Status.Conditions, workv1.WorkApplied):
		statusPayload.ResourceCondition = payload.ResourceCondition{
			Type:    payload.ResourceConditionTypeApplied,
			Status:  "True",
			Reason:  "Applied",
			Message: fmt.Sprintf("The resouce is applied on the cluster %s", e.clusterName),
		}
	default:
		statusPayload.ResourceCondition = payload.ResourceCondition{
			Type:    payload.ResourceConditionTypeApplied,
			Status:  "False",
			Reason:  "Progressing",
			Message: fmt.Sprintf("The resouce is in the progress to be applied on the cluster %s", e.clusterName),
		}
	}

	return json.Marshal(statusPayload)
}
