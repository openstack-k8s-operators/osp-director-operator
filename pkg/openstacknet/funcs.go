package openstacknet

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstacknetattachment "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknetattachment"
	v1 "k8s.io/api/apps/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

// GetOpenStackNetsMapWithLabel - Return a map[NameLower] of all OpenStackNets in the namespace that have (optional) labels
func GetOpenStackNetsMapWithLabel(
	r common.ReconcilerCommon,
	namespace string,
	labelSelector map[string]string,
) (map[string]ospdirectorv1beta1.OpenStackNet, error) {
	osNetList, err := GetOpenStackNetsWithLabel(
		r,
		namespace,
		labelSelector,
	)
	if err != nil {
		return nil, err
	}

	osNetMap := map[string]ospdirectorv1beta1.OpenStackNet{}
	for _, osNet := range osNetList.Items {
		osNetMap[osNet.Spec.NameLower] = osNet
	}

	return osNetMap, nil
}

//
// AddOSNetNameLowerLabels - add osnetcfg CR label reference which is used in
// the in the osnetcfg controller to watch this resource and reconcile
//
func AddOSNetNameLowerLabels(
	r common.ReconcilerCommon,
	obj client.Object,
	cond *ospdirectorv1beta1.Condition,
	networkNameLowerNames []string,
) error {

	currentLabels := obj.GetLabels()
	if currentLabels == nil {
		currentLabels = map[string]string{}
	}

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
	for _, label := range reflect.ValueOf(currentLabels).MapKeys() {
		//
		// has label key SubNetNameLabelSelector string included?
		//
		if strings.HasSuffix(label.String(), SubNetNameLabelSelector) {
			l := label.String()
			osNetLabels[l] = currentLabels[l]

			//
			// if l is not in networkNameLowerNamesMap it got removed
			//
			if _, ok := networkNameLowerNamesMap[l]; !ok {
				delete(currentLabels, l)
				removedOsNets[l] = true

			}
		}
	}

	if len(newOsNets) > 0 {
		common.LogForObject(
			r,
			fmt.Sprintf("%s %s removing network labels: %v",
				obj.GetObjectKind().GroupVersionKind().Kind,
				obj.GetName(),
				removedOsNets,
			),
			obj,
		)
	}

	//
	// identify if nets got added
	//
	for label := range networkNameLowerNamesMap {
		//
		// if label is not in osNetLabels its a new one
		//
		if _, ok := osNetLabels[label]; !ok {
			currentLabels[label] = strconv.FormatBool(true)
			newOsNets[label] = true
		}

	}

	if len(newOsNets) > 0 {
		common.LogForObject(
			r,
			fmt.Sprintf("%s %s adding network labels: %v",
				obj.GetObjectKind().GroupVersionKind().Kind,
				obj.GetName(),
				newOsNets,
			),
			obj,
		)
	}

	//
	// update labels on obj
	//
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.GetClient(), obj, func() error {
		obj.SetLabels(
			labels.Merge(
				obj.GetLabels(),
				currentLabels,
			))

		return nil
	})
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to update %s labels on %s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonAddOSNetLabelError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, obj, err)

		return err
	}

	return nil
}

//
// GetAllIPReservations - get all reservations from the osnet (already synamic created, static configured + just now new created)
//
func GetAllIPReservations(
	osNet *ospdirectorv1beta1.OpenStackNet,
	newReservations []ospdirectorv1beta1.IPReservation,
) []ospdirectorv1beta1.IPReservation {
	//
	// add just now new created
	//
	reservationList := newReservations

	//
	// add already synamic created
	//
	for _, roleReservations := range osNet.Spec.RoleReservations {
		reservationList = append(reservationList, roleReservations.Reservations...)
	}

	// TODO static reservation
	/*
		//
		// add static configured reservations
		//
		for node, res := range instance.Spec.XX.StaticReservations {
			reservations[node] = res
		}
	*/

	// sort reservationList by IP
	sort.Slice(reservationList[:], func(i, j int) bool {
		return reservationList[i].IP < reservationList[j].IP
	})

	return reservationList
}
