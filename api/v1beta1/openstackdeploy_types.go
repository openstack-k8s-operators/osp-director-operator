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

const (
	//
	// condition types
	//

	// DeployCondTypeWaiting - the deployment is waiting
	DeployCondTypeWaiting ProvisioningState = "Waiting"
	// DeployCondTypeInitializing - the deployment is waiting
	DeployCondTypeInitializing ProvisioningState = "Initializing"
	// DeployCondTypeRunning - the deployment is running
	DeployCondTypeRunning ProvisioningState = "Running"
	// DeployCondTypeFinished - the deploy has finished executing
	DeployCondTypeFinished ProvisioningState = "Finished"
	// DeployCondTypeError - the deployment hit a generic error
	DeployCondTypeError ProvisioningState = "Error"

	// DeployCondReasonJobCreated - job created
	DeployCondReasonJobCreated ConditionReason = "JobCreated"
	// DeployCondReasonJobCreateFailed - job create failed
	DeployCondReasonJobCreateFailed ConditionReason = "JobCreated"
	// DeployCondReasonJobDelete - job deleted
	DeployCondReasonJobDelete ConditionReason = "JobDeleted"
	// DeployCondReasonJobFinished - job deleted
	DeployCondReasonJobFinished ConditionReason = "JobFinished"
	// DeployCondReasonCVUpdated - error creating/update ConfigVersion
	DeployCondReasonCVUpdated ConditionReason = "ConfigVersionUpdated"
	// DeployCondReasonConfigVersionNotFound - error finding ConfigVersion
	DeployCondReasonConfigVersionNotFound ConditionReason = "ConfigVersionNotFound"
	// DeployCondReasonJobFailed - error creating/update CM
	DeployCondReasonJobFailed ConditionReason = "JobFailed"
	// DeployCondReasonConfigCreate - error creating/update CM
	DeployCondReasonConfigCreate ConditionReason = "ConfigCreate"
)

// OpenStackDeploySpec defines the desired state of OpenStackDeploy
type OpenStackDeploySpec struct {
	// Name of the image
	ImageURL string `json:"imageURL"`

	// ConfigVersion the config version/git hash of the playbooks to deploy.
	ConfigVersion string `json:"configVersion,omitempty"`

	// ConfigGenerator name of the configGenerator
	ConfigGenerator string `json:"configGenerator"`

	Playbook string   `json:"playbook,omitempty"`
	Limit    string   `json:"limit,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	SkipTags []string `json:"skipTags,omitempty"`
}

// OpenStackDeployStatus defines the observed state of OpenStackDeploy
type OpenStackDeployStatus struct {
	// ConfigVersion hash that has been deployed
	ConfigVersion string `json:"configVersion"`

	// CurrentState
	CurrentState ProvisioningState `json:"currentState"`

	// CurrentReason
	CurrentReason ConditionReason `json:"currentReason"`

	Conditions ConditionList `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=osdeploy;osdeploys;osdepl
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
