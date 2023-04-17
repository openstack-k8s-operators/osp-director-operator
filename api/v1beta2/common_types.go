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

package v1beta2

import (
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
)

// Hash - struct to add hashes to status
type Hash struct {
	// Name of hash referencing the parameter
	Name string `json:"name,omitempty"`
	// Hash
	Hash string `json:"hash,omitempty"`
}

// IPStatus represents the hostname and IP info for a specific host
type IPStatus struct {
	Hostname string `json:"hostname"`

	// +kubebuilder:default=unassigned
	HostRef string `json:"hostRef"`

	// +kubebuilder:validation:Optional
	IPAddresses map[string]string `json:"ipaddresses"`
}

// HostStatus represents the hostname and IP info for a specific host
type HostStatus struct {

	// +kubebuilder:validation:Required
	// IPStatus -
	IPStatus `json:",inline"`

	ProvisioningState shared.ProvisioningState `json:"provisioningState"`

	// +kubebuilder:default=false
	// Host annotated for deletion
	AnnotatedForDeletion bool `json:"annotatedForDeletion"`

	UserDataSecretName    string `json:"userDataSecretName"`
	NetworkDataSecretName string `json:"networkDataSecretName"`
}

// SyncIPsetStatus - sync relevant information from IPSet to CR status
func SyncIPsetStatus(
	cond *shared.Condition,
	instanceStatus map[string]HostStatus,
	ipsetHostStatus ospdirectorv1beta1.IPStatus,
) HostStatus {
	var hostStatus HostStatus
	if _, ok := instanceStatus[ipsetHostStatus.Hostname]; !ok {
		hostStatus.Hostname = ipsetHostStatus.Hostname
		hostStatus.HostRef = ipsetHostStatus.HostRef
		hostStatus.IPAddresses = ipsetHostStatus.IPAddresses
	} else {
		// Note:
		// do not sync all information as other controllers are
		// the master for e.g.
		// - BMH <-> hostname mapping
		// - create of networkDataSecretName and userDataSecretName
		hostStatus = instanceStatus[ipsetHostStatus.Hostname]
		hostStatus.IPAddresses = ipsetHostStatus.IPAddresses

		if ipsetHostStatus.HostRef != shared.HostRefInitState {
			hostStatus.HostRef = ipsetHostStatus.HostRef
		}
	}

	hostStatus.ProvisioningState = shared.ProvisioningState(cond.Type)

	return hostStatus
}
