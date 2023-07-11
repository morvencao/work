package payload

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	workv1 "open-cluster-management.io/api/work/v1"
)

const (
	ResourceConditionTypeApplied = "Applied"

	ResourceConditionTypeAvailable = "Available"

	ResourceConditionTypeDeleted = "Deleted"
)

type ManifestPayload struct {
	ResourceName      string                        `json:"resourceName,omitempty"`
	ResourceID        string                        `json:"resourceID,omitempty"`
	ResourceVersion   int64                         `json:"resourceVersion,omitempty"`
	DeletionTimestamp *metav1.Time                  `json:"deletionTimestamp,omitempty"`
	Manifests         []workv1.Manifest             `json:"manifests,omitempty"`
	DeleteOption      *workv1.DeleteOption          `json:"deleteOption,omitempty"`
	ManifestConfigs   []workv1.ManifestConfigOption `json:"manifestConfigs,omitempty"`
}

type ManifestStatus struct {
	ClusterName        string                     `json:"clusterName,omitempty"`
	ResourceName       string                     `json:"resourceName,omitempty"`
	ResourceID         string                     `json:"resourceID,omitempty"`
	ResourceStatusHash string                     `json:"resourceStatusHash,omitempty"`
	ResourceVersion    int64                      `json:"resourceVersion,omitempty"`
	ResourceCondition  ResourceCondition          `json:"resourceCondition,omitempty"`
	ResourceStatus     []workv1.ManifestCondition `json:"resourceStatus,omitempty"`
}

type ResourceCondition struct {
	Type    string `json:"type,omitempty"`
	Status  string `json:"status,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}
