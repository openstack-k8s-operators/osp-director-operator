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

package openstackipset

import (
	"context"
	"fmt"
	"strconv"
	"time"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

//
// CreateOrUpdateIPset - Create/Update IPSet
//
func CreateOrUpdateIPset(
	r common.ReconcilerCommon,
	object client.Object,
	cond *ospdirectorv1beta1.Condition,
	ipsetDetails common.IPSet,
) (*ospdirectorv1beta1.OpenStackIPSet, ctrl.Result, error) {

	ipSet := &ospdirectorv1beta1.OpenStackIPSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      object.GetName(),
			Namespace: object.GetNamespace(),
			Labels: map[string]string{
				AddToPredictableIPsLabel: strconv.FormatBool(ipsetDetails.AddToPredictableIPs),
			},
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.GetClient(), ipSet, func() error {
		ipSet.Spec.Networks = ipsetDetails.Networks
		ipSet.Spec.RoleName = ipsetDetails.Role
		ipSet.Spec.HostCount = ipsetDetails.HostCount
		ipSet.Spec.VIP = ipsetDetails.VIP
		ipSet.Spec.AddToPredictableIPs = ipsetDetails.AddToPredictableIPs
		ipSet.Spec.HostNameRefs = ipsetDetails.HostNameRefs

		err := controllerutil.SetControllerReference(object, ipSet, r.GetScheme())

		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update OpenStackIPSet %v ", ipsetDetails)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.VMSetCondReasonIPsetCreateOrUpdateError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, object, err)

		return ipSet, ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		cond.Message = fmt.Sprintf("OpenStackIPSet for %s successfully reconciled - operation: %s", object.GetName(), string(op))
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.VMSetCondReasonIPsetCreateOrUpdateError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeProvisioned)

		common.LogForObject(r, cond.Message, object)
	}

	if len(ipSet.Status.HostIPs) < ipsetDetails.HostCount {
		cond.Message = fmt.Sprintf("OpenStackIPSet has not yet reached the required count %d", ipsetDetails.HostCount)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.VMSetCondReasonIPsetWaitCount)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeWaiting)

		return ipSet, ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	return ipSet, ctrl.Result{}, nil
}
