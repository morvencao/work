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
	ResourceName        string                             `json:"resourceName,omitempty"`
	ResourceID          string                             `json:"resourceID,omitempty"`
	ResourceVersion     int64                              `json:"resourceVersion,omitempty"`
	DeletionTimestamp   *metav1.Time                       `json:"deletionTimestamp,omitempty"`
	Manifest            map[string]any                     `json:"manifest,omitempty"`
	StatusFeedbackRules []workv1.FeedbackRule              `json:"statusFeedbackRule,omitempty"`
	UpdateStrategy      *workv1.UpdateStrategy             `json:"updateStrategy,omitempty"`
	DeletePolicy        workv1.DeletePropagationPolicyType `json:"deletePolicy,omitempty"`
}

type ManifestStatus struct {
	ClusterName       string            `json:"clusterName,omitempty"`
	ResourceName      string            `json:"resourceName,omitempty"`
	ResourceID        string            `json:"resourceID,omitempty"`
	ResourceVersion   int64             `json:"resourceVersion,omitempty"`
	ResourceCondition ResourceCondition `json:"resourceCondition,omitempty"`
	ResourceStatus    ResourceStatus    `json:"resourceStatus,omitempty"`
}

type ResourceCondition struct {
	Type    string `json:"type,omitempty"`
	Status  string `json:"status,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type ResourceStatus struct {
	Values []workv1.FeedbackValue `json:"values,omitempty"`
}
