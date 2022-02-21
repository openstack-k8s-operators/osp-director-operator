package openstacknetconfig

import (
	"fmt"
	"time"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
	v1 "k8s.io/api/apps/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//
// AddOSNetConfigRefLabel - add osnetcfg CR label reference which is used in
// the in the osnetcfg controller to watch this resource and reconcile
//
func AddOSNetConfigRefLabel(
	r common.ReconcilerCommon,
	obj client.Object,
	cond *ospdirectorv1beta1.Condition,
	networkNameLower string,
) (map[string]string, reconcile.Result, error) {
	labels := obj.GetLabels()

	//
	// only add label if it is not already there
	// Note, any rename of the osnetcfg won't be reflected
	//
	if _, ok := obj.GetLabels()[OpenStackNetConfigReconcileLabel]; !ok {
		//
		// Get first OSnet from instance.Spec.Networks list
		//
		labelSelector := map[string]string{
			//
			// just use the first network in the list to get the ownerReferences
			//
			openstacknet.NetworkNameLowerLabelSelector: networkNameLower,
		}
		osnet, err := openstacknet.GetOpenStackNetWithLabel(r, obj.GetNamespace(), labelSelector)
		if err != nil && k8s_errors.IsNotFound(err) {
			cond.Message = fmt.Sprintf("OpenStackNet %s not found reconcile again in 10 seconds", networkNameLower)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonOSNetNotFound)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeWaiting)
			common.LogForObject(r, cond.Message, obj)

			return labels, ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		} else if err != nil {
			cond.Message = fmt.Sprintf("Failed to get OpenStackNet %s ", networkNameLower)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonOSNetError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetError)
			err = common.WrapErrorForObject(cond.Message, obj, err)

			return labels, ctrl.Result{}, err
		}

		//
		// get ownerReferences entry with Kind OpenStackNetConfig
		//
		for _, ownerRef := range osnet.ObjectMeta.OwnerReferences {
			if ownerRef.Kind == "OpenStackNetConfig" {
				//
				// merge with obj labels
				//
				labels = common.MergeStringMaps(
					labels,
					map[string]string{
						OpenStackNetConfigReconcileLabel: ownerRef.Name,
					},
				)

				common.LogForObject(
					r,
					fmt.Sprintf("%s updated with %s:%s label",
						obj.GetName(),
						OpenStackNetConfigReconcileLabel,
						ownerRef.Name,
					),
					obj,
				)
				break
			}
		}
	}

	return labels, ctrl.Result{}, nil
}

//
// WaitOnIPsCreated - Wait for IPs created on all configured networks
//
func WaitOnIPsCreated(
	r common.ReconcilerCommon,
	obj client.Object,
	cond *ospdirectorv1beta1.Condition,
	osnetcfg *ospdirectorv1beta1.OpenStackNetConfig,
	networks []string,
	hostname string,
	hostStatus *ospdirectorv1beta1.HostStatus,
) error {
	//
	// verify that we have the host entry on the status of the osnetcfg object
	//
	var osnetcfgHostStatus ospdirectorv1beta1.OpenStackHostStatus
	var ok bool
	if osnetcfgHostStatus, ok = osnetcfg.Status.Hosts[hostname]; !ok {
		common.LogForObject(
			r,
			fmt.Sprintf("%s %s waiting on node %s to be added to %s config %s",
				obj.GetObjectKind().GroupVersionKind().Kind,
				obj.GetName(),
				hostname,
				osnetcfg.Kind,
				osnetcfg.Name,
			),
			obj,
		)
		cond.Message = fmt.Sprintf("%s %s waiting on IPs to be created for all nodes and networks",
			obj.GetObjectKind().GroupVersionKind().Kind,
			obj.GetName(),
		)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.NetConfigCondReasonWaitingOnIPsForHost)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeWaiting)

		return k8s_errors.NewNotFound(v1.Resource(obj.GetObjectKind().GroupVersionKind().Kind), cond.Message)
	}

	//
	// verify we have IPs on all required networks
	//
	for _, osNet := range networks {
		if ip, ok := osnetcfgHostStatus.IPAddresses[osNet]; ok && ip != "" {
			hostStatus.IPAddresses[osNet] = ip
			continue
		}
		common.LogForObject(
			r,
			fmt.Sprintf("%s %s waiting on IP address for node %s on network %s to be available",
				obj.GetObjectKind().GroupVersionKind().Kind,
				obj.GetName(),
				hostname,
				osNet,
			),
			obj,
		)

		cond.Message = fmt.Sprintf("%s %s waiting on IPs to be created for all nodes and networks",
			obj.GetObjectKind().GroupVersionKind().Kind,
			obj.GetName(),
		)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.NetConfigCondReasonWaitingOnIPsForHost)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeWaiting)

		return k8s_errors.NewNotFound(v1.Resource(obj.GetObjectKind().GroupVersionKind().Kind), cond.Message)
	}

	return nil
}
