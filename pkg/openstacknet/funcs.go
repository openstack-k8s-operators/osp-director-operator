package openstacknet

import (
	"context"
	"fmt"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstacknetattachment "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknetattachment"
	v1 "k8s.io/api/apps/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetOpenStackNetsBindingMap - Returns map of OpenStackNet name to binding type
func GetOpenStackNetsBindingMap(r common.ReconcilerCommon, namespace string) (map[string]ospdirectorv1beta1.AttachType, error) {

	//
	// Acquire a list and map of all OpenStackNetworks available in this namespace
	//
	osNetList, err := GetOpenStackNetsWithLabel(r, namespace, map[string]string{})
	if err != nil {
		return nil, err
	}

	//
	// mapping of osNet name and binding type
	//
	osNetBindings := map[string]ospdirectorv1beta1.AttachType{}
	for _, osNet := range osNetList.Items {

		//
		// get osnetattachment used by this network
		//
		attachType, err := openstacknetattachment.GetOpenStackNetAttachmentType(
			r,
			namespace,
			osNet.Spec.AttachConfiguration,
		)
		if err != nil {
			return nil, err
		}

		osNetBindings[osNet.Spec.NameLower] = *attachType
	}

	return osNetBindings, nil
}

// GetOpenStackNetsWithLabel - Return a list of all OpenStackNets in the namespace that have (optional) labels
func GetOpenStackNetsWithLabel(r common.ReconcilerCommon, namespace string, labelSelector map[string]string) (*ospdirectorv1beta1.OpenStackNetList, error) {
	osNetList := &ospdirectorv1beta1.OpenStackNetList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	if len(labelSelector) > 0 {
		labels := client.MatchingLabels(labelSelector)
		listOpts = append(listOpts, labels)
	}

	if err := r.GetClient().List(context.Background(), osNetList, listOpts...); err != nil {
		return nil, err
	}

	return osNetList, nil
}

// GetOpenStackNetWithLabel - Return OpenStackNet with labels
func GetOpenStackNetWithLabel(r common.ReconcilerCommon, namespace string, labelSelector map[string]string) (*ospdirectorv1beta1.OpenStackNet, error) {

	osNetList, err := GetOpenStackNetsWithLabel(
		r,
		namespace,
		labelSelector,
	)
	if err != nil {
		return nil, err
	}
	if len(osNetList.Items) == 0 {
		return nil, k8s_errors.NewNotFound(v1.Resource("openstacknet"), fmt.Sprint(labelSelector))
	} else if len(osNetList.Items) > 1 {
		return nil, fmt.Errorf("multiple OpenStackNet with label %v not found", labelSelector)
	}
	return &osNetList.Items[0], nil
}
