OSP Director Operator Baremetal Provisioning
============================================

Overview
--------
OSP Director Operator utilizes OpenShift's (OCP) [Metal3 operator](https://github.com/metal3-io/baremetal-operator) to provide baremetal
compute provisioning functionality.  We rely on a PXE provisioning network to be configured
for Metal3, which is included by default with an IPI (Installer Provisioned Infrastructure)
OpenShift cluster deployment.  Another popular deployment approach, AI (Assisted Installer),
does not have this PXE network available by default, and must be manually added to the
cluster after installation.

Requirements
------------

__*NOTE:__ As mentioned above in the overview, AI clusters do not come with a PXE provisioning
network by default.  Therefore, for proper baremetal compute provisioning support for AI
clusters, there are the following further prerequisites:

- A layer-2 network must be available to all OCP masters and workers, as well as to the baremetal compute hosts you intend to provision for your OSP cloud (the same network for all hosts)
    - The interface name must be consistent across the masters
    - The baremetal compute nodes must have their BIOS pre-configured to PXE boot on the interface connected to the provisioning network
        - Ensure that the hard disk is boot option #1 with PXE being boot option #2
- The OCP baremetal cluster operator must be manually informed of the aforementioned network’s existence
    - This requires the creation of a `provisionings.metal3.io` CR to specify the network details.  For example:
        ```
        apiVersion: metal3.io/v1alpha1
        kind: Provisioning
        metadata:
          name: provisioning-configuration
        spec:
          provisioningDHCPRange: 172.22.0.10,172.22.0.254
          provisioningIP: 172.22.0.3
          provisioningInterface: enp2s0  # this is the provisioning interface on the master nodes
          provisioningNetwork: Managed
          provisioningNetworkCIDR: 172.22.0.0/24
        ```
- We provide our own `OpenStackProvisionServer` CRD to house the RHEL image desired by the user for baremetal compute provisioning.  If for some reason the OCP cluster's masters and workers do NOT share the same provisioning interface name (i.e. the one in the `Provisioning` CR), we provide the option to override this value.  Simply pre-create an `OpenStackProvisionServer` CR and specify which interface should be used for provisioning.  For example:
    ```
    apiVersion: osp-director.openstack.org/v1beta1
    kind: OpenStackProvisionServer
    metadata:
      name: openstack
      namespace: openstack
    spec:
      port: 6190
      baseImageUrl: http://192.168.111.1/images/rhel-guest-image-8.4-1168.x86_64.qcow2
      interface: enp3s0
    ```
    - Once this resource is available, you can designate it as the provision server for an `OpenStackBaremetalSet` like so:
        ```
        apiVersion: osp-director.openstack.org/v1beta1
        kind: OpenStackBaremetalSet
        metadata:
          name: compute
          namespace: openstack
        spec:
          ...
          provisionServerName: openstack
          ...
        ```
