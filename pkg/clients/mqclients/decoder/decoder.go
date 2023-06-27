package decoder

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	workv1 "open-cluster-management.io/api/work/v1"
)

type Decoder interface {
	Decode(data []byte) (*workv1.ManifestWork, error)
}

type MQTTDecoder struct {
	ClusterName string
}

func (d *MQTTDecoder) Decode(data []byte) (*workv1.ManifestWork, error) {
	payload := map[string]any{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload, %v", err)
	}

	resourceID, ok := payload["resourceID"]
	if !ok {
		return nil, fmt.Errorf("failed to find resourceID from payload")
	}

	resourceVersion, ok := payload["resourceVersion"]
	if !ok {
		return nil, fmt.Errorf("failed to find resourceVersion from payload")
	}

	generation, err := strconv.ParseInt(fmt.Sprintf("%v", resourceVersion), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("faild to parse resourceVersion from payload, %v", err)
	}

	work := &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:       fmt.Sprintf("%s", resourceID),
			Namespace:  d.ClusterName,
			UID:        types.UID(fmt.Sprintf("%s", resourceID)),
			Generation: generation,
		},
	}

	deletionTimestamp, ok := payload["deletionTimestamp"]
	if ok {
		var timestamp metav1.Time
		timestamp.UnmarshalQueryParameter(fmt.Sprintf("%s", deletionTimestamp))
		work.DeletionTimestamp = &timestamp

		// TODO
		// deletePolicy, ok := payload["deletePolicy"]
		// if ok {
		// 	policy, ok := deletePolicy.(workv1.DeletePropagationPolicyType)
		// 	if ok {
		// 		work.Spec.DeleteOption = &workv1.DeleteOption{
		// 			PropagationPolicy: policy,
		// 		}
		// 	}
		// }

		return work, nil
	}

	manifest, ok := payload["manifest"]
	if !ok {
		return nil, fmt.Errorf("failed to find manifest from payload")
	}

	unstructuredMap, ok := manifest.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("faild to convert manifest to unstructured map")
	}
	unstructuredObj := unstructured.Unstructured{Object: unstructuredMap}

	jsonData, err := json.Marshal(unstructuredObj.Object)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content, %v", err)
	}

	manifests := []workv1.Manifest{}
	manifests = append(manifests, workv1.Manifest{
		RawExtension: runtime.RawExtension{Raw: jsonData},
	})

	work.Spec = workv1.ManifestWorkSpec{
		Workload: workv1.ManifestsTemplate{
			Manifests: manifests,
		},
		ManifestConfigs: []workv1.ManifestConfigOption{
			{
				ResourceIdentifier: workv1.ResourceIdentifier{
					Group:     toGroup(unstructuredObj.GetAPIVersion()),
					Resource:  toResouce(unstructuredObj.GetKind()),
					Name:      unstructuredObj.GetName(),
					Namespace: unstructuredObj.GetNamespace(),
				},
				FeedbackRules: []workv1.FeedbackRule{
					{
						Type: workv1.WellKnownStatusType,
					},
				},
			},
		},
	}

	// TODO
	// updateStrategy, ok := payload["updateStrategy"]
	// if ok {
	// 	work.Spec.ManifestConfigs = []workv1.ManifestConfigOption{
	// 		{
	// 			ResourceIdentifier: workv1.ResourceIdentifier{
	// 				Group:     toGroup(unstructuredObj.GetAPIVersion()),
	// 				Resource:  toResouce(unstructuredObj.GetKind()),
	// 				Name:      unstructuredObj.GetName(),
	// 				Namespace: unstructuredObj.GetNamespace(),
	// 			},
	// 			UpdateStrategy: &workv1.UpdateStrategy{
	// 				Type: workv1.UpdateStrategyType(fmt.Sprintf("%s", updateStrategy)),
	// 			},
	// 		},
	// 	}
	// }

	return work, nil
}

// TODO need a way to refactor this
func toGroup(apiVersion string) string {
	gv := strings.Split(apiVersion, "/")
	if len(gv) == 1 {
		return ""
	}

	return gv[0]
}

// TODO need a way to refactor this
func toResouce(kind string) string {
	return strings.ToLower(kind) + "s"
}
