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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// GetLabels - get labelselector for CR
func GetLabels(obj metav1.Object, controller string, custom map[string]string) map[string]string {
	// Labels for all objects
	labelSelector := map[string]string{
		OwnerUIDLabelSelector:            string(obj.GetUID()),
		OwnerNameSpaceLabelSelector:      obj.GetNamespace(),
		OwnerNameLabelSelector:           obj.GetName(),
		OwnerControllerNameLabelSelector: controller,
	}
	for k, v := range custom {
		labelSelector[k] = v
	}

	return labelSelector
}
