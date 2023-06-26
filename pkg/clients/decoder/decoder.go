package decoder

import (
	"encoding/json"
	"fmt"
	"strconv"

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

// playload format: {resourceID:<uuid>,maestroGeneration:<generation>,content:<content>}
func (d *MQTTDecoder) Decode(data []byte) (*workv1.ManifestWork, error) {
	payload := map[string]any{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload, %v", err)
	}

	resourceID, ok := payload["resourceID"]
	if !ok {
		return nil, fmt.Errorf("failed to find resourceID from payload")
	}

	maestroGeneration, ok := payload["maestroGeneration"]
	if !ok {
		return nil, fmt.Errorf("failed to find maestroGeneration from payload")
	}

	generation, err := strconv.ParseInt(fmt.Sprintf("%v", maestroGeneration), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("faild to parse maestroGeneration from payload, %v", err)
	}

	content, ok := payload["content"]
	if !ok {
		return nil, fmt.Errorf("failed to find content from payload")
	}

	unstructuredMap, ok := content.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("faild to convert content to unstructured map")
	}
	unstructuredObj := unstructured.Unstructured{Object: unstructuredMap}

	name := fmt.Sprintf("%s-%s", d.ClusterName, unstructuredObj.GetName())
	if unstructuredObj.GetNamespace() != "" {
		name = fmt.Sprintf("%s-%s-%s", d.ClusterName, unstructuredObj.GetNamespace(), unstructuredObj.GetName())
	}

	work := &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  d.ClusterName,
			UID:        types.UID(fmt.Sprintf("%s", resourceID)),
			Generation: generation,
		},
	}

	deletionTimestamp := unstructuredObj.GetDeletionTimestamp()
	if deletionTimestamp != nil {
		work.DeletionTimestamp = deletionTimestamp
		unstructuredObj.SetDeletionTimestamp(nil)
	}

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
	}

	return work, nil
}
