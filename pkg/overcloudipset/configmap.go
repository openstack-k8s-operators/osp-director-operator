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

package overcloudipset

import (
	"fmt"
	"strconv"
	"strings"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	//	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
)

type network struct {
	Cidr   string
	IPaddr string
}

type deployedServerPortMapType struct {
	Network  map[string]*network
	Hostname string
	VIP      bool
}

// CreateConfigMapParams - creates a map of parameters for the overcloud ipset config map
func CreateConfigMapParams(overcloudIPList ospdirectorv1beta1.OvercloudIPSetList, overcloudNetList ospdirectorv1beta1.OvercloudNetList) (map[string]interface{}, error) {

	// network isolation w/ predictable IPs:
	//	   https://docs.openstack.org/project-deploy-guide/tripleo-docs/latest/features/network_isolation.html
	//     https://docs.openstack.org/project-deploy-guide/tripleo-docs/latest/provisioning/node_placement.html#predictable-ips
	roleIPSets := map[string]map[string][]string{}
	// DeployedServerPortMap:
	//     https://docs.openstack.org/project-deploy-guide/tripleo-docs/latest/features/deployed_server.html
	ctlPlaneIps := map[string]*deployedServerPortMapType{}

	//hostnameMap := make(map[string]string)
	hostnameFormat := make(map[string]string)
	roleCount := make(map[string]string)
	rolePortsFromPool := make(map[string]map[string]string)

	for _, net := range overcloudNetList.Items {

		for _, reservation := range net.Status.Reservations {

			// if ctlplane network add to DeployedServerPortMap
			if net.Name == "ctlplane" {
				netNameLower := GetNetNameLower("ctlplane")
				// create ctlPlaneIps for the host if it does not exist
				if ctlPlaneIps[reservation.Hostname] == nil {
					ctlPlaneIps[reservation.Hostname] = &deployedServerPortMapType{
						Hostname: reservation.Hostname,
						Network:  map[string]*network{},
						VIP:      reservation.VIP,
					}
				}

				if reservation.VIP {
					if ctlPlaneIps[reservation.Hostname].Network[netNameLower] == nil {
						ctlPlaneIps[reservation.Hostname].Network[netNameLower] = &network{
							IPaddr: reservation.IP,
							Cidr:   net.Spec.Cidr,
						}
					} else {
						ctlPlaneIps[reservation.Hostname].Network[netNameLower].IPaddr = reservation.IP
						ctlPlaneIps[reservation.Hostname].Network[netNameLower].Cidr = net.Spec.Cidr
					}
				} else if reservation.AddToPredictableIPs {

					if ctlPlaneIps[reservation.Hostname].Network[net.Name] == nil {
						ctlPlaneIps[reservation.Hostname].Network[net.Name] = &network{
							IPaddr: reservation.IP,
							Cidr:   net.Spec.Cidr,
						}
					} else {
						ctlPlaneIps[reservation.Hostname].Network[net.Name].IPaddr = reservation.IP
						ctlPlaneIps[reservation.Hostname].Network[net.Name].Cidr = net.Spec.Cidr
					}
				}
			} else if reservation.AddToPredictableIPs {
				// Add host to hostnamemap
				//hostnameMap[fmt.Sprintf("overcloud-%s", reservation.IDKey)] = reservation.Hostname

				// if not ctlplane network entry, add to Predictible IPs entry
				if roleIPSet, exists := roleIPSets[reservation.Role]; exists {
					roleIPSet[net.Name] = append(roleIPSet[net.Name], reservation.IP)
				} else {
					roleIPSets[reservation.Role] = map[string][]string{net.Name: {reservation.IP}}
				}
			}
		}
	}

	// create role specific parameters from ipsets
	// ipsets where overcloudipset.AddToPredictableIPsLabel: "true" will be skipped (openstackclient, controlplane)
	for _, ipset := range overcloudIPList.Items {
		// set <Role>HostnameFormat
		hostnameFormat[fmt.Sprintf("%sHostnameFormat", ipset.Spec.Role)] = fmt.Sprintf("%s-%%index%%", strings.ToLower(ipset.Spec.Role))
		// set <Role>Count
		roleCount[fmt.Sprintf("%sCount", ipset.Spec.Role)] = strconv.Itoa(ipset.Spec.HostCount)
		// create PortsFromPool map for network isolation to set Ports resource_registry entries
		for _, net := range ipset.Spec.Networks {
			if net == "ctlplane" {
				continue
			}
			portConfig := fmt.Sprintf("/usr/share/openstack-tripleo-heat-templates/network/ports/%s_from_pool.yaml", net)
			if rolePortsFromPool[ipset.Spec.Role][GetNetName(net)] == "" {
				rolePortsFromPool[ipset.Spec.Role] = map[string]string{GetNetName(net): portConfig}
			} else {
				rolePortsFromPool[ipset.Spec.Role][GetNetName(net)] = portConfig
			}
		}

	}

	templateParameters := make(map[string]interface{})
	// TODO: make sure the list order won't change
	templateParameters["PredictableIps"] = roleIPSets
	templateParameters["CtlPlaneIps"] = ctlPlaneIps
	templateParameters["RoleCount"] = roleCount
	templateParameters["HostnameFormat"] = hostnameFormat
	templateParameters["RolePortsFromPool"] = rolePortsFromPool

	return templateParameters, nil

}

// GetNetNameLower -
func GetNetNameLower(net string) string {
	return strings.ToLower(networkDict()(net))
}

// GetNetName -
func GetNetName(net string) string {
	return networkDict()(net)
}

func networkDict() func(string) string {
	// innerMap is captured in the closure returned below
	innerMap := map[string]string{
		"ctlplane":     "Control",
		"internal_api": "InternalApi",
		"external":     "External",
		"storage":      "Storage",
		"storage_mgmt": "StorageMgmt",
		"tenant":       "Tenant",
		"management":   "Management",
	}

	return func(key string) string {
		return innerMap[key]
	}
}
