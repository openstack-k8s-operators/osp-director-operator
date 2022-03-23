package v1beta1

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// AddOSNetNameLowerLabels - add osnetcfg CR label reference which is used in
// the in the osnetcfg controller to watch this resource and reconcile
//
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
		label := fmt.Sprintf("%s/%s", SubNetNameLabelSelector, n)
		networkNameLowerNamesMap[label] = true
	}

	//
	// get current osNet Labels and verify if nets got removed
	//
	for _, label := range reflect.ValueOf(labels).MapKeys() {
		//
		// has label key SubNetNameLabelSelector string included?
		//
		if strings.HasSuffix(label.String(), SubNetNameLabelSelector) {
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

	if len(newOsNets) > 0 {
		controlplanelog.Info(fmt.Sprintf("removing network labels: %v",
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
		controlplanelog.Info(fmt.Sprintf("adding network labels: %v",
			newOsNets,
		))
	}

	return labels
}

//
// AddOSNetConfigRefLabel - add osnetcfg CR label reference which is used in
// the in the osnetcfg controller to watch this resource and reconcile
//
func AddOSNetConfigRefLabel(
	namespace string,
	subnetName string,
	labels map[string]string,
) (map[string]string, error) {

	//
	// Get OSnet with SubNetNameLabelSelector: subnetName
	//
	labelSelector := map[string]string{
		SubNetNameLabelSelector: subnetName,
	}
	osnet, err := GetOpenStackNetWithLabel(namespace, labelSelector)
	if err != nil && k8s_errors.IsNotFound(err) {
		return labels, fmt.Errorf(fmt.Sprintf("OpenStackNet %s not found reconcile again in 10 seconds", subnetName))
	} else if err != nil {
		return labels, fmt.Errorf(fmt.Sprintf("Failed to get OpenStackNet %s ", subnetName))
	}

	//
	// get ownerReferences entry with Kind OpenStackNetConfig
	//
	for _, ownerRef := range osnet.ObjectMeta.OwnerReferences {
		if ownerRef.Kind == "OpenStackNetConfig" {
			//
			// merge with obj labels
			//
			labels = MergeStringMaps(
				labels,
				map[string]string{
					OpenStackNetConfigReconcileLabel: ownerRef.Name,
				},
			)

			break
		}
	}

	return labels, nil
}

// GetOpenStackNetsWithLabel - Return a list of all OpenStackNets in the namespace that have (optional) labels
func GetOpenStackNetsWithLabel(
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

	if err := webhookClient.List(context.TODO(), osNetList, listOpts...); err != nil {
		return nil, err
	}

	return osNetList, nil
}

// GetOpenStackNetWithLabel - Return OpenStackNet with labels
func GetOpenStackNetWithLabel(
	namespace string,
	labelSelector map[string]string,
) (*OpenStackNet, error) {

	osNetList, err := GetOpenStackNetsWithLabel(
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

// GetOpenStackNetsMapWithLabel - Return a map[NameLower] of all OpenStackNets in the namespace that have (optional) labels
func GetOpenStackNetsMapWithLabel(
	namespace string,
	labelSelector map[string]string,
) (map[string]OpenStackNet, error) {
	osNetList, err := GetOpenStackNetsWithLabel(
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

// CreateVIPNetworkList - return list of all networks from all VM roles which has vip flag
func CreateVIPNetworkList(
	instance *OpenStackControlPlane,
) ([]string, error) {

	// create uniq list networls of all VirtualMachineRoles
	networkList := make(map[string]bool)
	uniqNetworksList := []string{}

	for _, vmRole := range instance.Spec.VirtualMachineRoles {
		for _, netNameLower := range vmRole.Networks {
			// get network with name_lower label
			labelSelector := map[string]string{
				SubNetNameLabelSelector: netNameLower,
			}

			// get network with name_lower label to verify if VIP needs to be requested from Spec
			network, err := GetOpenStackNetWithLabel(
				instance.Namespace,
				labelSelector,
			)
			if err != nil {
				if k8s_errors.IsNotFound(err) {
					return uniqNetworksList, fmt.Errorf(fmt.Sprintf("OpenStackNet with NameLower %s not found!", netNameLower))
				}
				// Error reading the object - requeue the request.
				return uniqNetworksList, fmt.Errorf(fmt.Sprintf("Error getting OSNet with labelSelector %v", labelSelector))
			}

			if _, value := networkList[netNameLower]; !value && network.Spec.VIP {
				networkList[netNameLower] = true
				uniqNetworksList = append(uniqNetworksList, netNameLower)
			}
		}
	}

	return uniqNetworksList, nil
}
