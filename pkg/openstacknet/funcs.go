package openstacknet

import (
	"context"

	"github.com/ghodss/yaml"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	"github.com/tidwall/gjson"
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

// GetOpenStackNetsAttachConfigBridgeNames - Return a map of OpenStackNet names in the passed namespace to the bridge-name in their attachConfig
func GetOpenStackNetsAttachConfigBridgeNames(r common.ReconcilerCommon, namespace string) (map[string]string, error) {
	osNets, err := GetOpenStackNetsWithLabel(r, namespace, map[string]string{})

	if err != nil {
		return nil, err
	}

	osNetBridgeNames := map[string]string{}

	for _, osNet := range osNets.Items {
		desiredStateBytes := osNet.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy.DesiredState.Raw

		if len(desiredStateBytes) > 0 {
			jsonBytes, err := yaml.YAMLToJSON(desiredStateBytes)

			if err != nil {
				return nil, err
			}

			jsonStr := string(jsonBytes)

			if gjson.Get(jsonStr, "interfaces.#.name").Exists() {
				osNetBridgeNames[osNet.Name] = gjson.Get(jsonStr, "interfaces.#.name").Array()[0].String()
			}
		}
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
