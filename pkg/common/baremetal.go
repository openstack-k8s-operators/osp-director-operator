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
	"strings"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetBmhHosts -
func GetBmhHosts(r ReconcilerCommon, namespace string, labelSelector map[string]string) (*metal3v1alpha1.BareMetalHostList, error) {

	bmhHostsList := &metal3v1alpha1.BareMetalHostList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	if len(labelSelector) > 0 {
		labels := client.MatchingLabels(labelSelector)
		listOpts = append(listOpts, labels)
	}

	err := r.GetClient().List(context.TODO(), bmhHostsList, listOpts...)
	if err != nil {
		return bmhHostsList, err
	}

	return bmhHostsList, nil
}

// GetDeletionAnnotatedBmhHosts -
func GetDeletionAnnotatedBmhHosts(r ReconcilerCommon, namespace string, labelSelector map[string]string) ([]string, error) {

	annotatedBMHs := []string{}

	baremetalHostList, err := GetBmhHosts(r, namespace, labelSelector)
	if err != nil {
		return annotatedBMHs, err
	}

	// Find deletion annotated BaremetalHosts belonging to this OpenStackBaremetalSet
	for _, baremetalHost := range baremetalHostList.Items {

		// Get list of OSP hostnames from HostRemovalAnnotation annotated BMHs
		if val, ok := baremetalHost.Annotations[HostRemovalAnnotation]; ok && (strings.ToLower(val) == "yes" || strings.ToLower(val) == "true") {
			annotatedBMHs = append(annotatedBMHs, baremetalHost.GetName())
		}
	}

	return annotatedBMHs, nil
}
