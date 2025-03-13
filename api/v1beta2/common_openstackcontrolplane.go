package v1beta2

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// GetControlPlane - Get OSP ControlPlane CR where e.g. the status information has the
// OSP version: controlPlane.Status.OSPVersion
// FIXME: We assume there is only one ControlPlane CR for now (enforced by webhook), but this might need to change
func GetControlPlane(
	c client.Client,
	obj metav1.Object,
) (OpenStackControlPlane, reconcile.Result, error) {

	controlPlane := OpenStackControlPlane{}
	controlPlaneList := &OpenStackControlPlaneList{}
	controlPlaneListOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
		client.Limit(1000),
	}
	err := c.List(context.TODO(), controlPlaneList, controlPlaneListOpts...)
	if err != nil {
		return controlPlane, ctrl.Result{}, err
	}

	if len(controlPlaneList.Items) == 0 {
		err := fmt.Errorf("no OpenStackControlPlanes found in namespace %s. Requeing", obj.GetNamespace())
		return controlPlane, ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	// FIXME: See FIXME above
	controlPlane = controlPlaneList.Items[0]

	return controlPlane, ctrl.Result{}, nil

}

// CreateVIPNetworkList - return list of all networks from all VM roles which has vip flag
func CreateVIPNetworkList(
	c client.Client,
	instance *OpenStackControlPlane,
) ([]string, error) {

	// create uniq list networls of all VirtualMachineRoles
	networkList := make(map[string]bool)
	uniqNetworksList := []string{}

	for _, vmRole := range instance.Spec.VirtualMachineRoles {
		for _, netNameLower := range vmRole.Networks {
			// get network with name_lower label
			labelSelector := map[string]string{
				shared.SubNetNameLabelSelector: netNameLower,
			}

			// get network with name_lower label to verify if VIP needs to be requested from Spec
			network, err := ospdirectorv1beta1.GetOpenStackNetWithLabel(
				c,
				instance.Namespace,
				labelSelector,
			)
			if err != nil {
				if k8s_errors.IsNotFound(err) {
					return uniqNetworksList, fmt.Errorf("OpenStackNet with NameLower %s not found", netNameLower)
				}
				// Error reading the object - requeue the request.
				return uniqNetworksList, fmt.Errorf("error getting OSNet with labelSelector %v", labelSelector)
			}

			if _, value := networkList[netNameLower]; !value && network.Spec.VIP {
				networkList[netNameLower] = true
				uniqNetworksList = append(uniqNetworksList, netNameLower)
			}
		}
	}

	return uniqNetworksList, nil
}
