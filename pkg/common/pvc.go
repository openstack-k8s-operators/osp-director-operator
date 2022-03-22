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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Pvc - pvc details
type Pvc struct {
	Name         string
	Namespace    string
	Size         string
	Labels       map[string]string
	StorageClass string
	AccessMode   []corev1.PersistentVolumeAccessMode
}

// CreateOrUpdatePvc -
func CreateOrUpdatePvc(ctx context.Context, r ReconcilerCommon, obj metav1.Object, pv *Pvc) (*corev1.PersistentVolumeClaim, controllerutil.OperationResult, error) {

	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Name = pv.Name
	pvc.Namespace = pv.Namespace

	op, err := controllerutil.CreateOrPatch(ctx, r.GetClient(), pvc, func() error {

		pvc.Labels = pv.Labels

		pvc.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse(pv.Size),
		}

		pvc.Spec.StorageClassName = &pv.StorageClass
		pvc.Spec.AccessModes = pv.AccessMode

		err := controllerutil.SetOwnerReference(obj, pvc, r.GetScheme())
		if err != nil {
			return fmt.Errorf("error set controller reverence for PVC: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, op, fmt.Errorf("error create/updating pvc: %v", err)
	}

	return pvc, op, nil
}
