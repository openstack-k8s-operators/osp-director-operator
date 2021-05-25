/*
Copyright 2021 Red Hat

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

package vmset

import (
	"context"

	"github.com/openstack-k8s-operators/osp-director-operator/pkg/common"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// ProcessInfoForProvisioningStatus - update OpenStackVMSet with non-error provisioning status
func ProcessInfoForProvisioningStatus(r common.ReconcilerCommon, instance *ospdirectorv1beta1.OpenStackVMSet, msg string, state ospdirectorv1beta1.VMSetProvisioningState) error {
	instance.Status.ProvisioningStatus.State = state
	instance.Status.ProvisioningStatus.Reason = msg

	// TODO: For now we use msg for both reason and message, but perhaps we'll want to change this later
	setCondition(instance, state, msg, msg)

	if msg != "" {
		r.GetLogger().Info(msg)
	}

	return setStatus(r, instance)
}

// ProcessErrorForProvisioningStatus - update OpenStackVMSet with provisioning status reporting an error
func ProcessErrorForProvisioningStatus(r common.ReconcilerCommon, instance *ospdirectorv1beta1.OpenStackVMSet, err error) {
	msg := err.Error()
	instance.Status.ProvisioningStatus.State = ospdirectorv1beta1.VMSetError
	instance.Status.ProvisioningStatus.Reason = msg

	// TODO: For now we use msg for both reason and message, but perhaps we'll want to change this later
	setCondition(instance, ospdirectorv1beta1.VMSetError, msg, msg)

	r.GetLogger().Info(msg)
	// The next line could return an error, but we log it in the "setStatus" func,
	// and we're more interested in the prior error anyhow
	_ = setStatus(r, instance)
}

// setCondition - Sets an overall condition for the OpenStackVMSet to surface in the web console
func setCondition(instance *ospdirectorv1beta1.OpenStackVMSet, state ospdirectorv1beta1.VMSetProvisioningState, reason string, msg string) {
	instance.Status.Conditions = ospdirectorv1beta1.ConditionList{}
	instance.Status.Conditions.Set(ospdirectorv1beta1.ConditionType(state), corev1.ConditionTrue, ospdirectorv1beta1.ConditionReason(reason), msg)
}

func setStatus(r common.ReconcilerCommon, instance *ospdirectorv1beta1.OpenStackVMSet) error {
	if err := r.GetClient().Status().Update(context.TODO(), instance); err != nil {
		r.GetLogger().Error(err, "Failed to update OpenStackVMSet CR status %v")
		return err
	}

	return nil
}
