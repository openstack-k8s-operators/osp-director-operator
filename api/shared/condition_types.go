/*
Copyright 2020 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// +kubebuilder:object:generate:=true

package shared

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Conditions for status in web console

// ConditionDetails used for passing condition information into generic functions
// e.f. GetDataFromSecret
type ConditionDetails struct {
	ConditionNotFoundType   ConditionType
	ConditionNotFoundReason ConditionReason
	ConditionErrorType      ConditionType
	ConditionErrordReason   ConditionReason
}

// ConditionList - A list of conditions
type ConditionList []Condition

// Condition - A particular overall condition of a certain resource
type Condition struct {
	Type               ConditionType          `json:"type"`
	Status             corev1.ConditionStatus `json:"status"`
	Reason             ConditionReason        `json:"reason,omitempty"`
	Message            string                 `json:"message,omitempty"`
	LastHeartbeatTime  metav1.Time            `json:"lastHearbeatTime,omitempty"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty"`
}

// ConditionType - A summarizing name for a given condition
type ConditionType string

// ConditionReason - Why a particular condition is true, false or unknown
type ConditionReason string

// NewCondition - Create a new condition object
func NewCondition(conditionType ConditionType, status corev1.ConditionStatus, reason ConditionReason, message string) Condition {
	now := metav1.Time{Time: time.Now()}
	condition := Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastHeartbeatTime:  now,
		LastTransitionTime: now,
	}
	return condition
}

// Set - Set a particular condition in a given condition list
func (conditions *ConditionList) Set(conditionType ConditionType, status corev1.ConditionStatus, reason ConditionReason, message string) {
	condition := conditions.Find(conditionType)

	// If there isn't condition we want to change, add new one
	if condition == nil {
		condition := NewCondition(conditionType, status, reason, message)
		*conditions = append(*conditions, condition)
		return
	}

	now := metav1.Time{Time: time.Now()}

	// If there is different status, reason or message update it
	if condition.Status != status || condition.Reason != reason || condition.Message != message {
		condition.Status = status
		condition.Reason = reason
		condition.Message = message
		condition.LastTransitionTime = now
	}
	condition.LastHeartbeatTime = now
}

// Find - Check for the existence of a particular condition type in a list of conditions
func (conditions ConditionList) Find(conditionType ConditionType) *Condition {
	for i, condition := range conditions {
		if condition.Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// InitCondition - Either return the current condition (if non-nil), or return an empty Condition
func (conditions ConditionList) InitCondition() *Condition {
	cond := conditions.GetCurrentCondition()

	if cond == nil {
		return &Condition{
			Type:    CommonCondTypeEmpty,
			Reason:  CommonCondReasonInit,
			Message: string(CommonCondReasonInit),
		}
	}

	return cond.DeepCopy()
}

// GetCurrentCondition - Get current condition with status == corev1.ConditionTrue
func (conditions ConditionList) GetCurrentCondition() *Condition {
	for i, cond := range conditions {
		if cond.Status == corev1.ConditionTrue {
			return &conditions[i]
		}
	}

	return nil
}

// UpdateCurrentCondition - update current state condition, and sets previous condition to corev1.ConditionFalse
func (conditions *ConditionList) UpdateCurrentCondition(conditionType ConditionType, reason ConditionReason, message string) {
	//
	// get current condition and update to corev1.ConditionFalse
	//
	currentCondition := conditions.GetCurrentCondition()
	if currentCondition != nil {
		conditions.Set(
			ConditionType(currentCondition.Type),
			corev1.ConditionFalse,
			currentCondition.Reason,
			currentCondition.Message,
		)
	}

	//
	// set new corev1.ConditionTrue condition
	//
	conditions.Set(
		conditionType,
		corev1.ConditionTrue,
		reason,
		message,
	)
}
