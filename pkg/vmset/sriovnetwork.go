package vmset

import (
	"context"

	sriovnetworkv1 "github.com/openshift/sriov-network-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSriovNetworksWithLabel - Returns list of sriovnetworks labeled with labelSelector
func GetSriovNetworksWithLabel(c client.Client, labelSelector map[string]string, namespace string) (*sriovnetworkv1.SriovNetworkList, error) {
	sriovNetworks := &sriovnetworkv1.SriovNetworkList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(labelSelector),
	}
	if err := c.List(context.Background(), sriovNetworks, listOpts...); err != nil {
		return nil, err
	}

	return sriovNetworks, nil
}
