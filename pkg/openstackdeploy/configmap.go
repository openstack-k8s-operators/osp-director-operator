/*
Copyright 2022 Red Hat

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

package openstackdeploy

import (
	"context"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// SetConfigMapOwner -
func SetConfigMapOwner(r common.ReconcilerCommon, cr *ospdirectorv1beta1.OpenStackDeploy) error {

	// set ownership on the ConfigMap created by our Deployment container
	configMap := &corev1.ConfigMap{}
	err := r.GetClient().Get(context.TODO(), types.NamespacedName{Name: ConfigMapBasename + cr.Name, Namespace: cr.Namespace}, configMap)
	if err != nil {
		return err
	}
	ownerErr := controllerutil.SetOwnerReference(cr, configMap, r.GetScheme())
	if ownerErr != nil {
		return ownerErr
	}
	err = r.GetClient().Update(context.TODO(), configMap)
	if err != nil {
		return err
	}
	return nil

}
