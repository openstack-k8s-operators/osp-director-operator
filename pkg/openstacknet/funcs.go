package openstacknet

import (
	"context"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetOpenStackNetsBindingMap - Returns map of OpenStackNet name to binding type
func GetOpenStackNetsBindingMap(r common.ReconcilerCommon, namespace string) (map[string]string, error) {
	// Acquire a list and map of all OpenStackNetworks available in this namespace
	osNetList := &ospdirectorv1beta1.OpenStackNetList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	if err := r.GetClient().List(context.Background(), osNetList, listOpts...); err != nil {
		return nil, err
	}

	osNets := map[string]string{}
	for _, osNet := range osNetList.Items {
		// Currently there are two binding types: SRIOV and...not-SRIOV.
		// If a network has SRIOV configuration, then that currently overrides any non-SRIOV configuration it might have
		if osNet.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.Port != "" {
			osNets[osNet.Name] = "sriov"
		} else {
			osNets[osNet.Name] = ""
		}
	}

	return osNets, nil
}
