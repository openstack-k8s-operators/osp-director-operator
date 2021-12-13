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

// Generated by:
//
// operator-sdk create webhook --group osp-director --version v1beta1 --kind OpenStackBackupRequest --programmatic-validation
//

package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var openstackbackuprequestlog = logf.Log.WithName("openstackbackuprequest-resource")

// SetupWebhookWithManager -
func (r *OpenStackBackupRequest) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if webhookClient == nil {
		webhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-osp-director-openstack-org-v1beta1-openstackbackuprequest,mutating=false,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackbackuprequests,verbs=create;update,versions=v1beta1,name=vopenstackbackuprequest.kb.io,admissionReviewVersions={v1},sideEffects=None

var _ webhook.Validator = &OpenStackBackupRequest{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackBackupRequest) ValidateCreate() error {
	openstackbackuprequestlog.Info("validate create", "name", r.Name)

	if err := r.validateCr(); err != nil {
		return err
	}

	currentBackupOperation, err := GetOpenStackBackupOperationInProgress(webhookClient, r.Namespace)

	if err != nil {
		return err
	}

	if currentBackupOperation != "" {
		return fmt.Errorf("cannot create a new backup request while an existing backup request is %s", currentBackupOperation)
	}

	return r.validateCr()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackBackupRequest) ValidateUpdate(old runtime.Object) error {
	openstackbackuprequestlog.Info("validate update", "name", r.Name)

	return r.validateCr()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackBackupRequest) ValidateDelete() error {
	openstackbackuprequestlog.Info("validate delete", "name", r.Name)

	return nil
}

func (r *OpenStackBackupRequest) validateCr() error {
	if r.Spec.Mode == BackupRestore {
		return webhookClient.Get(context.TODO(), types.NamespacedName{Name: r.Spec.RestoreSource, Namespace: r.Namespace}, &OpenStackBackup{})
	}

	return nil
}