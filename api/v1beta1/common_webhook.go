package v1beta1

// Because webhooks *MUST* exist within the api/<version> package, we need to place common
// functions here that might be used across different Kinds' webhooks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	goClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// CheckRoleNameExists - This function is needed by webhooks for both BaremetalSets and VMSets
// to help ensure that role names are unique across a given namespace
func CheckRoleNameExists(client goClient.Client, typeMeta metav1.TypeMeta, objectMeta metav1.ObjectMeta, role string) error {
	existing, err := getRoleNames(client, objectMeta.Namespace)

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

func getRoleNames(client goClient.Client, namespace string) (map[string]string, error) {
	found := map[string]string{}

	// Get BaremetalSet role names
	baremetalSetsList := &BaremetalSetList{}
	listOpts := []goClient.ListOption{
		goClient.InNamespace(namespace),
	}

	err := client.List(context.TODO(), baremetalSetsList, listOpts...)
	if err != nil {
		return nil, err
	}

	for _, bmSet := range baremetalSetsList.Items {
		found[fmt.Sprintf("%s/%s", bmSet.Kind, bmSet.Name)] = bmSet.Spec.Role
	}

	// Get VMSet role names
	vmSetsList := &ControllerVMList{}

	err = client.List(context.TODO(), vmSetsList, listOpts...)
	if err != nil {
		return nil, err
	}

	for _, vmSet := range vmSetsList.Items {
		found[fmt.Sprintf("%s/%s", vmSet.Kind, vmSet.Name)] = vmSet.Spec.Role
	}

	// baremetalSetClient := client.Resource(BaremetalSetGVR)
	// controllerVMClient := client.Resource(ControllerVMGVR)

	// // Get roles from BaremetalSets
	// list, err := baremetalSetClient.Namespace(namespace).List(context.Background(), metav1.ListOptions{})

	// if err != nil {
	// 	return nil, err
	// }

	// foundBaremetalSetRoles := getRoleNames(list.Items)

	// // Get roles from ControllerVMs
	// list, err = controllerVMClient.Namespace(namespace).List(context.Background(), metav1.ListOptions{})

	// if err != nil {
	// 	return nil, err
	// }

	// foundControllerVMRoles := getRoleNames(list.Items)

	// // Now just merge them together in the first map
	// for key, value := range foundControllerVMRoles {
	// 	foundBaremetalSetRoles[key] = value
	// }

	// return foundBaremetalSetRoles, nil

	return found, nil
}
