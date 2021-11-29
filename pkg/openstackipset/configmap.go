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

package openstackipset

import (
	"context"
	"fmt"
	"net"

	"strconv"
	"strings"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
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
	NetType         string // ipv4/v6?
}

// information to build NodePortMap entry:
//   IPaddr:          192.168.24.9
//   IPAddrSubnet :   192.168.24.9/24
//   IPAddrURI:       192.168.24.9
//   IPv6addr:        2001:DB8:24::9
//   IPv6AddrSubnet : 2001:DB8:24::9/64
//   IPv6AddrURI:     [2001:DB8:24::9]
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
	OVNStaticBridgeMappings map[string]string
}

// RoleType - details of the tripleo role
type RoleType struct {
	Name          string
	NameLower     string
	Networks      map[string]*roleNetworkType
	Nodes         map[string]*roleNodeType
	IsVMType      bool
	IsTripleoRole bool
}

func getCidrParts(cidr string) (string, int, error) {
	cidrPieces := strings.Split(cidr, "/")
	cidrSuffix, err := strconv.Atoi(cidrPieces[len(cidrPieces)-1])
	if err != nil {
		return "", cidrSuffix, err
	}

	return cidrPieces[0], cidrSuffix, nil
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
	Name          string     // ooo network name
	NameLower     string     // ooo network name lower
	VIP           bool       // allocate VIP on network, defaut true
	MTU           int        // default 1500
	DefaultSubnet subnetType //only required/used for Train/16.2
	Subnets       map[string]subnetType
}

// CreateConfigMapParams - creates a map of parameters for the overcloud ipset config map
func CreateConfigMapParams(r common.ReconcilerCommon, instance ospdirectorv1beta1.OpenStackIPSet, osNetList ospdirectorv1beta1.OpenStackNetList, osMACList ospdirectorv1beta1.OpenStackMACAddressList) (map[string]interface{}, map[string]*RoleType, error) {

	templateParameters := make(map[string]interface{})

	// DeployedServerPortMap:
	// https://docs.openstack.org/project-deploy-guide/tripleo-docs/latest/features/custom_networks.html
	// https://docs.openstack.org/project-deploy-guide/tripleo-docs/latest/features/deployed_server.html
	// https://specs.openstack.org/openstack/tripleo-specs/specs/wallaby/triplo-network-data-v2-node-ports.html

	// map with details for all networks
	networksMap := map[string]*networkType{}
	rolesMap := map[string]*RoleType{}

	//
	// get OSPVersion from ControlPlane CR
	//
	controlPlane, _, err := common.GetControlPlane(r, &instance.ObjectMeta)
	if err != nil {
		return templateParameters, rolesMap, err
	}
	OSPVersion, err := ospdirectorv1beta1.GetOSPVersion(string(controlPlane.Status.OSPVersion))
	if err != nil {
		return templateParameters, rolesMap, err
	}

	// map of subnet -> network_lower name used when creating the rolesMap
	// to get the network name from the subnet name
	networkMappingList := map[string]string{}

	//
	// create networksMap map from OpenStackNetConfig
	//
	// TODO add label to osNetCFG that we can filter the one corresponding to the ipset
	netConfigList := &ospdirectorv1beta1.OpenStackNetConfigList{}

	labelSelector := map[string]string{}

	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.MatchingLabels(labelSelector),
	}

	if err := r.GetClient().List(context.Background(), netConfigList, listOpts...); err != nil {
		return templateParameters, rolesMap, err
	}

	//
	// Create networksMap
	//
	if len(netConfigList.Items) > 0 {
		// for now expect there is only one osnetcfg object
		netcfg := netConfigList.Items[0]

		//
		// create map of all network,  used to create network_data.yaml
		//
		for _, n := range netcfg.Spec.Networks {
			network := &networkType{}

			//
			// global ooo network parameters
			//
			network.Name = n.Name
			network.NameLower = n.NameLower
			network.VIP = n.VIP
			network.MTU = n.MTU
			network.Subnets = map[string]subnetType{}

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
					ip, cidrSuffix, err := getCidrParts(s.IPv4.Cidr)
					if err != nil {
						return templateParameters, rolesMap, err
					}

					if common.IsIPv4(net.ParseIP(ip)) {
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
					ip, cidrSuffix, err := getCidrParts(s.IPv6.Cidr)
					if err != nil {
						return templateParameters, rolesMap, err
					}

					if common.IsIPv6(net.ParseIP(ip)) {
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
				if OSPVersion == ospdirectorv1beta1.OSPVersion(ospdirectorv1beta1.TemplateVersion16_2) &&
					n.NameLower == s.Name {
					network.DefaultSubnet = subnet
				} else {
					network.Subnets[s.Name] = subnet
				}
			}
			networksMap[n.NameLower] = network
		}
	}

	//
	// Create rolesMap
	//
	for _, osnet := range osNetList.Items {
		for roleName, roleReservation := range osnet.Status.RoleReservations {
			//
			// check if role is VM
			//
			isVMType, isTripleoRole, err := isVMRole(r, strings.ToLower(roleName), instance.Namespace)
			if err != nil {
				return templateParameters, rolesMap, err
			}

			if !roleReservation.AddToPredictableIPs {
				continue
			}

			//
			// create map of all roles with Name and Count
			//
			if rolesMap[roleName] == nil {
				rolesMap[roleName] = &RoleType{
					Name:          roleName,
					NameLower:     strings.ToLower(roleName),
					Networks:      map[string]*roleNetworkType{},
					Nodes:         map[string]*roleNodeType{},
					IsVMType:      isVMType,
					IsTripleoRole: isTripleoRole,
				}
			}

			//
			// add network details to role
			//
			netAddr, cidrSuffix, err := getCidrParts(osnet.Spec.Cidr)
			if err != nil {
				return templateParameters, rolesMap, err
			}

			// TODO ipv4/ipv6 dual network
			netType := "ipv4"
			if common.IsIPv6(net.ParseIP(osnet.Spec.Cidr)) {
				netType = "ipv6"
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
				NetType:         netType,
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

	templateParameters["RolesMap"] = rolesMap
	templateParameters["NetworksMap"] = networksMap

	return templateParameters, rolesMap, nil

}

//
// isVMRole - check if role is VMset and tripleo role
//
func isVMRole(r common.ReconcilerCommon, roleName string, namespace string) (bool, bool, error) {

	vmset := &ospdirectorv1beta1.OpenStackVMSet{}

	err := r.GetClient().Get(context.TODO(), types.NamespacedName{Name: roleName, Namespace: namespace}, vmset)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return false, false, err
	}

	if vmset != nil {
		return true, vmset.Spec.IsTripleoRole, nil
	}

	return false, false, nil
}
