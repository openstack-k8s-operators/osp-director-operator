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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	nmstateshared "github.com/nmstate/kubernetes-nmstate/api/shared"
	nmstatev1alpha1 "github.com/nmstate/kubernetes-nmstate/api/v1alpha1"
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	nmstate "github.com/openstack-k8s-operators/osp-director-operator/pkg/nmstate"
)

// log is for logging in this package.
var openstackdeploylog = logf.Log.WithName("openstackdeploy-resource")

// OpenStackDeployDefaults -
type OpenStackDeployDefaults struct {
	AgentImageURL string
}

var openstackDeployDefaults OpenStackDeployDefaults

// SetupWebhookWithManager -
func (r *OpenStackDeploy) SetupWebhookWithManager(mgr ctrl.Manager, defaults OpenStackDeployDefaults) error {
	openstackDeployDefaults = defaults

	if webhookClient == nil {
		webhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-osp-director-openstack-org-v1beta1-openstackdeploy,mutating=true,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackdeploys,verbs=create;update,versions=v1beta1,name=mopenstackdeploy.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OpenStackDeploy{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackDeploy) Default() {
	openstackdeploylog.Info("default", "name", r.Name)
	if r.Spec.ImageURL == "" {
		r.Spec.ImageURL = openstackDeployDefaults.AgentImageURL
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-osp-director-openstack-org-v1beta1-openstackdeploy,mutating=false,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackdeploys,verbs=create;update,versions=v1beta1,name=vopenstackdeploy.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpenStackDeploy{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackDeploy) ValidateCreate() error {
	openstackdeploylog.Info("validate create", "name", r.Name)

	//
	// Validates all NNCPs created by the used osnetcfg to be in condition.Reason == nmstateshared.NodeNetworkConfigurationPolicyConditionSuccessfullyConfigured.
	// If not, stop deployment. User can overwrite this via parameter spec.SkipNNCPValidation: true
	//
	if err := r.validateNNCP(); err != nil {
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackDeploy) ValidateUpdate(old runtime.Object) error {
	openstackdeploylog.Info("validate update", "name", r.Name)

	//
	// Validates all NNCPs created by the used osnetcfg to be in condition.Reason == nmstateshared.NodeNetworkConfigurationPolicyConditionSuccessfullyConfigured.
	// If not, stop deployment. User can overwrite this via parameter spec.SkipNNCPValidation: true
	//
	if err := r.validateNNCP(); err != nil {
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackDeploy) ValidateDelete() error {
	openstackdeploylog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

//
// Validates all NNCPs created by the used osnetcfg to be in condition.Reason == nmstateshared.NodeNetworkConfigurationPolicyConditionSuccessfullyConfigured.
// If not, stop deployment. User can overwrite this via parameter spec.SkipNNCPValidation: true
//
func (r *OpenStackDeploy) validateNNCP() error {

	// 1) get the osctlplane of the namespace (right now there can only be one)
	osctlplane, _, err := GetControlPlane(webhookClient, r)
	if err != nil {
		return err
	}

	osnetcfgName := osctlplane.GetLabels()[shared.OpenStackNetConfigReconcileLabel]

	// 2) get all nncp with osnetcfg ref label ospdirectorv1beta1.OpenStackNetConfigReconcileLabel
	nncpList := &nmstatev1alpha1.NodeNetworkConfigurationPolicyList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(
			map[string]string{
				shared.OpenStackNetConfigReconcileLabel: osnetcfgName,
			},
		),
	}

	if err := webhookClient.List(context.TODO(), nncpList, listOpts...); err != nil {
		return err
	}

	if len(nncpList.Items) == 0 {
		return fmt.Errorf("no NodeNetworkConfigurationPolicy found for reference label %s %s",
			shared.OpenStackNetConfigReconcileLabel,
			osnetcfgName)
	}

	nncpReadyCount := 0
	for _, nncp := range nncpList.Items {
		if nncp.Status.Conditions != nil && len(nncp.Status.Conditions) > 0 {
			condition := nmstate.GetCurrentCondition(nncp.Status.Conditions)
			if condition != nil {
				if condition.Type == nmstateshared.NodeNetworkConfigurationPolicyConditionAvailable &&
					condition.Reason == nmstateshared.NodeNetworkConfigurationPolicyConditionSuccessfullyConfigured {

					openstackdeploylog.Info(fmt.Sprintf("%s %s: %s", nncp.Kind, nncp.Name, condition.Message))
					nncpReadyCount++
				} else if condition.Type == nmstateshared.NodeNetworkConfigurationPolicyConditionDegraded {

					// get NNCE list to receive more details on why nncp is Degraded
					nnceList := &nmstatev1alpha1.NodeNetworkConfigurationEnactmentList{}
					listOpts := []client.ListOption{
						client.MatchingLabels(
							map[string]string{
								nmstateshared.EnactmentPolicyLabel: nncp.Name,
							},
						),
					}

					if err := webhookClient.List(context.TODO(), nnceList, listOpts...); err != nil {
						return err
					}

					var nnceError string
					var nnce nmstatev1alpha1.NodeNetworkConfigurationEnactment
					for _, nnce = range nnceList.Items {
						if nnce.Status.Conditions != nil && len(nnce.Status.Conditions) > 0 {
							condition := nmstate.GetCurrentCondition(nnce.Status.Conditions)

							// if the first nnce has condition.Reason == nmstateshared.NodeNetworkConfigurationEnactmentConditionFailedToConfigure
							// return the message as a hint to look at, do not return all nnce messages
							if condition != nil && condition.Reason == nmstateshared.NodeNetworkConfigurationEnactmentConditionFailedToConfigure {
								nnceError = condition.Message
								break
							}
						}
					}

					msg := fmt.Sprintf("validation failed: %s %s not in %s state: %s %s - NodeNetworkConfigurationEnactments %s message: %s",
						nncp.GroupVersionKind().Kind,
						nncp.GetName(),
						nmstateshared.NodeNetworkConfigurationPolicyConditionSuccessfullyConfigured,
						condition.Reason,
						condition.Message,
						nnce.Name,
						nnceError,
					)

					// if user skip validation, just log the failed validation
					if r.Spec.SkipNNCPValidation {
						openstackdeploylog.Info(fmt.Sprintf("SkipNNCPValidation configured! %s", msg))
					} else {
						info := "It's recommended to resolve the failures before proceeding with the deployment. If this validation should be skipped add spec.SkipNNCPValidation: true"
						openstackdeploylog.Error(
							fmt.Errorf(msg),
							info,
							"name",
							r.Name)
						return fmt.Errorf(fmt.Sprintf("%s, %s", msg, info))
					}
				}
			}
		}
	}

	return nil
}
