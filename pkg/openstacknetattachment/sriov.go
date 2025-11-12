package openstacknetattachment

import (
	"context"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSriovNetworksWithLabel - Returns list of sriovnetworks labeled with labelSelector
func GetSriovNetworksWithLabel(
	ctx context.Context,

	r common.ReconcilerCommon,
	labelSelector map[string]string,
	namespace string,
) (map[string]sriovnetworkv1.SriovNetwork, error) {
	sriovNetworks := &sriovnetworkv1.SriovNetworkList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(labelSelector),
	}
	if err := r.GetClient().List(ctx, sriovNetworks, listOpts...); err != nil {
		return nil, err
	}

	networkMap := map[string]sriovnetworkv1.SriovNetwork{}

	for _, network := range sriovNetworks.Items {
		networkMap[network.Name] = network
	}

	return networkMap, nil
}

// GetSriovNetworkNodePoliciesWithLabel - Returns list of sriovnetworknodepolicies labeled with labelSelector
func GetSriovNetworkNodePoliciesWithLabel(
	ctx context.Context,
	r common.ReconcilerCommon,
	labelSelector map[string]string,
	namespace string,
) (map[string]sriovnetworkv1.SriovNetworkNodePolicy, error) {
	sriovNetworkNodePolicies := &sriovnetworkv1.SriovNetworkNodePolicyList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(labelSelector),
	}
	if err := r.GetClient().List(ctx, sriovNetworkNodePolicies, listOpts...); err != nil {
		return nil, err
	}

	policyMap := map[string]sriovnetworkv1.SriovNetworkNodePolicy{}

	for _, policy := range sriovNetworkNodePolicies.Items {
		policyMap[policy.Name] = policy
	}

	return policyMap, nil
}
