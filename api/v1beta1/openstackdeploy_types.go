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

// DeployState - the state of the deployment
type DeployState string

// DeployReason - the reason of the condition for this deployment
type DeployReason string

const (
	//
	// condition types
	//

	// DeployCondTypeWaiting - the deployment is waiting
	DeployCondTypeWaiting DeployState = "Waiting"
	// DeployCondTypeInitializing - the deployment is waiting
	DeployCondTypeInitializing DeployState = "Initializing"
	// DeployCondTypeRunning - the deployment is running
	DeployCondTypeRunning DeployState = "Running"
	// DeployCondTypeFinished - the deploy has finished executing
	DeployCondTypeFinished DeployState = "Finished"
	// DeployCondTypeError - the deployment hit a generic error
	DeployCondTypeError DeployState = "Error"

	// DeployCondReasonJobCreated - configmap updated
	DeployCondReasonJobCreated DeployReason = "JobCreated"
	// DeployCondReasonJobDelete - configmap not found
	DeployCondReasonJobDelete DeployReason = "JobDeleted"
	// DeployCondReasonCMUpdated - error creating/update CM
	DeployCondReasonCMUpdated DeployReason = "ConfigVersionUpdated"
	// DeployCondReasonJobFailed - error creating/update CM
	DeployCondReasonJobFailed DeployReason = "JobFailed"
	// DeployCondReasonConfigCreate - error creating/update CM
	DeployCondReasonConfigCreate DeployReason = "ConfigCreate"
)

// OpenStackDeploySpec defines the desired state of OpenStackDeploy
type OpenStackDeploySpec struct {
	// Name of the image
	ImageURL string `json:"imageURL"`

	// ConfigVersion the config version/git hash of the playbooks to deploy.
	ConfigVersion string `json:"configVersion,omitempty"`

	// ConfigGenerator name of the configGenerator
	ConfigGenerator string `json:"configGenerator"`
}

// OpenStackDeployStatus defines the observed state of OpenStackDeploy
type OpenStackDeployStatus struct {
	// ConfigVersion hash that has been deployed
	ConfigVersion string `json:"configVersion"`

	// CurrentState
	CurrentState DeployState `json:"currentState"`

	// CurrentReason
	CurrentReason DeployReason `json:"currentReason"`

	Conditions ConditionList `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=osdeploy;osdeploys
//+operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Deploy"
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.currentState`,description="Status"

// OpenStackDeploy is the Schema for the openstackdeploys API
type OpenStackDeploy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackDeploySpec   `json:"spec,omitempty"`
	Status OpenStackDeployStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackDeployList contains a list of OpenStackDeploy
type OpenStackDeployList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackDeploy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackDeploy{}, &OpenStackDeployList{})
}
