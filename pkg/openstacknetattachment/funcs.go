package openstacknetattachment

import (
	"context"
	"fmt"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/common"

	v1 "k8s.io/api/apps/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetOpenStackNetsBindingMap - Returns map of OpenStackNet name to binding type
func GetOpenStackNetsBindingMap(r common.ReconcilerCommon, namespace string) (map[string]string, error) {
	// Acquire a list and map of all OpenStackNetworks available in this namespace
	osNetList, err := GetOpenStackNetsWithLabel(r, namespace, map[string]string{})

	if err != nil {
		return nil, err
	}

	osNets := map[string]string{}
	for _, osNet := range osNetList.Items {
		/*
			// Currently there are two binding types: SRIOV and...not-SRIOV.
			// If a network has SRIOV configuration, then that currently overrides any non-SRIOV configuration it might have
			if osNet.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.Port != "" {
				osNets[osNet.Spec.NameLower] = "sriov"
			} else {
				osNets[osNet.Spec.NameLower] = ""
			}
		*/
		osNets[osNet.Spec.NameLower] = ""
	}

	return osNets, nil
}

// GetOpenStackNetsAttachConfigBridgeNames - Return a map of OpenStackNet names in the passed namespace to the bridge-name in their attachConfig
func GetOpenStackNetsAttachConfigBridgeNames(r common.ReconcilerCommon, namespace string) (map[string]string, error) {
	osNets, err := GetOpenStackNetsWithLabel(r, namespace, map[string]string{})

	if err != nil {
		return nil, err
	}

	osNetBridgeNames := map[string]string{}

	for _, osNet := range osNets.Items {
		/*
			desiredStateBytes := osNet.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy.DesiredState.Raw

			if len(desiredStateBytes) > 0 {
				bridgeName, err := nmstate.GetDesiredStatedBridgeName(desiredStateBytes)

				if err != nil {
					return nil, err
				}

				osNetBridgeNames[osNet.Spec.NameLower] = bridgeName
			}
		*/
		osNetBridgeNames[osNet.Spec.NameLower] = "br-osp"
	}

	return osNetBridgeNames, nil
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
		//return nil, fmt.Errorf("OpenStackNet with label %v not found", labelSelector)
	} else if len(osNetList.Items) > 1 {
		return nil, fmt.Errorf("multiple OpenStackNet with label %v not found", labelSelector)
	}
	return &osNetList.Items[0], nil
}
