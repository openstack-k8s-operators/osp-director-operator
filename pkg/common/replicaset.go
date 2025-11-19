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

package common //revive:disable:var-naming

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteReplicasetsWithLabel - Delete all replicasets in namespace of the obj matching label selector
func DeleteReplicasetsWithLabel(
	ctx context.Context,
	r ReconcilerCommon,
	obj metav1.Object,
	labelSelectorMap map[string]string,
) error {
	err := r.GetClient().DeleteAllOf(
		ctx,
		&appsv1.ReplicaSet{},
		client.InNamespace(obj.GetNamespace()),
		client.MatchingLabels(labelSelectorMap),
	)
	if err != nil && !k8s_errors.IsNotFound(err) {
		err = fmt.Errorf("Error DeleteAllOf ReplicaSet: %w", err)
		return err
	}

	return nil
}
