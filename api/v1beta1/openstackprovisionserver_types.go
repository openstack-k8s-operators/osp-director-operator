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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	goClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// OpenStackProvisionServerSpec defines the desired state of OpenStackProvisionServer
type OpenStackProvisionServerSpec struct {
	// The port on which the Apache server should listen
	Port int `json:"port"`
	// An optional interface to use instead of the cluster's default provisioning interface (if any)
	Interface string `json:"interface,omitempty"`
	// URL for RHEL qcow2 image (compressed as gz, or uncompressed)
	BaseImageURL string `json:"baseImageUrl"`
	// Container image URL for init container that downloads the RHEL qcow2 image (baseImageUrl)
	// +kubebuilder:default="quay.io/openstack-k8s-operators/rhel-downloader:0.0.1"
	DownloaderImageURL string `json:"downloaderImageUrl,omitempty"`
	// Container image URL for the main container that serves the downloaded RHEL qcow2 image (baseImageUrl)
	// +kubebuilder:default="quay.io/openstack-k8s-operators/httpd-24-centos7:2.4"
	ApacheImageURL string `json:"apacheImageUrl,omitempty"`
	// Container image URL for the sidecar container that discovers provisioning network IPs
	// +kubebuilder:default="quay.io/openstack-k8s-operators/provision-ip-discovery-agent:0.0.1"
	ProvisioningAgentImageURL string `json:"provisioningAgentImageUrl,omitempty"`
}

// OpenStackProvisionServerStatus defines the observed state of OpenStackProvisionServer
type OpenStackProvisionServerStatus struct {
	// Surfaces status in GUI
	Conditions ConditionList `json:"conditions,omitempty" optional:"true"`
	// Holds provisioning status for this provision server
	ProvisioningStatus OpenStackProvisionServerProvisioningStatus `json:"provisioningStatus,omitempty"`
	// IP of the provisioning interface on the node running the ProvisionServer pod
	ProvisionIP string `json:"provisionIp,omitempty"`
	// URL of provisioning image on underlying Apache web server
	LocalImageURL string `json:"localImageUrl,omitempty"`
}

// OpenStackProvisionServerProvisioningStatus represents the overall provisioning state of all BaremetalHosts in
// the OpenStackProvisionServer (with an optional explanatory message)
type OpenStackProvisionServerProvisioningStatus struct {
	State  ProvisionServerProvisioningState `json:"state,omitempty"`
	Reason string                           `json:"reason,omitempty"`
}

// ProvisionServerProvisioningState - the overall state of this OpenStackProvisionServer
type ProvisionServerProvisioningState string

const (
	// ProvisionServerWaiting - something else is causing the OpenStackProvisionServer to wait
	ProvisionServerWaiting ProvisionServerProvisioningState = "Waiting"
	// ProvisionServerProvisioning - the provision server pod is provisioning
	ProvisionServerProvisioning ProvisionServerProvisioningState = "Provisioning"
	// ProvisionServerProvisioned - the provision server pod is ready
	ProvisionServerProvisioned ProvisionServerProvisioningState = "Provisioned"
	// ProvisionServerError - general catch-all for actual errors
	ProvisionServerError ProvisionServerProvisioningState = "Error"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osprovserver;osprovservers
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack ProvisionServer"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.provisioningStatus.state",description="Status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.provisioningStatus.reason",description="Reason"

// OpenStackProvisionServer is the Schema for the openstackprovisionservers API
type OpenStackProvisionServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackProvisionServerSpec   `json:"spec,omitempty"`
	Status OpenStackProvisionServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenStackProvisionServerList contains a list of OpenStackProvisionServer
type OpenStackProvisionServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackProvisionServer `json:"items"`
}

// GetExistingProvServerPorts - Get all ports currently in use by all OpenStackProvisionServers in this namespace
func (r *OpenStackProvisionServer) GetExistingProvServerPorts(client goClient.Client) (map[string]int, error) {
	found := map[string]int{}

	provServerList := &OpenStackProvisionServerList{}

	listOpts := []goClient.ListOption{
		goClient.InNamespace(r.Namespace),
	}

	err := client.List(context.TODO(), provServerList, listOpts...)

	if err != nil {
		return nil, err
	}

	for _, provServer := range provServerList.Items {
		found[provServer.Name] = provServer.Spec.Port
	}

	return found, nil
}

// AssignProvisionServerPort - Assigns an Apache listening port for a particular OpenStackProvisionServer.
func (r *OpenStackProvisionServer) AssignProvisionServerPort(client goClient.Client, portStart int) error {
	if r.Spec.Port != 0 {
		// Do nothing, already assigned
		return nil
	}

	existingPorts, err := r.GetExistingProvServerPorts(client)

	if err != nil {
		return err
	}

	// It's possible that this prov server already exists and we are just dealing with
	// a minimized version of it (only its ObjectMeta is set, etc)
	r.Spec.Port = existingPorts[r.GetName()]

	// If we get this far, no port has been previously assigned, so we pick one
	if r.Spec.Port == 0 {
		cur := portStart

		for {
			found := false

			for _, port := range existingPorts {
				if port == cur {
					found = true
					break
				}
			}

			if !found {
				break
			}

			cur++
		}

		r.Spec.Port = cur
	}

	return nil
}

func init() {
	SchemeBuilder.Register(&OpenStackProvisionServer{}, &OpenStackProvisionServerList{})
}
