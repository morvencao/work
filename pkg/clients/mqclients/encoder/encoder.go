package encoder

import (
	"encoding/json"
	"fmt"

	workv1 "open-cluster-management.io/api/work/v1"

	"open-cluster-management.io/work/pkg/clients/mqclients/payload"
)

type Encoder interface {
	Encode(*workv1.ManifestWork) ([]byte, error)
}

type MQTTEncoder struct {
}

func NewMQTTEncoder() *MQTTEncoder {
	return &MQTTEncoder{}
}

func (d *MQTTEncoder) Encode(work *workv1.ManifestWork) ([]byte, error) {
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

	return json.Marshal(manifestPayload)
}
