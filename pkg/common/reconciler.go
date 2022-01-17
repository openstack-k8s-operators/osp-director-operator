/*
Copyright 2020 Red Hat

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

package common

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconcilerCommon - common reconciler interface
type ReconcilerCommon interface {
	GetClient() client.Client
	GetKClient() kubernetes.Interface
	GetLogger() logr.Logger
	GetScheme() *runtime.Scheme
}

// InstanceCommon - common OSP-D resource instance interface
type InstanceCommon interface {
	// Place anything we want from "inherited" (metav1 types, etc) funcs here
	GetName() string
	GetNamespace() string

	// Place our types' custom funcs here
	IsReady() bool
}

// OpenStackBackupOverridesReconcile - Should a controller pause reconciliation for a particular resource given potential backup operations?
func OpenStackBackupOverridesReconcile(client client.Client, instance InstanceCommon) (bool, error) {
	var backupRequests *ospdirectorv1beta1.OpenStackBackupRequestList

	backupRequests, err := ospdirectorv1beta1.GetOpenStackBackupRequestsWithLabel(client, instance.GetNamespace(), map[string]string{})

	if err != nil {
		return true, err
	}

	for _, backup := range backupRequests.Items {
		// If this backup is quiescing...
		// - If this CR has reached its "finished" state, end this reconcile
		// If this backup is saving or loading...
		// - End this reconcile
		if backup.Status.CurrentState == ospdirectorv1beta1.BackupSaving ||
			backup.Status.CurrentState == ospdirectorv1beta1.BackupLoading ||
			(backup.Status.CurrentState == ospdirectorv1beta1.BackupQuiescing && instance.IsReady()) {
			return true, nil
		}
	}

	return false, nil
}
