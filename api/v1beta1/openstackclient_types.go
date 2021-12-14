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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackClientSpec defines the desired state of OpenStackClient
type OpenStackClientSpec struct {
	// Name of the image
	ImageURL string `json:"imageURL"`

	// +kubebuilder:validation:Optional
	// name of secret holding the stack-admin ssh keys
	DeploymentSSHSecret string `json:"deploymentSSHSecret"`

	// +kubebuilder:validation:Optional
	// GitSecret the name of the secret used to pull Ansible playbooks into the openstackclient pod. This secret should contain an entry for both 'git_url' and 'git_ssh_identity'
	GitSecret string `json:"gitSecret"`

	// +kubebuilder:validation:Optional
	// cloudname passed in via OS_CLOUDNAME
	CloudName string `json:"cloudName"`

	// +kubebuilder:default={ctlplane,external}
	// Networks the name(s) of the OpenStackNetworks used to generate IPs
	Networks []string `json:"networks"`

	// StorageClass to be used for the openstackclient persistent storage
	StorageClass string `json:"storageClass,omitempty"`

	// +kubebuilder:default=42401
	// RunUID user ID to run the pod with
	RunUID int `json:"runUID"`

	// +kubebuilder:default=42401
	// RunGID user ID to run the pod with
	RunGID int `json:"runGID"`

	// Domain name used to build fqdn
	DomainName string `json:"domainName,omitempty"`

	// Upstream DNS servers
	DNSServers       []string `json:"dnsServers,omitempty"`
	DNSSearchDomains []string `json:"dnsSearchDomains,omitempty"`

	// Idm secret used to register openstack client in IPA
	IdmSecret string `json:"idmSecret,omitempty"`
}

const (
	//
	// condition reasones
	//

	// OsClientCondReasonPVCError - error creating pvc
	OsClientCondReasonPVCError ConditionReason = "PVCError"
	// OsClientCondReasonPodError - error creating pod
	OsClientCondReasonPodError ConditionReason = "PodError"
	// OsClientCondReasonPodProvisioned - pod created
	OsClientCondReasonPodProvisioned ConditionReason = "OpenStackClientProvisioned"
	// OsClientCondReasonPodDeleteError - pod delete error
	OsClientCondReasonPodDeleteError ConditionReason = "PodDeleteError"
)

// OpenStackClientStatus defines the observed state of OpenStackClient
type OpenStackClientStatus struct {
	OpenStackClientNetStatus map[string]HostStatus `json:"netStatus,omitempty"`

	// Conditions - conditions to display in the OpenShift GUI, which reflect CurrentState
	Conditions ConditionList `json:"conditions,omitempty" optional:"true"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osclient;osclients
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Client"

// OpenStackClient used to create a container for deploying, scaling, and managing the OpenStack Overcloud
type OpenStackClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackClientSpec   `json:"spec,omitempty"`
	Status OpenStackClientStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenStackClientList contains a list of OpenStackClient
type OpenStackClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackClient `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackClient{}, &OpenStackClientList{})
}

// GetHostnames -
func (openstackclient OpenStackClient) GetHostnames() map[string]string {

	ret := make(map[string]string)
	for _, val := range openstackclient.Status.OpenStackClientNetStatus {
		ret[val.Hostname] = val.HostRef
	}
	return ret
}
