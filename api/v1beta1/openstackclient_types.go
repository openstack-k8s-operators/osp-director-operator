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
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackClientSpec defines the desired state of OpenStackClient
type OpenStackClientSpec struct {
	// +kubebuilder:validation:Optional
	// OpenStackClient image. If missing will be set to the configured OPENSTACKCLIENT_IMAGE_URL_DEFAULT in the CSV for the OSP Director Operator.
	ImageURL string `json:"imageURL,omitempty"`

	// +kubebuilder:validation:Optional
	// name of secret holding the stack-admin ssh keys
	DeploymentSSHSecret string `json:"deploymentSSHSecret"`

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

	// Idm secret used to register openstack client in IPA
	IdmSecret string `json:"idmSecret,omitempty"`

	// Name of the config map containing custom CA certificates to trust
	CAConfigMap string `json:"caConfigMap,omitempty"`
}

// OpenStackClientStatus defines the observed state of OpenStackClient
type OpenStackClientStatus struct {
	OpenStackClientNetStatus map[string]HostStatus `json:"netStatus,omitempty"`

	// Conditions - conditions to display in the OpenShift GUI, which reflect CurrentState
	Conditions shared.ConditionList `json:"conditions,omitempty" optional:"true"`
}

// IsReady - Is this resource in its fully-configured (quiesced) state?
func (instance *OpenStackClient) IsReady() bool {
	return instance.Status.Conditions.InitCondition().Reason == shared.OsClientCondReasonPodProvisioned
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
func (instance OpenStackClient) GetHostnames() map[string]string {

	ret := make(map[string]string)
	for _, val := range instance.Status.OpenStackClientNetStatus {
		ret[val.Hostname] = val.HostRef
	}
	return ret
}
