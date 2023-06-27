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
	ResourceID          string                             `json:"resourceID"`
	ResourceVersion     int64                              `json:"resourceVersion"`
	Manifest            []byte                             `json:"manifest"`
	StatusFeedbackRules workv1.FeedbackRule                `json:"statusFeedbackRule"`
	UpdateStrategy      workv1.UpdateStrategyType          `json:"updateStrategy"`
	DeletionTimestamp   metav1.Time                        `json:"deletionTimestamp"`
	DeletePolicy        workv1.DeletePropagationPolicyType `json:"deletePolicy"`
}

type ManifestStatus struct {
	ResourceID        string            `json:"resourceID"`
	ResourceVersion   int64             `json:"resourceVersion"`
	ResourceCondition ResourceCondition `json:"resourceCondition"`
	ResourceStatus    ResourceStatus    `json:"resourceStatus"`
}

type ResourceCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type ResourceStatus struct {
	Values []workv1.FeedbackValue `json:"values"`
}
