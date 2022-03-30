package v1beta1

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

//
// GetControlPlane - Get OSP ControlPlane CR where e.g. the status information has the
// OSP version: controlPlane.Status.OSPVersion
// FIXME: We assume there is only one ControlPlane CR for now (enforced by webhook), but this might need to change
//
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
