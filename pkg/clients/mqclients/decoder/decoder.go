package decoder

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/work/pkg/clients/mqclients/payload"
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

	work.Spec = workv1.ManifestWorkSpec{
		Workload: workv1.ManifestsTemplate{
			Manifests: payloadObj.Manifests,
		},
		DeleteOption:    payloadObj.DeleteOption,
		ManifestConfigs: payloadObj.ManifestConfigs,
	}

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
				Manifests: payloadObj.ResourceStatus,
			},
		},
	}

	return work, nil
}
