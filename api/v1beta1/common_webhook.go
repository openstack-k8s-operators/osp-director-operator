package v1beta1

// Because webhooks *MUST* exist within the api/<version> package, we need to place common
// functions here that might be used across different Kinds' webhooks

import (
	"context"
	"fmt"

	yaml "gopkg.in/yaml.v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	goClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Client needed for API calls (manager's client, set by first SetupWebhookWithManager() call
// to any particular webhook)
var webhookClient goClient.Client

// checkRoleNameExists - This function is needed by webhooks for both OpenStackBaremetalSets and VMSets
// to help ensure that role names are unique across a given namespace
func checkRoleNameExists(typeMeta metav1.TypeMeta, objectMeta metav1.ObjectMeta, role string) error {
	existing, err := getRoleNames(objectMeta.Namespace)

	if err != nil {
		return err
	}

	for resourceName, roleName := range existing {
		// If the role name is already in use by another resource, this resource is invalid
		if resourceName != fmt.Sprintf("%s/%s", typeMeta.Kind, objectMeta.Name) && roleName == role {
			return fmt.Errorf("Role \"%s\" is already in use by %s", roleName, resourceName)
		}
	}

	return nil
}

func getRoleNames(namespace string) (map[string]string, error) {
	found := map[string]string{}

	// Get OpenStackBaremetalSet role names
	baremetalSetsList := &OpenStackBaremetalSetList{}
	listOpts := []goClient.ListOption{
		goClient.InNamespace(namespace),
	}

	err := webhookClient.List(context.TODO(), baremetalSetsList, listOpts...)
	if err != nil {
		return nil, err
	}

	for _, bmSet := range baremetalSetsList.Items {
		found[fmt.Sprintf("%s/%s", bmSet.Kind, bmSet.Name)] = bmSet.Spec.RoleName
	}

	// Get VMSet role names
	vmSetsList := &OpenStackVMSetList{}

	err = webhookClient.List(context.TODO(), vmSetsList, listOpts...)
	if err != nil {
		return nil, err
	}

	for _, vmSet := range vmSetsList.Items {
		found[fmt.Sprintf("%s/%s", vmSet.Kind, vmSet.Name)] = vmSet.Spec.RoleName
	}

	return found, nil
}

// checkOspNetworkDefinition - Currently, this function:
// - Checks to make sure a BridgeName is provided if a NodeNetworkConfigurationPolicy was also provided in a Network object
// - Checks to make sure the BridgeName provided in NodeNetworkConfigurationPolicy matches that provided in the Network object
func checkOspNetworkDefinition(objectMeta metav1.ObjectMeta, ospNetwork Network) error {
	if ospNetwork.NodeNetworkConfigurationPolicy.DesiredState.String() != "" {
		if ospNetwork.BridgeName == "" {
			return fmt.Errorf("nodeNetworkConfigurationPolicy requires a \"bridgeName\" in the encompassing network object")
		}

		rawState := map[string]interface{}{}

		err := yaml.Unmarshal(ospNetwork.NodeNetworkConfigurationPolicy.DesiredState.Raw, rawState)

		if err != nil {
			return err
		}

		// Looking for something like this:
		//
		// interfaces:
		// - bridge:
		//     options:
		//       stp:
		//         enabled: false
		//     port:
		//     - name: enp6s0
		//   description: Linux bridge with enp6s0 as a port
		//   name: br-osp      <----- checking this
		//   state: up
		//   type: linux-bridge

		foundBridgeName := false

		if interfaces, ok := rawState["interfaces"].([]interface{}); ok {
			for _, interfaceItemIntf := range interfaces {
				if interfaceItem, ok := interfaceItemIntf.(map[interface{}]interface{}); ok {
					if _, ok := interfaceItem["bridge"]; ok {
						if name, ok := interfaceItem["name"].(string); ok {
							if name == ospNetwork.BridgeName {
								foundBridgeName = true
								break
							}
						}
					}
				}
			}
		}

		if !foundBridgeName {
			return fmt.Errorf("nodeNetworkConfigurationPolicy's \"desiredState\" must contain a bridge name that matches the encompassing network object's \"bridgeName\"")
		}
	}

	return nil
}
