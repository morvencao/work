package decoder

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/work/pkg/clients/mqclients/payload"
	"open-cluster-management.io/work/pkg/helper"
)

type Decoder interface {
	DecodeSpec(data []byte) (*workv1.ManifestWork, error)
	DecodeStatus(data []byte) (*workv1.ManifestWork, error)
}

type MQTTDecoder struct {
	clusterName string
	restMapper  meta.RESTMapper
}

func NewMQTTDecoder(clusterName string, restMapper meta.RESTMapper) *MQTTDecoder {
	return &MQTTDecoder{
		clusterName: clusterName,
		restMapper:  restMapper,
	}
}

func (d *MQTTDecoder) DecodeSpec(data []byte) (*workv1.ManifestWork, error) {
	payloadObj := payload.ManifestPayload{}
	if err := json.Unmarshal(data, &payloadObj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload %s, %v", string(data), err)
	}

	work := &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:            payloadObj.ResourceName,
			Namespace:       d.clusterName,
			UID:             types.UID(payloadObj.ResourceID),
			ResourceVersion: fmt.Sprintf("%d", payloadObj.ResourceVersion),
			Generation:      payloadObj.ResourceVersion,
		},
	}

	if payloadObj.DeletionTimestamp != nil {
		work.DeletionTimestamp = payloadObj.DeletionTimestamp
		return work, nil
	}

	unstructuredObj := &unstructured.Unstructured{Object: payloadObj.Manifest}
	manifestData, err := json.Marshal(unstructuredObj.Object)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content from payload %s, %v", string(data), err)
	}

	_, gvr, err := helper.BuildResourceMeta(0, unstructuredObj, d.restMapper)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest GVR from payload %s, %v", string(data), err)
	}

	resourceIdentifier := workv1.ResourceIdentifier{
		Group:     gvr.Group,
		Resource:  gvr.Resource,
		Name:      unstructuredObj.GetName(),
		Namespace: unstructuredObj.GetNamespace(),
	}

	work.Spec = workv1.ManifestWorkSpec{
		Workload: workv1.ManifestsTemplate{
			Manifests: []workv1.Manifest{
				{RawExtension: runtime.RawExtension{Raw: manifestData}},
			},
		},
	}

	if len(payloadObj.DeletePolicy) != 0 {
		work.Spec.DeleteOption = &workv1.DeleteOption{PropagationPolicy: payloadObj.DeletePolicy}
	}

	manifestConfigOption := workv1.ManifestConfigOption{
		ResourceIdentifier: resourceIdentifier,
	}

	manifestConfigOption.FeedbackRules = payloadObj.StatusFeedbackRules

	if payloadObj.UpdateStrategy != nil {
		manifestConfigOption.UpdateStrategy = payloadObj.UpdateStrategy
	}

	work.Spec.ManifestConfigs = []workv1.ManifestConfigOption{manifestConfigOption}
	return work, nil
}

func (d *MQTTDecoder) DecodeStatus(data []byte) (*workv1.ManifestWork, error) {
	payloadObj := payload.ManifestStatus{}
	if err := json.Unmarshal(data, &payloadObj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload %s, %v", string(data), err)
	}

	work := &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:            payloadObj.ResourceName,
			Namespace:       payloadObj.ClusterName,
			UID:             types.UID(payloadObj.ResourceID),
			ResourceVersion: fmt.Sprintf("%d", payloadObj.ResourceVersion),
			Generation:      payloadObj.ResourceVersion,
			Annotations: map[string]string{
				"statushash": payloadObj.ResourceStatusHash,
			},
		},
		Status: workv1.ManifestWorkStatus{
			Conditions: []metav1.Condition{
				{
					Type:    payloadObj.ResourceCondition.Type,
					Status:  metav1.ConditionStatus(payloadObj.ResourceCondition.Status),
					Reason:  payloadObj.ResourceCondition.Reason,
					Message: payloadObj.ResourceCondition.Message,
				},
			},
			ResourceStatus: workv1.ManifestResourceStatus{
				Manifests: []workv1.ManifestCondition{
					{
						StatusFeedbacks: workv1.StatusFeedbackResult{
							Values: payloadObj.ResourceStatus.Values,
						},
					},
				},
			},
		},
	}

	return work, nil
}
