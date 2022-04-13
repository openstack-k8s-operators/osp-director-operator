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

package v1beta1

import (
	"context"

	goClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
)

// GetExistingProvServerPorts - Get all ports currently in use by all OpenStackProvisionServers in this namespace
func (instance *OpenStackProvisionServer) GetExistingProvServerPorts(
	cond *shared.Condition,
	client goClient.Client,
) (map[string]int, error) {
	found := map[string]int{}

	provServerList := &OpenStackProvisionServerList{}

	listOpts := []goClient.ListOption{
		goClient.InNamespace(instance.Namespace),
	}

	err := client.List(context.TODO(), provServerList, listOpts...)
	if err != nil {
		cond.Message = "Failed to get list of all OpenStackProvisionServer(s)"
		cond.Reason = shared.OpenStackProvisionServerCondReasonListError
		cond.Type = shared.ProvisionServerCondTypeError

		return nil, err
	}

	for _, provServer := range provServerList.Items {
		found[provServer.Name] = provServer.Spec.Port
	}

	return found, nil
}

// AssignProvisionServerPort - Assigns an Apache listening port for a particular OpenStackProvisionServer.
func (instance *OpenStackProvisionServer) AssignProvisionServerPort(
	cond *shared.Condition,
	client goClient.Client,
	portStart int,
) error {
	if instance.Spec.Port != 0 {
		// Do nothing, already assigned
		return nil
	}

	existingPorts, err := instance.GetExistingProvServerPorts(cond, client)
	if err != nil {
		return err
	}

	// It's possible that this prov server already exists and we are just dealing with
	// a minimized version of it (only its ObjectMeta is set, etc)
	instance.Spec.Port = existingPorts[instance.GetName()]

	// If we get this far, no port has been previously assigned, so we pick one
	if instance.Spec.Port == 0 {
		cur := portStart

		for {
			found := false

			for _, port := range existingPorts {
				if port == cur {
					found = true
					break
				}
			}

			if !found {
				break
			}

			cur++
		}

		instance.Spec.Port = cur
	}

	return nil
}
