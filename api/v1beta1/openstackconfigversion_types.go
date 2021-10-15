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

// OpenStackConfigVersionSpec defines the desired state of OpenStackConfigVersion
type OpenStackConfigVersionSpec struct {
	Hash string `json:"hash"`
	Diff string `json:"diff"`
}

// OpenStackConfigVersionStatus defines the observed state of OpenStackConfigVersion
type OpenStackConfigVersionStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osconfigversion;osconfigversions
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Config Version"

// OpenStackConfigVersion is the Schema for the openstackconfigversions API
type OpenStackConfigVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackConfigVersionSpec   `json:"spec,omitempty"`
	Status OpenStackConfigVersionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackConfigVersionList contains a list of OpenStackConfigVersion
type OpenStackConfigVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackConfigVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackConfigVersion{}, &OpenStackConfigVersionList{})
}
