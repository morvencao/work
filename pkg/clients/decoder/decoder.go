package decoder

import (
	"encoding/json"
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"

	workv1 "open-cluster-management.io/api/work/v1"
)

type Decoder interface {
	Decode(data []byte) (*workv1.ManifestWork, error)
}

type MQTTDecoder struct{}

// playload format: {resourceID:<uuid>,maestroGeneration:<generation>,content:<content>}
func (d *MQTTDecoder) Decode(data []byte) (*workv1.ManifestWork, error) {
	//reporter := errors.NewClientErrorReporter(http.StatusInternalServerError, "sub", "ClientWatchDecoding")
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

	jsonData, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content, %v", err)
	}

	manifests := []workv1.Manifest{}
	manifests = append(manifests, workv1.Manifest{
		RawExtension: runtime.RawExtension{Raw: jsonData},
	})

	return &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:       fmt.Sprintf("%s-%s", "cluster1", "test"),
			Namespace:  "cluster1",
			UID:        types.UID(fmt.Sprintf("%s", resourceID)),
			Generation: generation,
			//Finalizers: []string{controllers.ManifestWorkFinalizer},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: manifests,
			},
			DeleteOption: &workv1.DeleteOption{
				PropagationPolicy: workv1.DeletePropagationPolicyTypeOrphan,
			},
		},
	}, nil
}
