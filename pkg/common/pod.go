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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetAllPodsWithLabel - get all pods from namespace with a specific label
func GetAllPodsWithLabel(r ReconcilerCommon, labelSelectorMap map[string]string, namespace string) (*corev1.PodList, error) {
	labelSelectorString := labels.Set(labelSelectorMap).String()

	podList, err := r.GetKClient().CoreV1().Pods(namespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: labelSelectorString,
		},
	)
	if err != nil {
		return podList, err
	}

	return podList, nil
}
