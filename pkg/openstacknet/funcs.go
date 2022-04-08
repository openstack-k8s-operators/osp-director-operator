package openstacknet

import (
	"context"
	"sort"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstacknetattachment "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknetattachment"
)

// GetOpenStackNetsBindingMap - Returns map of OpenStackNet name to binding type
func GetOpenStackNetsBindingMap(
	ctx context.Context,
	r common.ReconcilerCommon,
	namespace string,
) (map[string]ospdirectorv1beta1.AttachType, error) {

	//
	// Acquire a list and map of all OpenStackNetworks available in this namespace
	//
	osNetList, err := ospdirectorv1beta1.GetOpenStackNetsWithLabel(r.GetClient(), namespace, map[string]string{})
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
			ctx,
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

//
// GetAllIPReservations - get all reservations from the osnet (already synamic created, static configured + just now new created)
//
func GetAllIPReservations(
	osNet *ospdirectorv1beta1.OpenStackNet,
	newReservations []ospdirectorv1beta1.IPReservation,
	staticReservations []ospdirectorv1beta1.IPReservation,
) []ospdirectorv1beta1.IPReservation {
	//
	// add just now new created
	//
	reservationList := newReservations

	//
	// add reservation already stored in the osnet.Status.Reservations
	//
	for hostname, res := range osNet.Status.Reservations {
		reservationList = append(
			reservationList,
			ospdirectorv1beta1.IPReservation{
				IP:       res.IP,
				Hostname: hostname,
				Deleted:  res.Deleted,
			},
		)

	}

	//
	// add new reservations from osnet.Spec.Reservations which are not yet synced to osnet.Status.Reservations
	//
	for _, role := range osNet.Spec.RoleReservations {
		for _, res := range role.Reservations {
			found := false
			for _, resList := range reservationList {
				if res.IP == resList.IP {
					found = true
					break
				}
			}
			if !found {
				reservationList = append(
					reservationList,
					ospdirectorv1beta1.IPReservation{
						IP:       res.IP,
						Hostname: res.Hostname,
						Deleted:  res.Deleted,
					},
				)
			}
		}
	}

	//
	// add new staticReservations provided by the osnetcfg CR
	//
	for _, staticRes := range staticReservations {
		found := false
		for _, res := range reservationList {
			if res.IP == staticRes.IP {
				found = true
				break
			}
		}
		if !found {
			reservationList = append(reservationList, staticRes)
		}
	}

	//
	// add staticReservations provided by osnetcfg CR
	//
	reservationList = append(reservationList, staticReservations...)

	// sort reservationList by IP
	sort.Slice(reservationList[:], func(i, j int) bool {
		return reservationList[i].IP < reservationList[j].IP
	})

	return reservationList
}
