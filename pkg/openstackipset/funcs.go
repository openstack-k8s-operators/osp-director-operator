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
	"strconv"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// OvercloudipsetCreateOrUpdate -
func OvercloudipsetCreateOrUpdate(r common.ReconcilerCommon, obj metav1.Object, ipset common.IPSet) (*ospdirectorv1beta1.OpenStackIPSet, controllerutil.OperationResult, error) {
	overcloudIPSet := &ospdirectorv1beta1.OpenStackIPSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
			Labels: map[string]string{
				AddToPredictableIPsLabel: strconv.FormatBool(ipset.AddToPredictableIPs),
			},
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.GetClient(), overcloudIPSet, func() error {
		overcloudIPSet.Spec.Networks = ipset.Networks
		overcloudIPSet.Spec.RoleName = ipset.Role
		overcloudIPSet.Spec.HostCount = ipset.HostCount
		overcloudIPSet.Spec.VIP = ipset.VIP
		overcloudIPSet.Spec.AddToPredictableIPs = ipset.AddToPredictableIPs
		overcloudIPSet.Spec.HostNameRefs = ipset.HostNameRefs

		err := controllerutil.SetControllerReference(obj, overcloudIPSet, r.GetScheme())

		if err != nil {
			return err
		}

		return nil
	})

	return overcloudIPSet, op, err
}
