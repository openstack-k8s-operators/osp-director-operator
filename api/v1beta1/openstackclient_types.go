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

// OpenStackClientSpec defines the desired state of OpenStackClient
type OpenStackClientSpec struct {
	// Name of the image
	ImageURL string `json:"imageURL"`
	// name of secret holding the stack-admin ssh keys
	DeploymentSSHSecret string `json:"deploymentSSHSecret"`
	// cloudname passed in via OS_CLOUDNAME
	CloudName string `json:"cloudName"`
	// additional Hostaliases added to the openstackclient hosts file
	HostAliases []corev1.HostAlias `json:"hostAliases"`
}

// OpenStackClientStatus defines the observed state of OpenStackClient
type OpenStackClientStatus struct {
	// OS_AUTH_URL
	AuthURL string `json:"authURL"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OpenStackClient is the Schema for the openstackclients API
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
