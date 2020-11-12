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

// BaremetalSetSpec defines the desired state of BaremetalSet
type BaremetalSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Replicas The number of baremetalhosts to attempt to aquire
	Replicas int `json:"replicas,omitempty"`
}

// BaremetalSetStatus defines the observed state of BaremetalSet
type BaremetalSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// BaremetalSet is the Schema for the baremetalsets API
type BaremetalSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BaremetalSetSpec   `json:"spec,omitempty"`
	Status BaremetalSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BaremetalSetList contains a list of BaremetalSet
type BaremetalSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BaremetalSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BaremetalSet{}, &BaremetalSetList{})
}
