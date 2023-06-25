package decoder

import (
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"

	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/work/pkg/spoke/controllers"
)

type Decoder interface {
	Decode(data []byte) watch.Event
}

type MQTTDecoder struct{}

// TODO:
// resourceVersion := meta.GetResourceVersion()
// watch.Added
// watch.Modified
// watch.Deleted
func (d *MQTTDecoder) Decode(data []byte) watch.Event {
	reporter := errors.NewClientErrorReporter(http.StatusInternalServerError, "sub", "ClientWatchDecoding")
	payload := map[string]any{}
	if err := json.Unmarshal(data, &payload); err != nil {
		klog.Errorf("failed to unmarshal payload %s, %v", string(data), err)
		return watch.Event{
			Type:   watch.Error,
			Object: reporter.AsObject(fmt.Errorf("unable to decode an event from the watch stream: %v", err)),
		}
	}

	content, ok := payload["content"]
	if !ok {
		return watch.Event{
			Type:   watch.Error,
			Object: reporter.AsObject(fmt.Errorf("unable to decode an event from the watch stream")),
		}
	}

	jsonData, err := json.Marshal(content)
	if err != nil {
		return watch.Event{
			Type:   watch.Error,
			Object: reporter.AsObject(fmt.Errorf("unable to decode an event from the watch stream")),
		}
	}

	manifests := []workv1.Manifest{}
	manifests = append(manifests, workv1.Manifest{
		RawExtension: runtime.RawExtension{Raw: jsonData},
	})

	work := &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:       fmt.Sprintf("%s-%s", "cluster1", "test"),
			Namespace:  "cluster1",
			Finalizers: []string{controllers.ManifestWorkFinalizer},
			// Labels: map[string]string{
			// 	constants.KlusterletWorksLabel: "true",
			// },
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: manifests,
			},
			DeleteOption: &workv1.DeleteOption{
				PropagationPolicy: workv1.DeletePropagationPolicyTypeOrphan,
			},
		},
	}
	return watch.Event{
		Type:   watch.Added,
		Object: work,
	}
}
