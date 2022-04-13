/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package openstackconfiggenerator

import (
	"context"
	"fmt"
	"net"

	"strings"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/controlplane"
	"k8s.io/apimachinery/pkg/types"
)

type roleNetworkType struct {
	Name            string
	NameLower       string
	Cidr            string // e.g. 192.168.24.0/24
	NetAddr         string // e.g. 192.168.24.0
	CidrSuffix      int    // e.g. 24
	MTU             int    // default 1500
	AllocationStart string
	AllocationEnd   string
	Gateway         string
	VIP             bool // allocate VIP on network, defaut true
	Vlan            int
	IPv6            bool // ipv4/v6?
	IsControlPlane  bool
}

// information to build NodePortMap entry:
//   IPaddr:          192.168.24.9
//   IPAddrSubnet :   192.168.24.9/24
//   IPAddrURI:       192.168.24.9
//   IPv6addr:        2001:DB8:24::9
//   IPv6AddrSubnet : 2001:DB8:24::9/64
//   IPv6AddrURI:     [2001:DB8:24::9]
//   Network:         flattened network details
type roleIPType struct {
	IPaddr         string
	IPAddrURI      string
	IPAddrSubnet   string
	IPv6addr       string
	IPv6AddrURI    string
	IPv6AddrSubnet string
	Network        *roleNetworkType
}

type roleNodeType struct {
	Index                   int
	IPaddr                  map[string]*roleIPType
	Hostname                string
	VIP                     bool
	ServiceVIP              bool
	OVNStaticBridgeMappings map[string]string
}

// RoleType - details of the tripleo role
type RoleType struct {
	Name           string
	NameLower      string
	Networks       map[string]*roleNetworkType
	Nodes          map[string]*roleNodeType
	IsVMType       bool
	IsTripleoRole  bool
	IsControlPlane bool
}

type netDetailsType struct {
	Cidr            string // e.g. 192.168.24.0/24
	CidrSuffix      int    // e.g. 24
	AllocationStart string
	AllocationEnd   string
	Gateway         string
	Routes          []ospdirectorv1beta1.Route
}

type subnetType struct {
	Name string // ooo subnet name
	IPv4 netDetailsType
	IPv6 netDetailsType
	Vlan int
}

type networkType struct {
	Name           string     // ooo network name
	NameLower      string     // ooo network name lower
	VIP            bool       // allocate VIP on network, defaut true
	MTU            int        // default 1500
	DefaultSubnet  subnetType //only required/used for Train/16.2
	Subnets        map[string]subnetType
	IPv6           bool
	IsControlPlane bool
	DomainName     string
	DNSServers     []string
}

// CreateConfigMapParams - creates a map of parameters to render the required overcloud parameter files
func CreateConfigMapParams(
	ctx context.Context,
	r common.ReconcilerCommon,
	instance *ospdirectorv1beta1.OpenStackConfigGenerator,
	cond *shared.Condition,
) (map[string]interface{}, map[string]*RoleType, error) {

	templateParameters := make(map[string]interface{})
	rolesMap := map[string]*RoleType{}

	//
	// get list of all osNets
	//
	osNetList := &ospdirectorv1beta1.OpenStackNetList{}
	osNetListOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.Limit(1000),
	}
	err := r.GetClient().List(ctx, osNetList, osNetListOpts...)
	if err != nil {
		cond.Message = fmt.Sprintf("%s %s failed to get list of all OSNets", instance.Kind, instance.Name)
		cond.Reason = shared.NetCondReasonNetNotFound
		cond.Type = shared.ConfigGeneratorCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return templateParameters, rolesMap, err
	}

	//
	// get list of all osMACs
	//
	osMACList := &ospdirectorv1beta1.OpenStackMACAddressList{}
	osMACListOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.Limit(1000),
	}
	err = r.GetClient().List(ctx, osMACList, osMACListOpts...)
	if err != nil {
		cond.Message = fmt.Sprintf("%s %s failed to get list of all OSMACs", instance.Kind, instance.Name)
		cond.Reason = shared.MACCondReasonMACNotFound
		cond.Type = shared.ConfigGeneratorCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return templateParameters, rolesMap, err
	}

	//
	// get OSPVersion from ControlPlane CR
	//
	controlPlane, _, err := ospdirectorv1beta1.GetControlPlane(r.GetClient(), &instance.ObjectMeta)
	if err != nil {
		return templateParameters, rolesMap, err
	}
	OSPVersion, err := shared.GetOSPVersion(string(controlPlane.Status.OSPVersion))
	if err != nil {
		return templateParameters, rolesMap, err
	}

	//
	// create networksMap map from OpenStackNetConfig
	//
	netConfigList := &ospdirectorv1beta1.OpenStackNetConfigList{}
	labelSelector := map[string]string{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.MatchingLabels(labelSelector),
	}
	if err := r.GetClient().List(ctx, netConfigList, listOpts...); err != nil {
		return templateParameters, rolesMap, err
	}

	netcfg := ospdirectorv1beta1.OpenStackNetConfig{}
	if len(netConfigList.Items) > 0 {
		// for now expect there is only one osnetcfg object
		netcfg = netConfigList.Items[0]
	}

	//
	// Create networksMap, networkMappingList
	//
	// networksMap: all networks except ctlplane
	// networkMappingList: map of subnet -> network_lower name used when creating the rolesMap
	//                     to get the network name from the subnet name

	networksMap, networkMappingList, err := createNetworksMap(
		OSPVersion,
		&netcfg,
	)

	if err != nil {
		return templateParameters, rolesMap, err
	}

	//
	// Create rolesMap
	//
	err = createRolesMap(
		ctx,
		r,
		instance,
		osNetList,
		osMACList,
		networksMap,
		networkMappingList,
		rolesMap,
	)
	if err != nil {
		return templateParameters, rolesMap, err
	}

	templateParameters["RolesMap"] = rolesMap
	templateParameters["NetworksMap"] = networksMap

	return templateParameters, rolesMap, nil

}

//
// createNetworksMap - create map with network details and map of subnet -> network_lower name used when creating the rolesMap
//	               to get the network name from the subnet name
//
func createNetworksMap(
	ospVersion shared.OSPVersion,
	netConfig *ospdirectorv1beta1.OpenStackNetConfig,
) (
	map[string]*networkType,
	map[string]string,
	error,
) {
	networksMap := map[string]*networkType{}
	networkMappingList := map[string]string{}

	//
	// Create networksMap, all networks except ctlplane  used to create network_data.yaml
	//
	for _, n := range netConfig.Spec.Networks {
		network := &networkType{}

		//
		// global ooo network parameters
		//
		network.Name = n.Name
		network.NameLower = n.NameLower
		network.VIP = n.VIP
		network.MTU = n.MTU
		network.Subnets = map[string]subnetType{}
		network.IsControlPlane = n.IsControlPlane

		if network.IsControlPlane {
			network.DomainName = fmt.Sprintf("%s.%s", ospdirectorv1beta1.ControlPlaneNameLower, netConfig.Spec.DomainName)
		} else {
			network.DomainName = fmt.Sprintf("%s.%s", strings.ToLower(n.Name), netConfig.Spec.DomainName)
		}

		network.DNSServers = netConfig.Spec.DNSServers

		//
		// per subnet parameters
		//
		for _, s := range n.Subnets {

			// add mapping reference to networkMappingList
			networkMappingList[s.Name] = network.NameLower

			//
			// IPv4 subnet details
			//
			subnetDetailsV4 := netDetailsType{}
			if s.IPv4.Cidr != "" {
				var err error
				ip, cidrSuffix, err := common.GetCidrParts(s.IPv4.Cidr)
				if err != nil {
					return networksMap, networkMappingList, err
				}

				if common.IsIPv4(net.ParseIP(ip)) {
					network.IPv6 = false
					subnetDetailsV4 = netDetailsType{
						AllocationEnd:   s.IPv4.AllocationEnd,
						AllocationStart: s.IPv4.AllocationStart,
						Cidr:            s.IPv4.Cidr,
						CidrSuffix:      cidrSuffix,
						Gateway:         s.IPv4.Gateway,
						Routes:          s.IPv4.Routes,
					}
				}
			}

			//
			// IPv6 subnet details
			//
			subnetDetailsV6 := netDetailsType{}
			if s.IPv6.Cidr != "" {
				var err error
				ip, cidrSuffix, err := common.GetCidrParts(s.IPv6.Cidr)
				if err != nil {
					return networksMap, networkMappingList, err
				}

				if common.IsIPv6(net.ParseIP(ip)) {
					network.IPv6 = true
					subnetDetailsV6 = netDetailsType{
						AllocationEnd:   s.IPv6.AllocationEnd,
						AllocationStart: s.IPv6.AllocationStart,
						Cidr:            s.IPv6.Cidr,
						CidrSuffix:      cidrSuffix,
						Gateway:         s.IPv6.Gateway,
						Routes:          s.IPv6.Routes,
					}
				}
			}

			subnet := subnetType{
				Name: s.Name,
				Vlan: s.Vlan,
				IPv4: subnetDetailsV4,
				IPv6: subnetDetailsV6,
			}

			//
			// In train, there is a top level default subnet, while with networkv2 there are only subnets.
			// For default subnet in Train, the network NameLower and subnet Name must match
			//
			if ospVersion == shared.OSPVersion(shared.TemplateVersion16_2) &&
				n.NameLower == s.Name {
				network.DefaultSubnet = subnet
			} else {
				network.Subnets[s.Name] = subnet
			}
		}
		networksMap[n.NameLower] = network
	}

	return networksMap, networkMappingList, nil
}

// IsRoleIncluded - checks if the role exists in the ConfigGenerator Roles set
func IsRoleIncluded(roleName string, instance *ospdirectorv1beta1.OpenStackConfigGenerator) bool {

	if len(instance.Spec.Roles) == 0 || roleName == controlplane.Role {
		return true
	}
	for _, r := range instance.Spec.Roles {
		if roleName == r {
			return true
		}
	}
	return false

}

//
// createRolesMap - create map with all roles
//
func createRolesMap(
	ctx context.Context,
	r common.ReconcilerCommon,
	instance *ospdirectorv1beta1.OpenStackConfigGenerator,
	osNetList *ospdirectorv1beta1.OpenStackNetList,
	osMACList *ospdirectorv1beta1.OpenStackMACAddressList,
	networksMap map[string]*networkType,
	networkMappingList map[string]string,
	rolesMap map[string]*RoleType,
) error {

	for _, osnet := range osNetList.Items {
		for roleName, roleReservation := range osnet.Spec.RoleReservations {
			if IsRoleIncluded(roleName, instance) {
				//
				// check if role is VM
				//
				isVMType, isTripleoRole, err := isVMRole(ctx, r, strings.ToLower(roleName), instance.Namespace)
				if err != nil {
					return err
				}

				if !roleReservation.AddToPredictableIPs {
					continue
				}

				//
				// create map of all roles with Name and Count
				//
				// the role is ControlPlane if its either vip or serviceVIP
				isControlPlane := false
				for _, res := range roleReservation.Reservations {
					if res.VIP || res.ServiceVIP {
						isControlPlane = true
					}
				}

				if rolesMap[roleName] == nil {
					rolesMap[roleName] = &RoleType{
						Name:           roleName,
						NameLower:      strings.ToLower(roleName),
						Networks:       map[string]*roleNetworkType{},
						Nodes:          map[string]*roleNodeType{},
						IsVMType:       isVMType,
						IsTripleoRole:  isTripleoRole,
						IsControlPlane: isControlPlane,
					}
				}

				//
				// add network details to role
				//
				netAddr, cidrSuffix, err := common.GetCidrParts(osnet.Spec.Cidr)
				if err != nil {
					return err
				}

				isIPv6 := false
				if common.IsIPv6(net.ParseIP(osnet.Spec.Cidr)) {
					isIPv6 = true
				}

				nameLower := networkMappingList[osnet.Spec.NameLower]

				rolesMap[roleName].Networks[osnet.Spec.NameLower] = &roleNetworkType{
					Name:            osnet.Spec.Name,
					NameLower:       nameLower,
					Cidr:            osnet.Spec.Cidr,
					CidrSuffix:      cidrSuffix,
					NetAddr:         netAddr,
					MTU:             osnet.Spec.MTU,
					Vlan:            osnet.Spec.Vlan,
					AllocationStart: osnet.Spec.AllocationStart,
					AllocationEnd:   osnet.Spec.AllocationEnd,
					Gateway:         osnet.Spec.Gateway,
					VIP:             osnet.Spec.VIP,
					IPv6:            isIPv6,
					IsControlPlane:  networksMap[nameLower].IsControlPlane,
				}

				hostnameMapIndex := 0
				for _, reservation := range roleReservation.Reservations {

					//
					// get OVNStaticBridgeMacMappings information from overcloudMACList
					//
					ovnStaticBridgeMappings := map[string]string{}
					for _, macReservations := range osMACList.Items {
						for node, macReservation := range macReservations.Status.MACReservations {
							if node == reservation.Hostname {
								for net, mac := range macReservation.Reservations {
									ovnStaticBridgeMappings[net] = mac
								}
							}
						}
					}

					//
					// update rolesMap with reservations
					//
					if !reservation.Deleted {
						if rolesMap[roleName].Nodes[reservation.Hostname] == nil {
							rolesMap[roleName].Nodes[reservation.Hostname] = &roleNodeType{
								Index:                   hostnameMapIndex,
								IPaddr:                  map[string]*roleIPType{},
								Hostname:                reservation.Hostname,
								VIP:                     reservation.VIP,
								ServiceVIP:              reservation.ServiceVIP,
								OVNStaticBridgeMappings: ovnStaticBridgeMappings,
							}
						}

						uri := reservation.IP
						if common.IsIPv6(net.ParseIP(reservation.IP)) {
							// IP address with brackets in case of IPv6, e.g. [2001:DB8:24::15]
							uri = fmt.Sprintf("[%s]", uri)
						}
						if rolesMap[roleName].Nodes[reservation.Hostname].IPaddr[osnet.Spec.NameLower] == nil {
							rolesMap[roleName].Nodes[reservation.Hostname].IPaddr[osnet.Spec.NameLower] = &roleIPType{
								IPaddr:       reservation.IP,
								IPAddrURI:    uri,
								IPAddrSubnet: fmt.Sprintf("%s/%d", reservation.IP, cidrSuffix),
								Network:      rolesMap[roleName].Networks[osnet.Spec.NameLower],
							}
						}

						hostnameMapIndex++
					}
				}
			}
		}
	}

	return nil
}

//
// isVMRole - check if role is VMset and tripleo role
//
func isVMRole(
	ctx context.Context,
	r common.ReconcilerCommon,
	roleName string,
	namespace string,
) (bool, bool, error) {

	vmset := &ospdirectorv1beta1.OpenStackVMSet{}

	err := r.GetClient().Get(ctx, types.NamespacedName{Name: roleName, Namespace: namespace}, vmset)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return false, false, err
	}

	if vmset != nil {
		return true, vmset.Spec.IsTripleoRole, nil
	}

	return false, false, nil
}
