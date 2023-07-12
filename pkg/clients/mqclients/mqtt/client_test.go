package mqtt

import "testing"

func TestSplitTopic(t *testing.T) {
	cases := []struct {
		name         string
		topic        string
		expectedType string
	}{
		{
			name:         "manifests resync",
			topic:        "resync/+/manifests",
			expectedType: "manifests",
		},
		{
			name:         "manifests status resync",
			topic:        "resync/clusters/manifestsstatus",
			expectedType: "manifestsstatus",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, rtype := splitTopic(c.topic)
			if rtype != c.expectedType {
				t.Errorf("unexpected type %s", rtype)
			}
		})
	}
}
