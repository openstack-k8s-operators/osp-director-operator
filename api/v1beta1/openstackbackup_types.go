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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackBackupSpec defines the desired state of OpenStackBackup
type OpenStackBackupSpec struct {
	// +kubebuilder:validation:Optional
	// OSP-D-operator-specific resources saved in the backup
	Crs CrsForBackup `json:"crs,omitempty" optional:"true"`

	// Generic resources

	// +kubebuilder:validation:Optional
	// List of ConfigMaps saved in the backup
	ConfigMaps corev1.ConfigMapList `json:"configMaps,omitempty" optional:"true"`
	// +kubebuilder:validation:Optional
	// List of Secrets saved in the backup
	Secrets corev1.SecretList `json:"secrets,omitempty" optional:"true"`
}

// CrsForBackup -
type CrsForBackup struct {
	// OpenStackBaremetalSets - list of OpenStackBaremetalSets saved in the backup
	OpenStackBaremetalSets OpenStackBaremetalSetList `json:"openStackBaremetalSets,omitempty" optional:"true"`
	// OpenStackClients - list of OpenStackClients saved in the backup
	OpenStackClients OpenStackClientList `json:"openStackClients,omitempty" optional:"true"`
	// OpenStackControlPlanes - list of OpenStackControlPlanes saved in the backup
	OpenStackControlPlanes OpenStackControlPlaneList `json:"openStackControlPlanes,omitempty" optional:"true"`
	// OpenStackMACAddresses - list of OpenStackMACAddresses saved in the backup
	OpenStackMACAddresses OpenStackMACAddressList `json:"openStackMACAddresses,omitempty" optional:"true"`
	// OpenStackNets - list of OpenStackNets saved in the backup
	OpenStackNets OpenStackNetList `json:"openStackNets,omitempty" optional:"true"`
	// OpenStackNetAttachments - list of OpenStackNetAttachments saved in the backup
	OpenStackNetAttachments OpenStackNetAttachmentList `json:"openStackNetAttachments,omitempty" optional:"true"`
	// OpenStackNetConfigs - list of OpenStackNetConfigs saved in the backup
	OpenStackNetConfigs OpenStackNetConfigList `json:"openStackNetConfigs,omitempty" optional:"true"`
	// OpenStackProvisionServers - list of OpenStackProvisionServers saved in the backup
	OpenStackProvisionServers OpenStackProvisionServerList `json:"openStackProvisionServers,omitempty" optional:"true"`
	// OpenStackVMSets - list of OpenStackVMSets saved in the backup
	OpenStackVMSets OpenStackVMSetList `json:"openStackVMSets,omitempty" optional:"true"`
}

// OpenStackBackupStatus defines the observed state of OpenStackBackup
type OpenStackBackupStatus struct {
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osbackup;osbackups
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Backup"

// OpenStackBackup is used to backup OpenShift resources representing an OSP overcloud
type OpenStackBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackBackupSpec   `json:"spec,omitempty"`
	Status OpenStackBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenStackBackupList contains a list of OpenStackBackup
type OpenStackBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackBackup{}, &OpenStackBackupList{})
}
