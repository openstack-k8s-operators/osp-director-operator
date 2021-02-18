package v1beta1

// Because webhooks *MUST* exist within the api/<version> package, we need to place common
// functions here that might be used across different Kinds' webhooks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	goClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Client needed for API calls (manager's client, set by first SetupWebhookWithManager() call
// to any particular webhook)
var webhookClient goClient.Client

// checkRoleNameExists - This function is needed by webhooks for both BaremetalSets and VMSets
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

	// Get BaremetalSet role names
	baremetalSetsList := &OpenStackBaremetalSetList{}
	listOpts := []goClient.ListOption{
		goClient.InNamespace(namespace),
	}

	err := webhookClient.List(context.TODO(), baremetalSetsList, listOpts...)
	if err != nil {
		return nil, err
	}

	for _, bmSet := range baremetalSetsList.Items {
		found[fmt.Sprintf("%s/%s", bmSet.Kind, bmSet.Name)] = bmSet.Spec.Role
	}

	// Get VMSet role names
	vmSetsList := &VMSetList{}

	err = webhookClient.List(context.TODO(), vmSetsList, listOpts...)
	if err != nil {
		return nil, err
	}

	for _, vmSet := range vmSetsList.Items {
		found[fmt.Sprintf("%s/%s", vmSet.Kind, vmSet.Name)] = vmSet.Spec.Role
	}

	return found, nil
}
