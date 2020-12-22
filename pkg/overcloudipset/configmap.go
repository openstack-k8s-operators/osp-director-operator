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
	"bytes"
	"fmt"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
)

// CreateConfigMapParams - creates a map of parameters for the overcloud ipset config map
func CreateConfigMapParams(overcloudIPList ospdirectorv1beta1.OvercloudIPSetList, ctlplaneCidr string) map[string]string {

	// network isolation w/ predictable IPs:
	//	   https://docs.openstack.org/project-deploy-guide/tripleo-docs/latest/features/network_isolation.html
	//     https://docs.openstack.org/project-deploy-guide/tripleo-docs/latest/provisioning/node_placement.html#predictable-ips
	roleIPSets := map[string]map[string][]string{}
	// DeployedServerPortMap:
	//     https://docs.openstack.org/project-deploy-guide/tripleo-docs/latest/features/deployed_server.html
	ctlPlaneIps := map[string]string{}

	for _, ipset := range overcloudIPList.Items {
		for count := 1; count <= ipset.Spec.HostCount; count++ {
			hostname := fmt.Sprintf("%s%d", ipset.Name, count)

			for netName, addr := range ipset.Status.HostIPs[hostname].IPAddresses {
				if netName == "ctlplane" {
					//ctlPlaneIps[ipset.Name] = addr
					ctlPlaneIps[hostname] = addr
				} else {
					if roleIPSet, exists := roleIPSets[ipset.Spec.Role]; exists {
						//roleIPSets[ipset.Spec.Role] = map[string][]string{netName: append(roleIPSet[netName], addr)}
						roleIPSet[netName] = append(roleIPSet[netName], addr)
					} else {
						roleIPSets[ipset.Spec.Role] = map[string][]string{netName: {addr}}
					}
				}
			}
		}
	}

	templateParameters := make(map[string]string)
	templateParameters["PredictableIps"] = roleIPSetToString(roleIPSets)
	templateParameters["DeployedServerPortMap"] = deployedServerPortMap(ctlPlaneIps, ctlplaneCidr)
	templateParameters["CtlplaneCidr"] = ctlplaneCidr
	return templateParameters

}

// roleIPSetToString -
func roleIPSetToString(r map[string]map[string][]string) string {
	var b bytes.Buffer
	for role, roleIps := range r {
		b.WriteString(fmt.Sprintf("  %sIPs:\n", role))
		for netname, ips := range roleIps {
			b.WriteString(fmt.Sprintf("    %s:\n", netname))
			for _, ip := range ips {
				b.WriteString(fmt.Sprintf("    - %s\n", ip))
			}
		}
	}
	return b.String()
}

// deployedServerPortMap -
func deployedServerPortMap(r map[string]string, c string) string {
	var b bytes.Buffer
	for hostname, ipaddr := range r {
		b.WriteString(fmt.Sprintf("    %s-ctlplane:\n", hostname))
		b.WriteString(fmt.Sprintf("      fixed_ips:\n"))
		b.WriteString(fmt.Sprintf("        - ip_address: %s\n", ipaddr))
		b.WriteString(fmt.Sprintf("      subnets:\n"))
		b.WriteString(fmt.Sprintf("        - cidr: %s\n", c))
	}
	return b.String()
}
