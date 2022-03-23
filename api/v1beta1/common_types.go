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

package v1beta1

import (
	"time"

	goClient "sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// APIAction - typedef to enumerate API verbs
type APIAction string

const (
	// APIActionCreate - "create" API verb
	APIActionCreate APIAction = "create"
	// APIActionUpdate - "update" API verb
	APIActionUpdate APIAction = "update"
	// APIActionDelete - "delete" API verb
	APIActionDelete APIAction = "delete"
)

// ConditionDetails used for passing condition information into generic functions
// e.f. GetDataFromSecret
type ConditionDetails struct {
	ConditionNotFoundType   ConditionType
	ConditionNotFoundReason ConditionReason
	ConditionErrorType      ConditionType
	ConditionErrordReason   ConditionReason
}

const (
	//
	// condition types
	//

	// CommonCondTypeEmpty - special state for 0 requested resources and 0 already provisioned
	CommonCondTypeEmpty ConditionType = "Empty"
	// CommonCondTypeWaiting - something is causing the CR to wait
	CommonCondTypeWaiting ConditionType = "Waiting"
	// CommonCondTypeProvisioning - one or more resoources are provisioning
	CommonCondTypeProvisioning ConditionType = "Provisioning"
	// CommonCondTypeProvisioned - the requested resource count has been satisfied
	CommonCondTypeProvisioned ConditionType = "Provisioned"
	// CommonCondTypeDeprovisioning - one or more resources are deprovisioning
	CommonCondTypeDeprovisioning ConditionType = "Deprovisioning"
	// CommonCondTypeError - general catch-all for actual errors
	CommonCondTypeError ConditionType = "Error"
	// CommonCondTypeCreated - general resource created
	CommonCondTypeCreated ConditionType = "Created"

	//
	// condition reasones
	//

	// CommonCondReasonInit - new resource set to reason Init
	CommonCondReasonInit ConditionReason = "CommonInit"
	// CommonCondReasonDeploymentSecretMissing - deployment secret does not exist
	CommonCondReasonDeploymentSecretMissing ConditionReason = "DeploymentSecretMissing"
	// CommonCondReasonDeploymentSecretError - deployment secret error
	CommonCondReasonDeploymentSecretError ConditionReason = "DeploymentSecretError"
	// CommonCondReasonSecretMissing - secret does not exist
	CommonCondReasonSecretMissing ConditionReason = "SecretMissing"
	// CommonCondReasonSecretError - secret error
	CommonCondReasonSecretError ConditionReason = "SecretError"
	// CommonCondReasonSecretDeleteError - secret deletion error
	CommonCondReasonSecretDeleteError ConditionReason = "SecretDeleteError"
	// CommonCondReasonConfigMapMissing - config map does not exist
	CommonCondReasonConfigMapMissing ConditionReason = "ConfigMapMissing"
	// CommonCondReasonConfigMapError - config map error
	CommonCondReasonConfigMapError ConditionReason = "ConfigMapError"
	// CommonCondReasonIdmSecretMissing - idm secret does not exist
	CommonCondReasonIdmSecretMissing ConditionReason = "IdmSecretMissing"
	// CommonCondReasonIdmSecretError - idm secret error
	CommonCondReasonIdmSecretError ConditionReason = "IdmSecretError"
	// CommonCondReasonCAConfigMapMissing - ca config map does not exist
	CommonCondReasonCAConfigMapMissing ConditionReason = "CAConfigMapMissing"
	// CommonCondReasonCAConfigMapError - ca config map error
	CommonCondReasonCAConfigMapError ConditionReason = "CAConfigMapError"
	// CommonCondReasonNewHostnameError - error creating new hostname
	CommonCondReasonNewHostnameError ConditionReason = "NewHostnameError"
	// CommonCondReasonCRStatusUpdateError - error updating CR status
	CommonCondReasonCRStatusUpdateError ConditionReason = "CRStatusUpdateError"
	// CommonCondReasonNNCPError - NNCP error
	CommonCondReasonNNCPError ConditionReason = "NNCPError"
	// CommonCondReasonOSNetNotFound - openstack network not found
	CommonCondReasonOSNetNotFound ConditionReason = "OSNetNotFound"
	// CommonCondReasonOSNetError - openstack network error
	CommonCondReasonOSNetError ConditionReason = "OSNetError"
	// CommonCondReasonOSNetWaiting - openstack network waiting
	CommonCondReasonOSNetWaiting ConditionReason = "OSNetWaiting"
	// CommonCondReasonOSNetAvailable - openstack networks available
	CommonCondReasonOSNetAvailable ConditionReason = "OSNetAvailable"
	// CommonCondReasonControllerReferenceError - error set controller reference on object
	CommonCondReasonControllerReferenceError ConditionReason = "ControllerReferenceError"
	// CommonCondReasonOwnerRefLabeledObjectsDeleteError - error deleting object using OwnerRef label
	CommonCondReasonOwnerRefLabeledObjectsDeleteError ConditionReason = "OwnerRefLabeledObjectsDeleteError"
	// CommonCondReasonRemoveFinalizerError - error removing finalizer from object
	CommonCondReasonRemoveFinalizerError ConditionReason = "RemoveFinalizerError"
	// CommonCondReasonAddRefLabelError - error adding reference label
	CommonCondReasonAddRefLabelError ConditionReason = "AddRefLabelError"
	// CommonCondReasonAddOSNetLabelError - error adding osnet labels
	CommonCondReasonAddOSNetLabelError ConditionReason = "AddOSNetLabelError"
	// CommonCondReasonCIDRParseError - could not parse CIDR
	CommonCondReasonCIDRParseError ConditionReason = "CIDRParseError"
	// CommonCondReasonServiceNotFound - service not found
	CommonCondReasonServiceNotFound ConditionReason = "ServiceNotFound"

	//
	// LabelSelectors
	//

	// NetworkNameLabelSelector -
	NetworkNameLabelSelector = "ooo-netname"
	// NetworkNameLowerLabelSelector -
	NetworkNameLowerLabelSelector = "ooo-netname-lower"
	// SubNetNameLabelSelector -
	SubNetNameLabelSelector = "ooo-subnetname"
	// ControlPlaneNetworkLabelSelector - is the network a ctlplane network?
	ControlPlaneNetworkLabelSelector = "ooo-ctlplane-network"
	// OpenStackNetConfigReconcileLabel - label set on objects on which change
	// trigger a reconcile of the osnetconfig
	OpenStackNetConfigReconcileLabel = "osnetconfig-reconcile-dep"
)

// Hash - struct to add hashes to status
type Hash struct {
	// Name of hash referencing the parameter
	Name string `json:"name,omitempty"`
	// Hash
	Hash string `json:"hash,omitempty"`
}

// ProvisioningState - the overall state of all VMs in this OpenStackVmSet
type ProvisioningState string

const (
	// HostRefInitState - intial HostRef state of a new node which has not yet assigned
	HostRefInitState string = "unassigned"
)

// HostStatus represents the hostname and IP info for a specific host
type HostStatus struct {
	Hostname          string            `json:"hostname"`
	ProvisioningState ProvisioningState `json:"provisioningState"`

	// +kubebuilder:default=unassigned
	HostRef string `json:"hostRef"`

	// +kubebuilder:validation:Optional
	IPAddresses map[string]string `json:"ipaddresses"`

	// +kubebuilder:default=false
	// Host annotated for deletion
	AnnotatedForDeletion bool `json:"annotatedForDeletion"`

	UserDataSecretName    string `json:"userDataSecretName"`
	NetworkDataSecretName string `json:"networkDataSecretName"`
	CtlplaneIP            string `json:"ctlplaneIP"`
}

// NetworkStatus represents the network details of a network
type NetworkStatus struct {
	Cidr string `json:"cidr"`

	// +kubebuilder:validation:Optional
	Vlan int `json:"vlan"`

	AllocationStart string `json:"allocationStart"`
	AllocationEnd   string `json:"allocationEnd"`

	// +kubebuilder:validation:Optional
	Gateway string `json:"gateway"`
}

// Conditions for status in web console

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

// OpenStackBackupOverridesReconcile - Should a controller pause reconciliation for a particular resource given potential backup operations?
func OpenStackBackupOverridesReconcile(client goClient.Client, namespace string, resourceReady bool) (bool, error) {
	var backupRequests *OpenStackBackupRequestList

	backupRequests, err := GetOpenStackBackupRequestsWithLabel(client, namespace, map[string]string{})

	if err != nil {
		return true, err
	}

	for _, backup := range backupRequests.Items {
		// If this backup is quiescing...
		// - If this CR has reached its "finished" state, end this reconcile
		// If this backup is saving or loading...
		// - End this reconcile
		if backup.Status.CurrentState == BackupSaving ||
			backup.Status.CurrentState == BackupLoading ||
			(backup.Status.CurrentState == BackupQuiescing && resourceReady) {
			return true, nil
		}
	}

	return false, nil
}
