package openstacknetconfig

import (
	"fmt"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	v1 "k8s.io/api/apps/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// WaitOnIPsCreated - Wait for IPs created on all configured networks
//
func WaitOnIPsCreated(
	r common.ReconcilerCommon,
	obj client.Object,
	cond *shared.Condition,
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
		cond.Reason = shared.NetConfigCondReasonWaitingOnIPsForHost
		cond.Type = shared.CommonCondTypeWaiting

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
		cond.Reason = shared.NetConfigCondReasonWaitingOnIPsForHost
		cond.Type = shared.CommonCondTypeWaiting

		return k8s_errors.NewNotFound(v1.Resource(obj.GetObjectKind().GroupVersionKind().Kind), cond.Message)
	}

	return nil
}
