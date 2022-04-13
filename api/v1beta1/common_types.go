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
	goClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
)

// Hash - struct to add hashes to status
type Hash struct {
	// Name of hash referencing the parameter
	Name string `json:"name,omitempty"`
	// Hash
	Hash string `json:"hash,omitempty"`
}

const (
	// HostRefInitState - intial HostRef state of a new node which has not yet assigned
	HostRefInitState string = "unassigned"
)

// HostStatus represents the hostname and IP info for a specific host
type HostStatus struct {
	Hostname          string                   `json:"hostname"`
	ProvisioningState shared.ProvisioningState `json:"provisioningState"`

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
