OSP Director Operator Tech Preview
==================================

Overview
--------
The OSP Director Operator is an OpenShift Container Platform (OCP) “Operator” that streamlines the installation of OpenStack on OpenShift. The OSP Director Operator creates a set of Custom Resource Definitions (CRDs) on top of OpenShift to manage resources normally created by the TripleO Undercloud. These CRDs are split into two types for hardware provisioning and software configuration which aligns well with TripleO's Undercloud Lite architecture.

These CRDs allow administrators to provision and scale out virtual and baremetal hardware for an OSP deployment. Virtual machines for the OSP control plane are provisioned via Kubevirt(OpenShift Virtualization). Baremetal for OSP compute hosts is provisioned via Metal3. And OvercloudNet provides a simple IPAM solution for IPv4 and IPv6 static IP assignments.

Supported Features
------------------

The following table highlights features supported today in OSP 16.2 compared with future planned feature work for OSP 17 (GA).

|                                    | OSP 16.2 (Tech Preview) | OSP 17 (GA)
| ---------------------------------- | ----------------------- | ----------- |
| Virtualized Controlplane (kubevirt)| Yes                     | Yes         |
| [Baremetal Compute](https://github.com/openstack-k8s-operators/osp-director-operator/blob/master/docs/README-baremetal-provisioning.md) (metal3)         | Yes                     | Yes         |
| Network Isolation                  | Yes                     | Yes         |
| Custom Roles                       | Yes                     | Yes         |
| Playbook Generator (scale)         | Yes                     | Yes         |
| Playbook Runner workflow           | No                      | Yes         |
| DCN/Cells/Multiple Stacks          | No                      | Yes         |
| IPv4                               | Yes                     | Yes         |
| IPv6                               | No (untested)           | Yes         |
| Spine Leaf networking              | No                      | Yes         |
| TLS-Everywhere                     | No                      | Yes         |
| Scale Testing                      | No                      | Yes         |
| NFV features                       | No                      | Yes         |
| Overcloud HA (fencing agent)       | No                      | Yes         |
