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
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteServicesWithLabel - Delete all services in namespace of the obj matching label selector
func DeleteServicesWithLabel(
	ctx context.Context,
	r ReconcilerCommon,
	obj metav1.Object,
	labelSelectorMap map[string]string,
) error {
	// Service have not implemented DeleteAllOf
	// https://github.com/operator-framework/operator-sdk/issues/3101
	// https://github.com/kubernetes/kubernetes/issues/68468#issuecomment-419981870
	// delete services
	serviceList := &corev1.ServiceList{}
	listOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
		client.MatchingLabels(labelSelectorMap),
	}

	if err := r.GetClient().List(ctx, serviceList, listOpts...); err != nil {
		err = fmt.Errorf("Error listing services for %s: %v", obj.GetName(), err)
		return err
	}

	// delete all pods
	for _, pod := range serviceList.Items {
		err := r.GetClient().Delete(ctx, &pod)
		if err != nil && !k8s_errors.IsNotFound(err) {
			err = fmt.Errorf("Error deleting service %s: %v", pod.Name, err)
			return err
		}
	}

	return nil
}

// GetServicesListWithLabel - Get all services in namespace of the obj matching label selector
func GetServicesListWithLabel(
	ctx context.Context,
	r ReconcilerCommon,
	namespace string,
	labelSelectorMap map[string]string,
) (*corev1.ServiceList, error) {

	labelSelectorString := labels.Set(labelSelectorMap).String()

	// use kclient to not use a cached client to be able to list services in namespace which are not cached
	// otherwise we hit "Error listing services for labels: map[ ... ] - unable to get: default because of unknown namespace for the cache"
	serviceList, err := r.GetKClient().CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelectorString})
	if err != nil {
		err = fmt.Errorf("Error listing services for labels: %v - %v", labelSelectorMap, err)
		return nil, err
	}

	return serviceList, nil
}
