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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteSericesWithLabel - Delete all services in namespace of the obj matching label selector
func DeleteSericesWithLabel(r ReconcilerCommon, obj metav1.Object, labelSelectorMap map[string]string) error {
	err := r.GetClient().DeleteAllOf(
		context.TODO(),
		&corev1.Service{},
		client.InNamespace(obj.GetNamespace()),
		client.MatchingLabels(labelSelectorMap),
	)
	if err != nil && !k8s_errors.IsNotFound(err) {
		err = fmt.Errorf("Error DeleteAllOf Service: %v", err)
		return err
	}

	return nil
}
