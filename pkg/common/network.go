/*
Copyright 2020 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"encoding/json"
	"fmt"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/nmstate/kubernetes-nmstate/api/shared"
	nmstate "github.com/nmstate/kubernetes-nmstate/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// NetworkConfigurationPolicy -
type NetworkConfigurationPolicy struct {
	Name                           string
	Labels                         map[string]string
	NodeNetworkConfigurationPolicy shared.NodeNetworkConfigurationPolicySpec
}

// CreateOrUpdateNetworkConfigurationPolicy - create or update NetworkConfigurationPolicy
func CreateOrUpdateNetworkConfigurationPolicy(r ReconcilerCommon, obj metav1.Object, managerRef string, ncp *NetworkConfigurationPolicy) error {

	networkConfigurationPolicy := nmstate.NodeNetworkConfigurationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ncp.Name,
			Labels: ncp.Labels,
		},
		Spec: ncp.NodeNetworkConfigurationPolicy,
	}
	networkConfigurationPolicy.Kind = "NodeNetworkConfigurationPolicy"
	networkConfigurationPolicy.APIVersion = "nmstate.io/v1alpha1"

	b, err := json.Marshal(networkConfigurationPolicy.DeepCopyObject())
	if err != nil {
		return err
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	// apply object using server side apply
	if err := SSAApplyYaml(context.TODO(), cfg, managerRef, b); err != nil {
		r.GetLogger().Error(err, fmt.Sprintf("Failed to apply networkConfigurationPolicy object %s", networkConfigurationPolicy.GetName()))
		return err
	}

	return nil
}

// GetAllNetworkConfigurationPolicies - get all NetworkConfigurationPolicy
func GetAllNetworkConfigurationPolicies(r ReconcilerCommon, labelSelectorMap map[string]string) (map[string]nmstate.NodeNetworkConfigurationPolicy, error) {

	nncpMap := make(map[string]nmstate.NodeNetworkConfigurationPolicy)

	nncpList := &nmstate.NodeNetworkConfigurationPolicyList{}

	nncpListOpts := []client.ListOption{
		client.MatchingLabels(labelSelectorMap),
	}

	err := r.GetClient().List(context.TODO(), nncpList, nncpListOpts...)
	if err != nil {
		return nncpMap, err
	}

	for _, nncp := range nncpList.Items {
		nncpMap[nncp.Name] = nncp
	}

	return nncpMap, nil
}

// NetworkAttachmentDefinition -
type NetworkAttachmentDefinition struct {
	Name      string
	Namespace string
	Labels    map[string]string
	Data      map[string]string
}

const (
	// cniConfigTemplate -
	cniConfigTemplate = `
{
    "cniVersion": "0.3.1",
    "name": "{{ .Name }}",
    "plugins": [
	{
	    "type": "bridge",
	    "bridge": "{{ .BridgeName }}",
{{- if ne .Vlan "0"}}
            "vlan": {{ .Vlan }},
{{- end }}
	    "ipam": {
{{- if .Static }}
                "type": "static"
{{- end }}
	    }
	},
	{
	    "type": "tuning"
	}
    ]
}
`
)

// CreateOrUpdateNetworkAttachmentDefinition - create or update NetworkConfigurationPolicy
func CreateOrUpdateNetworkAttachmentDefinition(r ReconcilerCommon, obj metav1.Object, managerRef string, ownerRef *metav1.OwnerReference, nad *NetworkAttachmentDefinition) error {

	// render CNIConfigTemplate
	CNIConfig := ExecuteTemplateData(cniConfigTemplate, nad.Data)
	if err := isJSON(CNIConfig); err != nil {
		return fmt.Errorf("Failure rendering CNIConfig for NetworkAttachmentDefinition %s: %v", nad.Name, nad.Data)
	}

	networkAttachmentDefinition := networkv1.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nad.Name,
			Namespace: nad.Namespace,
			Annotations: map[string]string{
				"k8s.v1.cni.cncf.io/resourceName": fmt.Sprintf("bridge.network.kubevirt.io/%s", nad.Data["BridgeName"]),
			},
			Labels: nad.Labels,
		},
		Spec: networkv1.NetworkAttachmentDefinitionSpec{
			Config: CNIConfig,
		},
	}
	networkAttachmentDefinition.Kind = "NetworkAttachmentDefinition"
	networkAttachmentDefinition.APIVersion = "k8s.cni.cncf.io/v1"

	// set owner ref
	networkAttachmentDefinition.SetOwnerReferences([]metav1.OwnerReference{*ownerRef})

	b, err := json.Marshal(networkAttachmentDefinition.DeepCopyObject())
	if err != nil {
		return err
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	// apply object using server side apply
	if err := SSAApplyYaml(context.TODO(), cfg, managerRef, b); err != nil {
		r.GetLogger().Error(err, fmt.Sprintf("Failed to apply networkAttachmentDefinition object %s", networkAttachmentDefinition.GetName()))
		return err
	}

	return nil
}

// GetAllNetworkAttachmentDefinitions - get all NetworkAttachmentDefinition
func GetAllNetworkAttachmentDefinitions(r ReconcilerCommon, obj metav1.Object) (map[string]networkv1.NetworkAttachmentDefinition, error) {

	nadMap := make(map[string]networkv1.NetworkAttachmentDefinition)

	nadList := &networkv1.NetworkAttachmentDefinitionList{}

	nadListOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
	}

	err := r.GetClient().List(context.TODO(), nadList, nadListOpts...)
	if err != nil {
		return nadMap, err
	}

	for _, nad := range nadList.Items {
		nadMap[nad.Name] = nad
	}

	return nadMap, nil
}
