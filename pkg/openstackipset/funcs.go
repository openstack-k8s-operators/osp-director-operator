package openstackipset

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	controlplane "github.com/openstack-k8s-operators/osp-director-operator/pkg/controlplane"
	openstacknetconfig "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknetconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// EnsureIPs - Creates IPSet and verify/wait for IPs created
func EnsureIPs(
	ctx context.Context,
	r common.ReconcilerCommon,
	obj client.Object,
	cond *shared.Condition,
	name string,
	networks []string,
	hostCount int,
	vip bool,
	serviceVIP bool,
	deletedHosts []string,
	addToPredictableIPs bool,
) (map[string]ospdirectorv1beta1.IPStatus, reconcile.Result, error) {

	status := map[string]ospdirectorv1beta1.IPStatus{}

	//
	// create IPSet to request IPs for all networks
	//
	_, err := createOrUpdateIPSet(
		ctx,
		r,
		obj,
		cond,
		name,
		networks,
		hostCount,
		vip,
		serviceVIP,
		deletedHosts,
		addToPredictableIPs,
	)
	if err != nil {
		return status, reconcile.Result{}, err
	}

	//
	// get ipset and verify all IPs got created
	//
	ipSet := &ospdirectorv1beta1.OpenStackIPSet{}
	err = r.GetClient().Get(ctx, types.NamespacedName{
		Name:      strings.ToLower(name),
		Namespace: obj.GetNamespace()},
		ipSet)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to get %s %s ", ipSet.Kind, strings.ToLower(name))
		cond.Reason = shared.IPSetCondReasonError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, obj, err)

		return status, reconcile.Result{}, err
	}

	//
	// check for all hosts got created on the IPset
	//
	if len(ipSet.Status.Hosts) < hostCount {
		cond.Message = fmt.Sprintf("Waiting on hosts to be created on IPSet %v - %v", len(ipSet.Status.Hosts), hostCount)
		cond.Reason = shared.IPSetCondReasonWaitingOnHosts
		cond.Type = shared.CommonCondTypeWaiting

		return status, reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	//
	// check if hosts got deleted
	//
	if len(ipSet.Status.Hosts) > hostCount &&
		(len(ipSet.Status.Hosts)-len(deletedHosts)) == hostCount {
		for _, hostname := range deletedHosts {
			delete(ipSet.Status.Hosts, hostname)
		}

	}

	//
	// get OSNetCfg object
	//
	osNetCfg, err := ospdirectorv1beta1.GetOsNetCfg(r.GetClient(), obj.GetNamespace(), obj.GetLabels()[shared.OpenStackNetConfigReconcileLabel])
	if err != nil {
		cond.Type = shared.CommonCondTypeError
		cond.Reason = shared.NetConfigCondReasonError
		cond.Message = fmt.Sprintf("error getting OpenStackNetConfig %s: %s",
			obj.GetLabels()[shared.OpenStackNetConfigReconcileLabel],
			err)

		return status, reconcile.Result{}, err
	}

	for hostname, hostStatus := range ipSet.Status.Hosts {
		err = openstacknetconfig.WaitOnIPsCreated(
			r,
			obj,
			cond,
			osNetCfg,
			networks,
			hostname,
			&hostStatus,
		)
		if err != nil {
			return status, reconcile.Result{RequeueAfter: 10 * time.Second}, nil
		}

		status[hostname] = hostStatus
	}

	return status, reconcile.Result{}, nil
}

// createOrUpdateIPSet - Creates or updates IPSet
func createOrUpdateIPSet(
	ctx context.Context,
	r common.ReconcilerCommon,
	obj client.Object,
	cond *shared.Condition,
	name string,
	networks []string,
	hostCount int,
	vip bool,
	serviceVIP bool,
	deletedHosts []string,
	addToPredictableIPs bool,
) (*ospdirectorv1beta1.OpenStackIPSet, error) {
	ipSet := &ospdirectorv1beta1.OpenStackIPSet{
		ObjectMeta: metav1.ObjectMeta{
			// use the role name as the VM CR name
			Name:      strings.ToLower(name),
			Namespace: obj.GetNamespace(),
		},
	}

	op, err := controllerutil.CreateOrPatch(ctx, r.GetClient(), ipSet, func() error {
		ipSet.Labels = shared.MergeStringMaps(
			ipSet.Labels,
			common.GetLabels(obj, controlplane.AppLabel, map[string]string{}),
		)

		ipSet.Spec.HostCount = hostCount
		ipSet.Spec.Networks = networks
		ipSet.Spec.RoleName = name
		ipSet.Spec.VIP = vip
		ipSet.Spec.ServiceVIP = serviceVIP
		ipSet.Spec.DeletedHosts = deletedHosts
		ipSet.Spec.AddToPredictableIPs = addToPredictableIPs

		// TODO: (mschupppert) move to webhook
		if len(networks) == 0 {
			ipSet.Spec.Networks = []string{"ctlplane"}
		}

		err := controllerutil.SetControllerReference(obj, ipSet, r.GetScheme())
		if err != nil {
			cond.Message = fmt.Sprintf("Error set controller reference for %s %s", ipSet.Kind, ipSet.Name)
			cond.Reason = shared.CommonCondReasonControllerReferenceError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, obj, err)

			return err
		}

		return nil
	})
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update %s %s ", ipSet.Kind, ipSet.Name)
		cond.Reason = shared.IPSetCondReasonError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, obj, err)

		return ipSet, err
	}

	cond.Message = fmt.Sprintf("%s %s successfully reconciled", ipSet.Kind, ipSet.Name)
	if op != controllerutil.OperationResultNone {
		cond.Message = fmt.Sprintf("%s - operation: %s", cond.Message, string(op))

		common.LogForObject(
			r,
			cond.Message,
			obj,
		)
	}
	cond.Reason = shared.IPSetCondReasonCreated
	cond.Type = shared.CommonCondTypeCreated

	return ipSet, nil
}
