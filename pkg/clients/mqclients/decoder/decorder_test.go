package decoder

import (
	"fmt"
	"log"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/work/pkg/spoke/spoketesting"
	"sigs.k8s.io/yaml"
)

var deployYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
  labels:
    app: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 80
`

func TestMQTTDecode(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		verify  func(t *testing.T, work *workv1.ManifestWork)
	}{
		{
			name:    "default payload",
			payload: toPayload("", "", false),
			verify: func(t *testing.T, work *workv1.ManifestWork) {
				if !work.DeletionTimestamp.IsZero() {
					t.Errorf("unexpected deletion timestamp")
				}

				config := work.Spec.ManifestConfigs[0]
				resourceIdentifier := config.ResourceIdentifier
				if resourceIdentifier.Group != "apps" {
					t.Errorf("unexpected group %v", resourceIdentifier)
				}

				if resourceIdentifier.Resource != "deployments" {
					t.Errorf("unexpected resoruce %v", resourceIdentifier)
				}

				if resourceIdentifier.Namespace != "default" {
					t.Errorf("unexpected namespace %v", resourceIdentifier)
				}

				if resourceIdentifier.Name != "nginx" {
					t.Errorf("unexpected name %v", resourceIdentifier)
				}

				if config.FeedbackRules[0].Type != workv1.WellKnownStatusType {
					t.Errorf("unexpected feed back rules, %v", config.FeedbackRules[0].Type)
				}

				if config.UpdateStrategy != nil {
					t.Errorf("unexpected strategy")
				}
			},
		},
		{
			name:    "deletion payload",
			payload: toPayload("", "", true),
			verify: func(t *testing.T, work *workv1.ManifestWork) {
				if work.DeletionTimestamp.IsZero() {
					t.Errorf("expected deletion timestamp")
				}
			},
		},
		{
			name:    "payload with deletion option",
			payload: toPayload("", "Orphan", false),
			verify: func(t *testing.T, work *workv1.ManifestWork) {
				//config := work.Spec.ManifestConfigs[0]
				if work.Spec.DeleteOption.PropagationPolicy != workv1.DeletePropagationPolicyTypeOrphan {
					t.Errorf("unexpected deleteOption, %v", work.Spec.DeleteOption)
				}

				// if config.UpdateStrategy.Type != workv1.UpdateStrategyTypeCreateOnly {
				// 	t.Errorf("unexpected updateStrategy, %v", config)
				// }
			},
		},
		{
			name:    "payload with update strategy",
			payload: toPayload("CreateOnly", "", false),
			verify: func(t *testing.T, work *workv1.ManifestWork) {
				config := work.Spec.ManifestConfigs[0]
				if config.UpdateStrategy.Type != workv1.UpdateStrategyTypeCreateOnly {
					t.Errorf("unexpected updateStrategy, %v", config)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mqttDecoder := NewMQTTDecoder("test", spoketesting.NewFakeRestMapper())
			work, err := mqttDecoder.Decode(c.payload)
			if err != nil {
				t.Errorf("%v", err)
			}

			c.verify(t, work)
		})
	}
}

func toPayload(updateStrategy, deletePolicy string, delete bool) []byte {
	msgs := []string{fmt.Sprintf("\"resourceID\":\"%s\"", uuid.NewUUID())}

	msgs = append(msgs, fmt.Sprintf("\"resourceVersion\":%d", 128))

	if delete {
		now := metav1.Now()
		msgs = append(msgs, fmt.Sprintf("\"deletionTimestamp\":\"%s\"", now.Format("2006-01-02T15:04:05Z")))
		return []byte(fmt.Sprintf("{%s}", strings.Join(msgs, ",")))
	}

	manifest, err := yaml.YAMLToJSON([]byte(deployYaml))
	if err != nil {
		log.Fatalf("Failed to convert resource yaml to json, %v", err)
	}

	msgs = append(msgs, fmt.Sprintf("\"manifest\":%s", manifest))

	msgs = append(msgs, "\"statusFeedbackRule\":{\"type\":\"WellKnownStatus\"}")

	if len(updateStrategy) != 0 {
		msgs = append(msgs, fmt.Sprintf("\"updateStrategy\":{\"type\":\"%s\"}", updateStrategy))
	}

	if len(deletePolicy) != 0 {
		msgs = append(msgs, fmt.Sprintf("\"deletePolicy\":\"%s\"", deletePolicy))
	}

	return []byte(fmt.Sprintf("{%s}", strings.Join(msgs, ",")))
}
