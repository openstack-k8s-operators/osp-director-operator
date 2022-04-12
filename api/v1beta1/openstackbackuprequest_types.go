/*


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
	"context"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	goClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// OpenStackBackupRequestSpec defines the desired state of OpenStackBackupRequest
type OpenStackBackupRequestSpec struct {
	// Mode - what this backup should be doing (if it hasn't already done so)
	// +kubebuilder:default=save
	// +kubebuilder:validation:Enum={"save","restore","cleanRestore"}
	Mode BackupMode `json:"mode"`

	// If "Mode" is "save", optional list of additional config maps to save in the backup
	// If "Mode" is "cleanRestore", optional list of additional config maps to remove before restoring the backup
	AdditionalConfigMaps []string `json:"additionalConfigMaps,omitempty" optional:"true"`

	// If "Mode" is "save", optional list of additional secrets to save in the backup
	// If "Mode" is "cleanRestore", optional list of additional secrets to remove before restoring the backup
	AdditionalSecrets []string `json:"additionalSecrets,omitempty" optional:"true"`

	// Name of an OpenStackBackup to use if "Mode" is "restore" or "cleanRestore", in which case this field is required
	RestoreSource string `json:"restoreSource,omitempty" optional:"true"`
}

// OpenStackBackupRequestStatus defines the observed state of OpenStackBackupRequest
type OpenStackBackupRequestStatus struct {
	// CompletionTimestamp - If the request succeeded, the timestamp for that completion
	CompletionTimestamp metav1.Time `json:"completionTimestamp,omitempty" optional:"true"`

	// CurrentState - the overall state of this backup request
	CurrentState BackupState `json:"currentState"`

	// TODO: It would be simpler, perhaps, to just have Conditions and get rid of CurrentState,
	// but we are using the same approach in other CRDs for now anyhow
	// Conditions - conditions to display in the OpenShift GUI, which reflect CurrentState
	Conditions shared.ConditionList `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osbackuprequest;osbackuprequests
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Backup Request"
// +kubebuilder:printcolumn:name="Operation",type=string,JSONPath=`.spec.mode`,description="Operation"
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.restoreSource`,description="Source"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.currentState`,description="Status"
// +kubebuilder:printcolumn:name="Completion Timestamp",type=string,JSONPath=`.status.completionTimestamp`,description="Completion Timestamp"

// OpenStackBackupRequest a request to backup the OpenStack Director Operator
type OpenStackBackupRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackBackupRequestSpec   `json:"spec,omitempty"`
	Status OpenStackBackupRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackBackupRequestList contains a list of OpenStackBackupRequest
type OpenStackBackupRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackBackupRequest `json:"items"`
}

// BackupMode - whether the backup should be saved or restored
type BackupMode string

const (
	// BackupSave - save current operator config
	BackupSave BackupMode = "save"
	// BackupRestore - restore operator config contained in this backup
	BackupRestore BackupMode = "restore"
	// BackupCleanRestore - restore operator config contained in this backup after first deleting current config
	BackupCleanRestore BackupMode = "cleanRestore"
)

// BackupState - the state of this openstack network
type BackupState string

const (
	// BackupWaiting - the backup/restore is blocked by prerequisite objects
	BackupWaiting BackupState = "Waiting"
	// BackupQuiescing - the backup/restore is waiting for controllers to complete pending operations
	BackupQuiescing BackupState = "Quiescing"
	// BackupSaving - the backup is saving the current config of the operator
	BackupSaving BackupState = "Saving"
	// BackupSaved - the backup contains the saved config of the operator
	BackupSaved BackupState = "Saved"
	// BackupSaveError - the backup failed to save the operator config for some reason
	BackupSaveError BackupState = "Save Error"
	// BackupCleaning - the backup is waiting to restore until cleaning is completed
	BackupCleaning BackupState = "Cleaning"
	// BackupLoading - the backup is being loaded into the operator to prepare for restoring via reconciliation
	BackupLoading BackupState = "Loading"
	// BackupReconciling - the backup is being restored via reconciliation as the config of the operator
	BackupReconciling BackupState = "Reconciling"
	// BackupRestored - the backup was restored as the config of the operator
	BackupRestored BackupState = "Restored"
	// BackupRestoreError - the backup restore failed for some reason
	BackupRestoreError BackupState = "Restore Error"
)

// GetOpenStackBackupOperationInProgress - If there is a backup or restore in progress, returns a string indicating the operation
func GetOpenStackBackupOperationInProgress(client goClient.Client, namespace string) (BackupState, error) {

	backups, err := GetOpenStackBackupRequestsWithLabel(client, namespace, map[string]string{})

	if err != nil {
		return "", err
	}

	for _, backup := range backups.Items {
		if backup.Status.CurrentState == BackupCleaning || backup.Status.CurrentState == BackupSaving || backup.Status.CurrentState == BackupQuiescing ||
			backup.Status.CurrentState == BackupReconciling || backup.Status.CurrentState == BackupLoading {
			return backup.Status.CurrentState, nil
		}
	}

	return "", nil
}

// GetOpenStackBackupRequestsWithLabel - Return a list of all OpenStackBackupRequestss in the namespace that have (optional) labels
func GetOpenStackBackupRequestsWithLabel(client goClient.Client, namespace string, labelSelector map[string]string) (*OpenStackBackupRequestList, error) {
	osBackupRequestList := &OpenStackBackupRequestList{}

	listOpts := []goClient.ListOption{
		goClient.InNamespace(namespace),
	}

	if len(labelSelector) > 0 {
		labels := goClient.MatchingLabels(labelSelector)
		listOpts = append(listOpts, labels)
	}

	if err := client.List(context.Background(), osBackupRequestList, listOpts...); err != nil {
		return nil, err
	}

	return osBackupRequestList, nil
}

func init() {
	SchemeBuilder.Register(&OpenStackBackupRequest{}, &OpenStackBackupRequestList{})
}
