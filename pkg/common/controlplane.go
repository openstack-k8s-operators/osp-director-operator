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
	"context"
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// GetControlPlane -
func GetControlPlane(
	ctx context.Context,
	r ReconcilerCommon,
	obj metav1.Object,
) (ospdirectorv1beta1.OpenStackControlPlane, reconcile.Result, error) {

	controlPlane := ospdirectorv1beta1.OpenStackControlPlane{}

	// Get OSP ControlPlane CR where e.g. the status information has the OSP version: controlPlane.Status.OSPVersion
	// FIXME: We assume there is only one ControlPlane CR for now (enforced by webhook), but this might need to change
	controlPlaneList := &ospdirectorv1beta1.OpenStackControlPlaneList{}
	controlPlaneListOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
		client.Limit(1000),
	}
	err := r.GetClient().List(ctx, controlPlaneList, controlPlaneListOpts...)
	if err != nil {
		return controlPlane, ctrl.Result{}, err
	}

	if len(controlPlaneList.Items) == 0 {
		err := fmt.Errorf("no OpenStackControlPlanes found in namespace %s. Requeing", obj.GetNamespace())
		return controlPlane, ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	// FIXME: See FIXME above
	controlPlane = controlPlaneList.Items[0]

	return controlPlane, ctrl.Result{}, nil

}
