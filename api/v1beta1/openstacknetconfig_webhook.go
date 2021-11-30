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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var openstacknetconfiglog = logf.Log.WithName("openstacknetconfig-resource")

// SetupWebhookWithManager - register this webhook with the controller manager
func (r *OpenStackNetConfig) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-osp-director-openstack-org-v1beta1-openstacknetconfig,mutating=true,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstacknetconfigs,verbs=create;update,versions=v1beta1,name=mopenstacknetconfig.kb.io,admissionReviewVersions={v1,v1beta1}
//+kubebuilder:webhook:path=/validate-osp-director-openstack-org-v1beta1-openstackprovisionserver,mutating=false,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackprovisionservers,verbs=create;update,versions=v1beta1,name=vopenstackprovisionserver.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &OpenStackNetConfig{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackNetConfig) Default() {
	openstacknetconfiglog.Info("default", "name", r.Name)

	for netIdx := range r.Spec.Networks {
		//
		// Auto flag IsControlPlane if Name and NameLower matches ControlPlane network names
		//
		if r.Spec.Networks[netIdx].Name == ControlPlaneName && r.Spec.Networks[netIdx].NameLower == ControlPlaneNameLower {
			r.Spec.Networks[netIdx].IsControlPlane = true
		}

		//
		// The OpenStackNetConfig "XYZ" is invalid: spec.routes: Invalid value: "null": spec.routes in body must be of type array: "null"
		//
		for subnetIdx := range r.Spec.Networks[netIdx].Subnets {
			if r.Spec.Networks[netIdx].Subnets[subnetIdx].IPv4.Routes == nil {
				r.Spec.Networks[netIdx].Subnets[subnetIdx].IPv4.Routes = []Route{}
			}
			if r.Spec.Networks[netIdx].Subnets[subnetIdx].IPv6.Routes == nil {
				r.Spec.Networks[netIdx].Subnets[subnetIdx].IPv6.Routes = []Route{}
			}
		}
	}

	//
	// The default domain name if non specified
	//
	if r.Spec.DomainName == "" {
		r.Spec.DomainName = DefaultDomainName
	}

	//
	//  spec.dnsServers in body must be of type array
	//
	if r.Spec.DNSServers == nil {
		r.Spec.DNSServers = []string{}
	}

	//
	//  spec.dnsSearchDomains in body must be of type array
	//
	if r.Spec.DNSSearchDomains == nil {
		r.Spec.DNSSearchDomains = []string{}
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-osp-director-openstack-org-v1beta1-openstacknetconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstacknetconfigs,verbs=create;update,versions=v1beta1,name=vopenstacknetconfig.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &OpenStackNetConfig{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackNetConfig) ValidateCreate() error {
	openstacknetconfiglog.Info("validate create", "name", r.Name)

	//
	// Verify that the specified control plane network name and name_lower match the expected ooo names
	//
	err := r.validateControlPlaneNetworkNames()
	if err != nil {
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackNetConfig) ValidateUpdate(old runtime.Object) error {
	openstacknetconfiglog.Info("validate update", "name", r.Name)

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackNetConfig) ValidateDelete() error {
	openstacknetconfiglog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

// validateControlPlaneNetworkNames - validate that the specified control plane network name and name_lower match the expected ooo names
func (r *OpenStackNetConfig) validateControlPlaneNetworkNames() error {
	// Verify that the specified control plane network name and name_lower match the expected ooo names
	for _, net := range r.Spec.Networks {
		if net.IsControlPlane {
			if net.Name != ControlPlaneName {
				return fmt.Errorf(fmt.Sprintf("control plane network name %s does not match %s", net.Name, ControlPlaneName))
			}
			if net.NameLower != ControlPlaneNameLower {
				return fmt.Errorf(fmt.Sprintf("control plane network name_lower  %s does not match %s", net.NameLower, ControlPlaneNameLower))
			}
		}
	}

	return nil
}
