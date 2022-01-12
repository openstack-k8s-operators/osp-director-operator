package openstacknetconfig

import (
	"context"
	"fmt"
	"time"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
) (reconcile.Result, error) {

	//
	// only add label if it is not already there
	// Note, any rename of the osnetcfg will be reflected
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

			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		} else if err != nil {
			cond.Message = fmt.Sprintf("Failed to get OpenStackNet %s ", networkNameLower)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonOSNetError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetError)
			err = common.WrapErrorForObject(cond.Message, obj, err)

			return ctrl.Result{}, err
		}

		//
		// get ownerReferences entry with Kind OpenStackNetConfig
		//
		for _, ownerRef := range osnet.ObjectMeta.OwnerReferences {
			if ownerRef.Kind == "OpenStackNetConfig" {
				//
				// update labels on obj
				//
				op, err := controllerutil.CreateOrUpdate(context.TODO(), r.GetClient(), obj, func() error {
					obj.SetLabels(
						labels.Merge(
							obj.GetLabels(),
							map[string]string{
								OpenStackNetConfigReconcileLabel: ownerRef.Name,
							},
						))

					return nil
				})
				if err != nil {
					cond.Message = fmt.Sprintf("Failed to update RefLabel label on %s %s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
					cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonAddRefLabelError)
					cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorCondTypeError)
					err = common.WrapErrorForObject(cond.Message, obj, err)

					return ctrl.Result{}, err
				}
				common.LogForObject(
					r,
					fmt.Sprintf("%s updated with %s:%s label: %s",
						obj.GetName(),
						OpenStackNetConfigReconcileLabel,
						ownerRef.Name,
						op,
					),
					obj,
				)

				return ctrl.Result{}, nil
			}
		}
	}

	return ctrl.Result{}, nil
}
