package v1beta1

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	v1 "k8s.io/api/apps/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AddOSNetNameLowerLabels - add osnetcfg CR label reference which is used in
// the in the osnetcfg controller to watch this resource and reconcile
func AddOSNetNameLowerLabels(
	log logr.Logger,
	labels map[string]string,
	networkNameLowerNames []string,
) map[string]string {
	osNetLabels := map[string]string{}
	removedOsNets := map[string]bool{}
	newOsNets := map[string]bool{}
	networkNameLowerNamesMap := map[string]bool{}

	for _, n := range networkNameLowerNames {
		label := fmt.Sprintf("%s/%s", shared.SubNetNameLabelSelector, n)
		networkNameLowerNamesMap[label] = true
	}

	//
	// get current osNet Labels and verify if nets got removed
	//
	for _, label := range reflect.ValueOf(labels).MapKeys() {
		//
		// has label key SubNetNameLabelSelector string included?
		//
		if strings.HasPrefix(label.String(), shared.SubNetNameLabelSelector) {
			l := label.String()
			osNetLabels[l] = labels[l]

			//
			// if l is not in networkNameLowerNamesMap it got removed
			//
			if _, ok := networkNameLowerNamesMap[l]; !ok {
				delete(labels, l)
				removedOsNets[l] = true
			}
		}
	}

	if len(removedOsNets) > 0 {
		log.Info(fmt.Sprintf("removing network labels: %v",
			removedOsNets,
		))
	}

	//
	// identify if nets got added
	//
	for label := range networkNameLowerNamesMap {
		//
		// if label is not in osNetLabels its a new one
		//
		if _, ok := osNetLabels[label]; !ok {
			labels[label] = strconv.FormatBool(true)
			newOsNets[label] = true
		}

	}

	if len(newOsNets) > 0 {
		log.Info(fmt.Sprintf("adding network labels: %v",
			newOsNets,
		))
	}

	return labels
}

// AddOSNetConfigRefLabel - add osnetcfg CR label reference which is used in
// the in the osnetcfg controller to watch this resource and reconcile
func AddOSNetConfigRefLabel(
	c client.Client,
	namespace string,
	subnetName string,
	labels map[string]string,
) (map[string]string, error) {

	//
	// Get OSnet with SubNetNameLabelSelector: subnetName
	//
	labelSelector := map[string]string{
		shared.SubNetNameLabelSelector: subnetName,
	}
	osnet, err := GetOpenStackNetWithLabel(c, namespace, labelSelector)
	if err != nil && k8s_errors.IsNotFound(err) {
		return labels, fmt.Errorf("OpenStackNet %s not found reconcile again in 10 seconds", subnetName)
	} else if err != nil {
		return labels, fmt.Errorf("failed to get OpenStackNet %s ", subnetName)
	}

	//
	// get ownerReferences entry with Kind OpenStackNetConfig
	//
	for _, ownerRef := range osnet.OwnerReferences {
		if ownerRef.Kind == "OpenStackNetConfig" {
			//
			// merge with obj labels
			//
			labels = shared.MergeStringMaps(
				labels,
				map[string]string{
					shared.OpenStackNetConfigReconcileLabel: ownerRef.Name,
				},
			)

			break
		}
	}

	if _, ok := labels[shared.OpenStackNetConfigReconcileLabel]; !ok {
		return labels, fmt.Errorf("OpenStackNet %s misses OpenStackNetConfig owner reference", subnetName)
	}

	return labels, nil
}

// GetOpenStackNetsWithLabel - Return a list of all OpenStackNets in the namespace that have (optional) labels
func GetOpenStackNetsWithLabel(
	c client.Client,
	namespace string,
	labelSelector map[string]string,
) (*OpenStackNetList, error) {
	osNetList := &OpenStackNetList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	if len(labelSelector) > 0 {
		labels := client.MatchingLabels(labelSelector)
		listOpts = append(listOpts, labels)
	}

	if err := c.List(context.TODO(), osNetList, listOpts...); err != nil {
		return nil, err
	}

	return osNetList, nil
}

// GetOpenStackNetWithLabel - Return OpenStackNet with labels
func GetOpenStackNetWithLabel(
	c client.Client,
	namespace string,
	labelSelector map[string]string,
) (*OpenStackNet, error) {

	osNetList, err := GetOpenStackNetsWithLabel(
		c,
		namespace,
		labelSelector,
	)
	if err != nil {
		return nil, err
	}
	if len(osNetList.Items) == 0 {
		return nil, k8s_errors.NewNotFound(v1.Resource("openstacknet"), fmt.Sprint(labelSelector))
	} else if len(osNetList.Items) > 1 {
		return nil, fmt.Errorf("multiple OpenStackNet with label %v found", labelSelector)
	}
	return &osNetList.Items[0], nil
}

// GetOpenStackNetsMapWithLabel - Return a map[NameLower] of all OpenStackNets in the namespace that have (optional) labels
func GetOpenStackNetsMapWithLabel(
	c client.Client,
	namespace string,
	labelSelector map[string]string,
) (map[string]OpenStackNet, error) {
	osNetList, err := GetOpenStackNetsWithLabel(
		c,
		namespace,
		labelSelector,
	)
	if err != nil {
		return nil, err
	}

	osNetMap := map[string]OpenStackNet{}
	for _, osNet := range osNetList.Items {
		osNetMap[osNet.Spec.NameLower] = osNet
	}

	return osNetMap, nil
}

// GetOsNetCfg -
func GetOsNetCfg(
	c client.Client,
	namespace string,
	osNetCfgName string,
) (*OpenStackNetConfig, error) {

	osNetCfg := &OpenStackNetConfig{}
	err := c.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      osNetCfgName,
			Namespace: namespace,
		},
		osNetCfg)

	if err != nil {
		if k8s_errors.IsNotFound(err) {
			return nil, fmt.Errorf("%s with name %s not found", osNetCfg.DeepCopy().GroupVersionKind().Kind, osNetCfg.Name)
		}
		return nil, err
	}

	return osNetCfg, nil
}

// ValidateNetworks - validate that for all configured subnets an osnet exists
func ValidateNetworks(namespace string, networks []string) error {
	for _, subnetName := range networks {
		//
		// Get OSnet with SubNetNameLabelSelector: subnetName
		//
		labelSelector := map[string]string{
			shared.SubNetNameLabelSelector: subnetName,
		}
		_, err := GetOpenStackNetWithLabel(webhookClient, namespace, labelSelector)
		if err != nil && k8s_errors.IsNotFound(err) {
			return fmt.Errorf("OpenStackNet %s not found, validate the object network list", subnetName)
		} else if err != nil {
			return fmt.Errorf("failed to get OpenStackNet %s", subnetName)
		}
	}

	return nil
}
