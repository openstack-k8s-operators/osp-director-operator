package vmset

import (
	"context"

	sriovnetworkv1 "github.com/openshift/sriov-network-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSriovNetworkNodePoliciesWithLabel - Returns list of sriovnetworknodepolicies labeled with labelSelector
func GetSriovNetworkNodePoliciesWithLabel(c client.Client, labelSelector map[string]string, namespace string) (*sriovnetworkv1.SriovNetworkNodePolicyList, error) {
	sriovNetworkNodePolicies := &sriovnetworkv1.SriovNetworkNodePolicyList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(labelSelector),
	}
	if err := c.List(context.Background(), sriovNetworkNodePolicies, listOpts...); err != nil {
		return nil, err
	}

	return sriovNetworkNodePolicies, nil
}
