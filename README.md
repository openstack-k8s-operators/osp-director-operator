OSP Director Operator
=====================

[![Go Report Card](https://goreportcard.com/badge/github.com/openstack-k8s-operators/osp-director-operator)](https://goreportcard.com/report/github.com/openstack-k8s-operators/osp-director-operator)

Description
-----------


The OSP Director Operator creates a set of Custom Resource Definitions on top of OpenShift to manage resources normally created by the TripleO's Undercloud. These CRDs are split into two types for hardware provisioning and software configuration.


Hardware Provisioning CRDs
--------------------------
- openstacknetattachment: manages NodeNetworkConfigurationPolicy and NodeSriovConfigurationPolicy used to attach networks to virtual machines
- openstacknetconfig: high level CRD to specify openstacknetattachments and openstacknets to describe the full network configuration. The set of reserved IP/MAC addresses per node are reflected in the status.
- openstackbaremetalset: create sets of baremetal hosts for a specific TripleO role (Compute, Storage, etc.)
- openstackcontrolplane: A CRD used to create the OpenStack control plane and manage associated openstackvmsets
- openstacknet: Create networks which are used to assign IPs to the vmset and baremetalset resources below
- openstackipset: Contains a set of IPs for a given network and role. Used internally to manage IP addresses.
- openstackprovisionservers: used to serve custom images for baremetal provisioning with Metal3
- openstackvmset: create sets of VMs using OpenShift Virtualization for a specific TripleO role (Controller, Database, NetworkController, etc.)

Software Configuration CRDs
---------------------------
- openstackconfiggenerator: automatically generate Ansible playbooks for deployment when you scale up or make changes to custom ConfigMaps for deployment
- openstackclient: creates a pod used to run TripleO deployment commands

Installation
------------

## Prerequisite:
- OCP 4.6+ installed
- OpenShift Virtualization 2.6+
- SRIOV Operator

## Install the OSP Director Operator
The OSP Director Operator is installed and managed via the OLM [Operator Lifecycle Manager](https://github.com/operator-framework/operator-lifecycle-manager). OLM is installed automatically with your OpenShift installation. To obtain the latest OSP Director Operator snapshot you need to create the appropriate CatalogSource, OperatorGroup, and Subscription to drive the installation with OLM:

### Create the "openstack" Namespace
```bash
oc new-project openstack
```

### Create a CatalogSource (using 'openstack' namespace, and our upstream quay.io tag)
```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: osp-director-operator-index
  namespace: openstack
spec:
  sourceType: grpc
  image: quay.io/openstack-k8s-operators/osp-director-operator-index:0.0.1
```

### Create an OperatorGroup(using the 'openstack' namespace)
```yaml
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: "osp-director-operator-group"
  namespace: openstack
spec:
  targetNamespaces:
  - openstack
```

### Create a Subscription
```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: osp-director-operator-subscription
  namespace: openstack
spec:
  config:
    env:
    - name: WATCH_NAMESPACE
      value: openstack,openshift-machine-api,openshift-sriov-network-operator
  source: osp-director-operator-index
  sourceNamespace: openstack
  name: osp-director-operator
  startingCSV: osp-director-operator.v0.0.1
  channel: alpha
```

We have a script to automate the installation here with OLM for a specific tag: [script to automate the installation](https://github.com/openstack-k8s-operators/osp-director-operator/blob/master/scripts/deploy-with-olm.sh)

**NOTE**: At some point in the future we may integrate into OperatorHub so that OSP Director Operator is available automatically in your OCP installations default OLM Catalog sources.

## Creating a RHEL data volume

Create a base RHEL data volume prior to deploying OpenStack.  This will be used by the controller VMs which are provisioned via OpenShift Virtualization. The approach to doing this is as follows:

1) Install the KubeVirt CLI tool, `virtctl`:
    ```
    sudo subscription-manager repos --enable=cnv-2.6-for-rhel-8-x86_64-rpms
    sudo dnf install -y kubevirt-virtctl
    ```
2) Download the RHEL QCOW2 you wish to use.  For example:
    ```
    curl -O http://download.devel.redhat.com/brewroot/packages/rhel-guest-image/8.4/1168/images/rhel-guest-image-8.4-1168.x86_64.qcow2
    ```
    or get a RHEL8.4 image from [Installers and Images for Red Hat Enterprise Linux for x86_64 (v. 8.4 for x86_64)](https://access.redhat.com/downloads/content/479/ver=/rhel---8/8.4/x86_64/product-software)
3) If the rhel-guest-image is used, make sure to remove the net.ifnames=0 kernel parameter from the image to have the biosdev network interface naming. This can be done like:
    ```bash
    dnf install -y libguestfs-tools-c
    virt-customize -a <rhel guest image> --run-command 'sed -i -e "s/^\(kernelopts=.*\)net.ifnames=0 \(.*\)/\1\2/" /boot/grub2/grubenv'
    virt-customize -a <rhel guest image> --run-command 'sed -i -e "s/^\(GRUB_CMDLINE_LINUX=.*\)net.ifnames=0 \(.*\)/\1\2/" /etc/default/grub'
    ```
4) If your local machine cannot resolve hostnames for within the cluster, add the following to your `/etc/hosts`:
    ```
    <cluster ingress VIP>     cdi-uploadproxy-openshift-cnv.apps.<cluster name>.<domain name>
    ```
5) Upload the image to OpenShift Virtualization via `virtctl`:
    ```
    virtctl image-upload dv openstack-base-img -n openstack --size=50Gi --image-path=<local path to image> --storage-class <desired storage class> --insecure
    ```
    For the `storage-class` above, pick one you want to use from those shown in:
    ```
    oc get storageclass
    ```
## Deploying OpenStack once you have the OSP Director Operator installed

1) Define your OpenStackNetConfig custom resource. At least one network is required for the ctlplane. Optionally you may define multiple networks in the CR to be used with TripleO's network isolation architecture. In addition to the network definiition the OpenStackNet includes information that is used to define the network configuration policy used to attach any VM's to this network via OpenShift Virtualization. The following is an example of a simple IPv4 ctlplane network which uses linux bridge for its host configuration.
    ```yaml
    apiVersion: osp-director.openstack.org/v1beta1
    kind: OpenStackNetConfig
    metadata:
      name: openstacknetconfig
    spec:
      attachConfigurations:
        br-osp:
          nodeNetworkConfigurationPolicy:
            nodeSelector:
              node-role.kubernetes.io/worker: ""
            desiredState:
              interfaces:
              - bridge:
                  options:
                    stp:
                      enabled: false
                  port:
                  - name: enp7s0
                description: Linux bridge with enp7s0 as a port
                name: br-osp
                state: up
                type: linux-bridge
                mtu: 1500
      # optional DnsServers list
      dnsServers:
      - 192.168.25.1
      # optional DnsSearchDomains list
      dnsSearchDomains:
      - osptest.test.metalkube.org
      - some.other.domain
      # DomainName of the OSP environment
      domainName: osptest.test.metalkube.org
      networks:
      - name: Control
        nameLower: ctlplane
        subnets:
        - name: ctlplane
          ipv4:
            allocationEnd: 192.168.25.250
            allocationStart: 192.168.25.100
            cidr: 192.168.25.0/24
            gateway: 192.168.25.1
          attachConfiguration: br-osp
      # optional: (OSP17 only) specify all phys networks with optional MAC address prefix, used to
      # create static OVN Bridge MAC address mappings. Unique OVN bridge mac address per node is
      # dynamically allocated by creating OpenStackMACAddress resource and create a MAC per physnet per node.
      # - If PhysNetworks is not provided, the tripleo default physnet datacentre gets created.
      # - If the macPrefix is not specified for a physnet, the default macPrefix "fa:16:3a" is used.
      # - If PreserveReservations is not specified, the default is true.
      ovnBridgeMacMappings:
        preserveReservations: True
        physNetworks:
        - macPrefix: fa:16:3a
          name: datacentre
        - macPrefix: fa:16:3b
          name: datacentre2
        # optional: configure static mapping for the networks per nodes. If there is none, a random gets created
      reservations:
        controller-0:
          macReservations:
            datacentre: fa:16:3a:aa:aa:aa
            datacentre2: fa:16:3b:aa:aa:aa
        compute-0:
          macReservations:
            datacentre: fa:16:3a:bb:bb:bb
            datacentre2: fa:16:3b:bb:bb:bb
    ```

    If you write the above YAML into a file called networkconfig.yaml you can create the OpenStackNetConfig via this command:

    ```bash
    oc create -n openstack -f networkconfig.yaml
    ```

    To use network isolation using VLAN add the vlan ID to the spec of the network definition
    ```yaml
    apiVersion: osp-director.openstack.org/v1beta1
    kind: OpenStackNetConfig
    metadata:
      name: openstacknetconfig
    spec:
      attachConfigurations:
        br-osp:
          nodeNetworkConfigurationPolicy:
            nodeSelector:
              node-role.kubernetes.io/worker: ""
            desiredState:
              interfaces:
              - bridge:
                  options:
                    stp:
                      enabled: false
                  port:
                  - name: enp7s0
                description: Linux bridge with enp7s0 as a port
                name: br-osp
                state: up
                type: linux-bridge
                mtu: 1500
        br-ex:
          nodeNetworkConfigurationPolicy:
            nodeSelector:
              node-role.kubernetes.io/worker: ""
            desiredState:
              interfaces:
              - bridge:
                  options:
                    stp:
                      enabled: false
                  port:
                  - name: enp6s0
                description: Linux bridge with enp6s0 as a port
                name: br-ex
                state: up
                type: linux-bridge
                mtu: 1500
      # optional DnsServers list
      dnsServers:
      - 192.168.25.1
      # optional DnsSearchDomains list
      dnsSearchDomains:
      - osptest.test.metalkube.org
      - some.other.domain
      # DomainName of the OSP environment
      domainName: osptest.test.metalkube.org
      networks:
      - name: Control
        nameLower: ctlplane
        subnets:
        - name: ctlplane
          ipv4:
            allocationEnd: 192.168.25.250
            allocationStart: 192.168.25.100
            cidr: 192.168.25.0/24
            gateway: 192.168.25.1
          attachConfiguration: br-osp
      - name: InternalApi
        nameLower: internal_api
        mtu: 1350
        subnets:
        - name: internal_api
          attachConfiguration: br-osp
          vlan: 20
          ipv4:
            allocationEnd: 172.17.0.250
            allocationStart: 172.17.0.10
            cidr: 172.17.0.0/24
      - name: External
        nameLower: external
        subnets:
        - name: external
          ipv6:
            allocationEnd: 2001:db8:fd00:1000:ffff:ffff:ffff:fffe
            allocationStart: 2001:db8:fd00:1000::10
            cidr: 2001:db8:fd00:1000::/64
            gateway: 2001:db8:fd00:1000::1
          attachConfiguration: br-ex
      - name: Storage
        nameLower: storage
        mtu: 1350
        subnets:
        - name: storage
          ipv4:
            allocationEnd: 172.18.0.250
            allocationStart: 172.18.0.10
            cidr: 172.18.0.0/24
          vlan: 30
          attachConfiguration: br-osp
      - name: StorageMgmt
        nameLower: storage_mgmt
        mtu: 1350
        subnets:
        - name: storage_mgmt
          ipv4:
            allocationEnd: 172.19.0.250
            allocationStart: 172.19.0.10
            cidr: 172.19.0.0/24
          vlan: 40
          attachConfiguration: br-osp
      - name: Tenant
        nameLower: tenant
        vip: False
        mtu: 1350
        subnets:
        - name: tenant
          ipv4:
            allocationEnd: 172.20.0.250
            allocationStart: 172.20.0.10
            cidr: 172.20.0.0/24
          vlan: 50
          attachConfiguration: br-osp
    ```

    When using VLAN for network isolation with linux-bridge
    - a Node Network Configuration Policy gets created for the bridge interface specified in the osnet CR, which uses nmstate to configure the bridge on the worker node
    - for each network a Network Attach Definition gets created which defines the Multus CNI plugin configuration. Specifying the vlan ID on the Network Attach Definition enables the bridge vlan-filtering.
    - for each network a dedicated interface gets attached to the virtual machine. Therefore the network template for the OSVMSet is a multi-nic network template

    **NOTE**: To use Jumbo Frames for a bridge, create a configuration for the device to configure the correnct MTU:
    ```yaml
    apiVersion: osp-director.openstack.org/v1beta1
    kind: OpenStackNetConfig
    metadata:
      name: openstacknetconfig
    spec:
      attachConfigurations:
        br-osp:
          nodeNetworkConfigurationPolicy:
            nodeSelector:
              node-role.kubernetes.io/worker: ""
            desiredState:
              interfaces:
              - bridge:
                  options:
                    stp:
                      enabled: false
                  port:
                  - name: enp7s0
                description: Linux bridge with enp7s0 as a port
                name: br-osp
                state: up
                type: linux-bridge
                mtu: 9000
              - name: enp7s0
                description: Configuring enp7s0 on workers
                type: ethernet
                state: up
                mtu: 9000
    ```

2) Create [ConfigMaps](https://kubernetes.io/docs/concepts/configuration/configmap/) which define any custom Heat environments, Heat templates and custom roles file (name must be `roles_data.yaml`) used for TripleO network configuration. Any adminstrator defined Heat environment files can be provided in the ConfigMap and will be used as a convention in later steps used to create the Heat stack for Overcloud deployment. As a convention each OSP Director Installation will use 2 ConfigMaps named `heat-env-config` and `tripleo-tarball-config` to provide this information. The `heat-env-config` configmap holds all deployment environment files where each file gets added as `-e file.yaml` to the `openstack stack create` command. A good example is:

    - [Tripleo Deploy custom files](https://github.com/openstack-k8s-operators/osp-director-dev-tools/tree/master/ansible/templates/osp/tripleo_deploy)
        **NOTE**: these are Ansible templates and need to have variables replaced to be used directly!
        **NOTE**: all references in the environment files need to be relative to the t-h-t root where the tarball gets extracted!

    A "Tarball Config Map" can be used to provide (binary) tarballs which are extracted in the tripleo-heat-templates when playbooks are generated. Each tarball should contain a directory of files relative to the root of a t-h-t directory. You will want to store things like the following examples in a config map containing custom tarballs:

    - [Net-Config files](https://github.com/openstack-k8s-operators/osp-director-dev-tools/tree/master/ansible/files/osp/net_config).

    - [Net-Config environment](https://github.com/openstack-k8s-operators/osp-director-dev-tools/blob/master/ansible/templates/osp/tripleo_deploy/vlan/network-environment.yaml.j2)

        **NOTE**: Net-Config files for the virtual machines get created by the operator, but can be overwritten using the "Tarball Config Map". To overwrite a pre-rendered Net-Config use the `<role lowercase>-nic-template.yaml` file name for OSP16.2 or `<role lowercase>-nic-template.j2` for OSP17.
        **NOTE**: network interface names for the VMs created by the OpenStackVMSet controller are alphabetically ordered by the network names assigned to the VM role. An exception is the `default` network interface of the VM pod which will always is the first interface. The resulting inteface section of the virtual machine definition will look like this:

        ```yaml
        interfaces:
          - masquerade: {}
            model: virtio
            name: default
          - bridge: {}
            model: virtio
            name: ctlplane
          - bridge: {}
            model: virtio
            name: external
          - bridge: {}
            model: virtio
            name: internalapi
          - bridge: {}
            model: virtio
            name: storage
          - bridge: {}
            model: virtio
            name: storagemgmt
          - bridge: {}
            model: virtio
            name: tenant
        ```

        With this the ctlplane interface is nic2, external nic3, ... and so on.

        **NOTE**: FIP traffic does not pass to a VLAN tenant network with ML2/OVN and DVR. DVR is enabled by default. If you need VLAN tenant networks with OVN, you can disable DVR. To disable DVR, include the following lines in an environment file:

        ```yaml
        parameter_defaults:
          NeutronEnableDVR: false
        ```
      
        Support for "distributed vlan traffic in ovn" is being tracked in [manage MAC addresses for "Add support in tripleo for distributed vlan traffic in ovn" ( https://bugs.launchpad.net/tripleo/+bug/1881593 )](https://github.com/openstack-k8s-operators/osp-director-operator/issues/254)

    - [Git repo config map] This ConfigMap contains the SSH key and URL for the Git repo used to store generated playbooks (below)

    Once you customize the above template/examples for your environment you can create configmaps for both the 'heat-env-config' and 'tripleo-tarball-config'(tarballs) ConfigMaps by using these example commands on the files containing each respective configmap type (one directory for each type of configmap):

    ```bash
    # create the configmap for heat-env-config
    oc create configmap -n openstack heat-env-config --from-file=heat-env-config/ --dry-run=client -o yaml | oc apply -f -

    # create the configmap containing a tarball of t-h-t network config files. NOTE: these files may overwrite default t-h-t files so keep this in mind when naming them.
    cd <dir with net config files>
    tar -cvzf net-config.tar.gz *.yaml
    oc create configmap -n openstack tripleo-tarball-config --from-file=tarball-config.tar.gz

    # create the Git secret used for the repo where Ansible playbooks are stored
    oc create secret generic git-secret -n openstack --from-file=git_ssh_identity=<path to git id_rsa> --from-literal=git_url=<your git server URL (git@...)>

    ```

3) (Optional) Create a [Secret](https://kubernetes.io/docs/concepts/configuration/secret/) for your OpenStackControlPlane. This secret will provide the default password for your virtual machine and baremetal hosts. If no secret is provided you will only be able to login with ssh keys defined in the osp-controlplane-ssh-keys Secret.
    ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
      name: userpassword
      namespace: openstack
    data:
      # 12345678
      NodeRootPassword: MTIzNDU2Nzg=
    ```

    If you write the above YAML into a file called ctlplane-secret.yaml you can create the Secret via this command:

    ```bash
    oc create -n openstack -f ctlplane-secret.yaml
    ```

4) Define your OpenStackControlPlane custom resource. The OpenStackControlPlane custom resource provides a central place to create and scale VMs used for the OSP Controllers along with any additional vmsets for your deployment. At least 1 Controller VM is required for a basic demo installation and per OSP High Availability guidelines 3 Controller VMs are recommended.

    **NOTE**: If the rhel-guest-image is used as base to deploy the OpenStackControlPlane virtual machines, make sure to remove the net.ifnames=0 kernel parameter from the image to have the biosdev network interface naming. This can be done like:

    ```bash
    dnf install -y libguestfs-tools-c
    virt-customize -a bms-image.qcow2 --run-command 'sed -i -e "s/^\(kernelopts=.*\)net.ifnames=0 \(.*\)/\1\2/" /boot/grub2/grubenv'
    ```

    ```yaml
    apiVersion: osp-director.openstack.org/v1beta1
    kind: OpenStackControlPlane
    metadata:
      name: overcloud
      namespace: openstack
    spec:
      openStackClientImageURL: quay.io/openstack-k8s-operators/rhosp16-openstack-tripleoclient:16.2_20210713.1
      openStackClientNetworks:
            - ctlplane
            - external
            - internalapi
      # openStackClientStorageClass must support RWX
      # https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes
      openStackClientStorageClass: host-nfs-storageclass
      passwordSecret: userpassword
      gitSecret: git-secret
      virtualMachineRoles:
        controller:
          roleName: Controller
          roleCount: 3
          networks:
            - ctlplane
            - internalapi
            - external
            - tenant
            - storage
            - storagemgmt
          cores: 6
          memory: 12
          diskSize: 50
          baseImageVolumeName: openstack-base-img
          # storageClass must support RWX to be able to live migrate VMs
          storageClass: host-nfs-storageclass
          storageAccessMode: ReadWriteMany
          # When using OpenShift Virtualization with OpenShift Container Platform Container Storage,
          # specify RBD block mode persistent volume claims (PVCs) when creating virtual machine disks. With virtual machine disks,
          # RBD block mode volumes are more efficient and provide better performance than Ceph FS or RBD filesystem-mode PVCs.
          # To specify RBD block mode PVCs, use the 'ocs-storagecluster-ceph-rbd' storage class and VolumeMode: Block.
          storageVolumeMode: Filesystem
    ```

    If you write the above YAML into a file called openstackcontrolplane.yaml you can create the OpenStackControlPlane via this command:

    ```bash
    oc create -f openstackcontrolplane.yaml
    ```

5) Define an OpenStackBaremetalSet to scale out OSP Compute hosts. The OpenStackBaremetal resource can be used to define and scale Compute resources and optionally be used to define and scale out baremetal hosts for other types of TripleO roles. The example below defines a single Compute host to be created.

    **NOTE**: If the rhel-guest-image is used as base to deploy the OpenStackBaremetalSet compute nodes, make sure to remove the net.ifnames=0 kernel parameter from the image to have the biosdev network interface naming. This can be done like:

    ```bash
    dnf install -y libguestfs-tools-c
    virt-customize -a bms-image.qcow2 --run-command 'sed -i -e "s/^\(kernelopts=.*\)net.ifnames=0 \(.*\)/\1\2/" /boot/grub2/grubenv'
    ```

    ```yaml
    apiVersion: osp-director.openstack.org/v1beta1
    kind: OpenStackBaremetalSet
    metadata:
      name: compute
      namespace: openstack
    spec:
      # How many nodes to provision
      count: 1
      # The image to install on the provisioned nodes. NOTE: needs to be accessible on the OpenShift Metal3 provisioning network.
      baseImageUrl: http://host/images/rhel-image-8.4.x86_64.qcow2
      # NOTE: these are automatically created via the OpenStackControlplane CR above
      deploymentSSHSecret: osp-controlplane-ssh-keys
      # The interface on the nodes that will be assigned an IP from the mgmtCidr
      ctlplaneInterface: enp7s0
      # Networks to associate with this host
      networks:
        - ctlplane
        - internalapi
        - tenant
        - storage
      roleName: Compute
      passwordSecret: userpassword
    ```

    If you write the above YAML into a file called compute.yaml you can create the OpenStackBaremetalSet via this command:

    ```bash
    oc create -f compute.yaml
    ```

6) Node registration (register the overcloud systems to required channels)

    Wait for the above resource to finish deploying (Compute and ControlPlane). Once the resources finish deploying proceed with node registration.

    Use the procedure as described in [5.9. Running Ansible-based registration manually](https://access.redhat.com/documentation/en-us/red_hat_openstack_platform/16.2/html-single/advanced_overcloud_customization/index#running-ansible-based-registration-manually-portal) do do so.

    NOTE: We recommend using manual registration as it works regardless of base image choice. If you are using
    overcloud-full as your base deployment image then automatic RHSM registration could be used via the 
    t-h-t rhsm.yaml environment role/file as an alternative to this approach. 

    ```bash
    oc rsh openstackclient
    bash
    cd /home/cloud-admin

    <create the ansible playbook for the overcloud nodes - e.g. rhsm.yaml>

    # register the overcloud nodes to required repositories
    ansible-playbook -i /home/cloud-admin/ctlplane-ansible-inventory ./rhsm.yaml
    ```

7) (optional) Create roles file
    a) use the openstackclient pod to generate a custom roles file
    ```bash
    oc rsh openstackclient
    unset OS_CLOUD
    cd /home/cloud-admin/
    openstack overcloud roles generate Controller ComputeHCI > roles_data.yaml
    exit
    ```

    b) copy the custom roles file out of the openstackclient pod
    ```bash
    oc cp openstackclient:/home/cloud-admin/roles_data.yaml roles_data.yaml
    ```

    Update the `tarballConfigMap` configmap to add the `roles_data.yaml` file to the tarball and update the configmap.

    **NOTE**: Make sure to use `roles_data.yaml` as the file name.

8) Define an OpenStackConfigGenerator to generate ansible playbooks for the OSP cluster deployment.
    ```yaml
    apiVersion: osp-director.openstack.org/v1beta1
    kind: OpenStackConfigGenerator
    metadata:
      name: default
      namespace: openstack
    spec:
      imageURL: quay.io/openstack-k8s-operators/rhosp16-openstack-tripleoclient:16.2_20210713.1
      gitSecret: git-secret
      heatEnvConfigMap: heat-env-config
      tarballConfigMap: tripleo-tarball-config
      # (optional) for debugging it is possible to set the interactive mode.
      # In this mode the playbooks won't get rendered automatically. Just the environment to start the rendering gets created
      # interactive: true
      # (optional) provide custom registry or specific container versions via the ephemeralHeatSettings
      #ephemeralHeatSettings:
      #  heatAPIImageURL: quay.io/tripleotraincentos8/centos-binary-heat-api:current-tripleo
      #  heatEngineImageURL: quay.io/tripleotraincentos8/centos-binary-heat-engine:current-tripleo
      #  mariadbImageURL: quay.io/tripleotraincentos8/centos-binary-mariadb:current-tripleo
      #  rabbitImageURL: quay.io/tripleotraincentos8/centos-binary-rabbitmq:current-tripleo
    ```

    If you write the above YAML into a file called generator.yaml you can create the OpenStackConfigGenerator via this command:

    ```bash
    oc create -f generator.yaml
    ```

    The osconfiggenerator created above will automatically generate playbooks any time you scale or modify the ConfigMaps for your OSP deployment. Generating these playbooks takes several minutes. You can monitor the osconfiggenerator's status condition for it to finish.

9) Login to the 'openstackclient' pod and deploy the OSP software via the rendered ansible playbooks. At this point all baremetal and virtualmachine resources have been provisioned within the OCP cluster.

    The `tripleo-deploy.sh` script supports three actions:
    * `-d` - show the `git diff` of the playbooks to the previous accepted
    * `-a` - accept the new available rendered playbooks and tag them as `latest`
    * `-p` - run the ansible driven OpenStack deployment

    a) check for new version of rendered playbooks and accept them

    ```bash
    oc rsh openstackclient
    bash
    cd /home/cloud-admin

    # (optional) show the `git diff` of the playbooks to the previous accepted
    ./tripleo-deploy.sh -d

    # accept the new available rendered playbooks (if available) and tag them as `latest`
    ./tripleo-deploy.sh -a
    ```
    b) run ansible driven OpenStack deployment

    ```bash
    oc rsh openstackclient
    bash
    cd /home/cloud-admin

    # run ansible driven OpenStack deployment
    ./tripleo-deploy.sh -p
    ```

Deploy Ceph via tripleo using ComputeHCI
----------------------------------------

It is possible to deploy tripleo's Hyper-Converged Infrastructure where compute nodes also act as Ceph OSD nodes.
The workflow to install Ceph via tripleo would be:

## Control Plane

Make sure to use `quay.io/openstack-k8s-operators/rhosp16-openstack-tripleoclient:16.2_20210521.1` or later for the openstackclient `openStackClientImageURL`.

## Baremetalset

Have compute nodes with extra disks to be used as OSDs and create a baremetalset for the ComputeHCI role which has
the storagemgmt network in addition to the default compute networks and the `IsHCI` parameter set to true.

**NOTE**: If the rhel-guest-image is used as base to deploy the OpenStackBaremetalSet compute nodes, make sure to remove the net.ifnames=0 kernel parameter form the image to have the biosdev network interface naming. This can be done like:

```bash
dnf install -y libguestfs-tools-c
virt-customize -a bms-image.qcow2 --run-command 'sed -i -e "s/^\(kernelopts=.*\)net.ifnames=0 \(.*\)/\1\2/" /boot/grub2/grubenv'
```

```yaml
apiVersion: osp-director.openstack.org/v1beta1
kind: OpenStackBaremetalSet
metadata:
  name: computehci
  namespace: openstack
spec:
  # How many nodes to provision
  replicas: 2
  # The image to install on the provisioned nodes
  baseImageUrl: http://host/images/rhel-image-8.4.x86_64.qcow2
  # The secret containing the SSH pub key to place on the provisioned nodes
  deploymentSSHSecret: osp-controlplane-ssh-keys
  # The interface on the nodes that will be assigned an IP from the mgmtCidr
  ctlplaneInterface: enp7s0
  # Networks to associate with this host
  networks:
    - ctlplane
    - internalapi
    - tenant
    - storage
    - storagemgmt
  roleName: ComputeHCI
  passwordSecret: userpassword
```

## Custom deployment parameters

* create a roles file as described in section `Deploying OpenStack once you have the OSP Director Operator installed` which includes the computeHCI role
* update the Net-Config to have the storagemgmt network for the ComputeHCI network config template
* add Ceph related deployment parameters from `/usr/share/openstack-tripleo-heat-templates/environments/ceph-ansible/ceph-ansible.yaml` and any other customization to the Tripleo Deploy custom configMap, e.g. `storage-backend.yaml`:

```yaml
resource_registry:
  OS::TripleO::Services::CephMgr: deployment/ceph-ansible/ceph-mgr.yaml
  OS::TripleO::Services::CephMon: deployment/ceph-ansible/ceph-mon.yaml
  OS::TripleO::Services::CephOSD: deployment/ceph-ansible/ceph-osd.yaml
  OS::TripleO::Services::CephClient: deployment/ceph-ansible/ceph-client.yaml

parameter_defaults:
  # needed for now because of the repo used to create tripleo-deploy image
  CephAnsibleRepo: "rhelosp-ceph-4-tools"
  CephAnsiblePlaybookVerbosity: 3
  CinderEnableIscsiBackend: false
  CinderEnableRbdBackend: true
  CinderBackupBackend: ceph
  CinderEnableNfsBackend: false
  NovaEnableRbdBackend: true
  GlanceBackend: rbd
  CinderRbdPoolName: "volumes"
  NovaRbdPoolName: "vms"
  GlanceRbdPoolName: "images"
  CephPoolDefaultPgNum: 32
  CephPoolDefaultSize: 2
  CephAnsibleDisksConfig:
    devices:
      - '/dev/sdb'
      - '/dev/sdc'
      - '/dev/sdd'
    osd_scenario: lvm
    osd_objectstore: bluestore
  CephAnsibleExtraConfig:
    is_hci: true
  CephConfigOverrides:
    rgw_swift_enforce_content_length: true
    rgw_swift_versioning_enabled: true
```

Once you customize the above template/examples for your environment, create/update configmaps like explained in `Deploying OpenStack once you have the OSP Director Operator installed`

## Render playbooks and apply them

* Define an OpenStackConfigGenerator to generate ansible playbooks for the OSP cluster deployment as in `Deploying OpenStack once you have the OSP Director Operator installed` and specify the roles generated roles file.

**NOTE**: Make sure to use `quay.io/openstack-k8s-operators/rhosp16-openstack-tripleoclient:16.2_20210521.1` or later for the osconfiggenerator `imageURL`.

## Run the software deployment

* Wait for the OpenStackConfigGenerator to finish the playbook rendering job

* In the openstackclient pod

a) check for new version of rendered playbooks and accept them

```bash
oc rsh openstackclient
bash
cd /home/cloud-admin

# (optional) show the `git diff` of the playbooks to the previous accepted
./tripleo-deploy.sh -d

# (optional) accept the new available rendered playbooks (if available) and tag them as `latest`
./tripleo-deploy.sh -a
```

b) Install the pre-requisites on overcloud systems for ceph-ansible

```bash
cd /home/cloud-admin
ansible -i /home/cloud-admin/playbooks/tripleo-ansible/inventory.yaml overcloud -a "sudo dnf -y install python3 lvm2"
```

d) run ansible driven OpenStack deployment

**NOTE**: for now the validation for ceph get skipped by adding `--skip-tags opendev-validation-ceph` when run ansible playbooks

```bash
oc rsh openstackclient
bash
cd /home/cloud-admin

# run ansible driven OpenStack deployment
./tripleo-deploy.sh -p
```

Remove a baremetal compute host
-------------------------------

Removing a baremetal compute host requires the following steps:

## Disable the compute service

In case a compute node gets removed, disable the Compute service on the outgoing node on the overcloud to prevent the node from scheduling new instances

```bash
openstack compute service list
openstack compute service set <hostname> nova-compute --disable
```

## Annotate the BMH resource for deletion

Annotation of a BMH resource

```bash
oc annotate -n openshift-machine-api bmh/openshift-worker-3 osp-director.openstack.org/delete-host=true --overwrite
```

The annotation status is being reflected in the OSBaremetalset/OSVMset using the `annotatedForDeletion` parameter:

```bash
oc get osbms computehci -o json | jq .status
{
  "baremetalHosts": {
    "computehci-0": {
      "annotatedForDeletion": true,
      "ctlplaneIP": "192.168.25.105/24",
      "hostRef": "openshift-worker-3",
      "hostname": "computehci-0",
      "networkDataSecretName": "computehci-cloudinit-networkdata-openshift-worker-3",
      "provisioningState": "provisioned",
      "userDataSecretName": "computehci-cloudinit-userdata-openshift-worker-3"
    },
    "computehci-1": {
      "annotatedForDeletion": false,
      "ctlplaneIP": "192.168.25.106/24",
      "hostRef": "openshift-worker-4",
      "hostname": "computehci-1",
      "networkDataSecretName": "computehci-cloudinit-networkdata-openshift-worker-4",
      "provisioningState": "provisioned",
      "userDataSecretName": "computehci-cloudinit-userdata-openshift-worker-4"
    }
  },
  "provisioningStatus": {
    "readyCount": 2,
    "reason": "All requested BaremetalHosts have been provisioned",
    "state": "provisioned"
  }
}
```

## Reduce the resource count

Reducing the resource count of the OSBaremetalset will trigger the corrensponding controller to handle the resource deletion

```bash
oc patch osbms computehci --type=merge --patch '{"spec":{"count":1}}'
```

As a result:
* the IPreservation entry in the OSNet resources gets flagged as deleted

```bash
oc get osnet ctlplane -o json | jq .status.roleReservations.ComputeHCI
{
  "addToPredictableIPs": true,
  "reservations": [
    {
      "deleted": true,
      "hostname": "computehci-0",
      "ip": "192.168.25.105",
      "vip": false
    },
    {
      "deleted": false,
      "hostname": "computehci-1",
      "ip": "192.168.25.106",
      "vip": false
    }
  ]
}
```

This results in the following behavior
* the IP is not free for use for another role
* if a new node gets scaled into the same role it will reuse the hostnames starting with lowest id suffix (if there are multiple) _and_ corresponding IP reservation
* if the OSBaremetalset or OSVMset resource gets deleted, all IP reservations for the role get deleted and are free to be used by other nodes

## Cleanup OpenStack resources

Right now if a compute node got removed, there are several leftover entries registerd on the OpenStack control plane and not being cleaned up automatically. To clean them up, perform the following steps.

### Remove the Compute service from the node

```bash
openstack compute service list
openstack compute service delete <service-id>
```

### Check the network agents and remove if needed

```bash
openstack network agent list
for AGENT in $(openstack network agent list --host <scaled-down-node> -c ID -f value) ; do openstack network agent delete $AGENT ; done
```

Remove an OpenStackControlPlane VM
----------------------------------

Removing an VM requires the following steps:

## (optional) Disable OSP service

If the VM hosts any OSP service which should be disabled before the removal, do so.

## Annotate the VM resource for deletion

Annotation of a VM resource

```bash
oc annotate -n openstack vm/controller-1 osp-director.openstack.org/delete-host=true --overwrite
```

## Reduce the roleCount

Reducing the resource roleCount of the virtualMachineRoles in the OpenStackControlPlane CR. The corrensponding controller to handle the resource deletion

```bash
oc patch osctlplane overcloud --type=merge --patch '{"spec":{"virtualMachineRoles":{"<RoleName>":{"roleCount":2}}}}'
```

As a result:
* the IPreservation entry in the OSNet resources is flagged as deleted

This results in the following behavior
* the IP is not free for use for another role
* if a new node gets scaled into the same role it will reuse the hostnames starting with lowest id suffix (if there are multiple) _and_ corresponding IP reservation
* if the OSBaremetalset or OSVMset resource gets deleted, all IP reservations for the role get deleted and are free to be used by other nodes

## (optional) Cleanup OpenStack resources

If the VM did host any OSP service which should be removed, delete the service using the corresponding openstack command.

Deploy nodes using multiple subnets (spine/leaf)
------------------------------------------------

It is possible to deploy tripleo's routed networks ([Spine/Leaf Networking](https://access.redhat.com/documentation/en-us/red_hat_openstack_platform/16.2/html-single/spine_leaf_networking/index)) architecture to configure overcloud leaf networks. Use the subnets parameter to define the additional Leaf subnets with a base network.

A limitation right now is that there can only be one provision network for metal3.

The workflow to install an overcloud using multiple subnets would be:

## Create/Update the OpenStackNetConfig CR to define all subnets

Define your OpenStackNetConfig custom resource and specify all the subnets for the overcloud networks. The operator will render the tripleo network_data.yaml for the used OSP release.

```yaml
apiVersion: osp-director.openstack.org/v1beta1
kind: OpenStackNetConfig
metadata:
  name: openstacknetconfig
spec:
  attachConfigurations:
    br-osp:
      nodeNetworkConfigurationPolicy:
        nodeSelector:
          node-role.kubernetes.io/worker: ""
        desiredState:
          interfaces:
          - bridge:
              options:
                stp:
                  enabled: false
              port:
              - name: enp7s0
            description: Linux bridge with enp7s0 as a port
            name: br-osp
            state: up
            type: linux-bridge
            mtu: 1500
    br-ex:
      nodeNetworkConfigurationPolicy:
        nodeSelector:
          node-role.kubernetes.io/worker: ""
        desiredState:
          interfaces:
          - bridge:
              options:
                stp:
                  enabled: false
              port:
              - name: enp6s0
            description: Linux bridge with enp6s0 as a port
            name: br-ex
            state: up
            type: linux-bridge
            mtu: 1500
  # optional DnsServers list
  dnsServers:
  - 192.168.25.1
  # optional DnsSearchDomains list
  dnsSearchDomains:
  - osptest.test.metalkube.org
  - some.other.domain
  # DomainName of the OSP environment
  domainName: osptest.test.metalkube.org
  networks:
  - name: Control
    nameLower: ctlplane
    subnets:
    - name: ctlplane
      ipv4:
        allocationEnd: 192.168.25.250
        allocationStart: 192.168.25.100
        cidr: 192.168.25.0/24
        gateway: 192.168.25.1
      attachConfiguration: br-osp
  - name: InternalApi
    nameLower: internal_api
    mtu: 1350
    subnets:
    - name: internal_api
      ipv4:
        allocationEnd: 172.17.0.250
        allocationStart: 172.17.0.10
        cidr: 172.17.0.0/24
        routes:
        - destination: 172.17.1.0/24
          nexthop: 172.17.0.1
        - destination: 172.17.2.0/24
          nexthop: 172.17.0.1
      vlan: 20
      attachConfiguration: br-osp
    - name: internal_api_leaf1
      ipv4:
        allocationEnd: 172.17.1.250
        allocationStart: 172.17.1.10
        cidr: 172.17.1.0/24
        routes:
        - destination: 172.17.0.0/24
          nexthop: 172.17.1.1
        - destination: 172.17.2.0/24
          nexthop: 172.17.1.1
      vlan: 21
      attachConfiguration: br-osp
    - name: internal_api_leaf2
      ipv4:
        allocationEnd: 172.17.2.250
        allocationStart: 172.17.2.10
        cidr: 172.17.2.0/24
        routes:
        - destination: 172.17.1.0/24
          nexthop: 172.17.2.1
        - destination: 172.17.0.0/24
          nexthop: 172.17.2.1
      vlan: 22
      attachConfiguration: br-osp
  - name: External
    nameLower: external
    subnets:
    - name: external
      ipv4:
        allocationEnd: 10.0.0.250
        allocationStart: 10.0.0.10
        cidr: 10.0.0.0/24
        gateway: 10.0.0.1
      attachConfiguration: br-ex
  - name: Storage
    nameLower: storage
    mtu: 1350
    subnets:
    - name: storage
      ipv4:
        allocationEnd: 172.18.0.250
        allocationStart: 172.18.0.10
        cidr: 172.18.0.0/24
        routes:
        - destination: 172.18.1.0/24
          nexthop: 172.18.0.1
        - destination: 172.18.2.0/24
          nexthop: 172.18.0.1
      vlan: 30
      attachConfiguration: br-osp
    - name: storage_leaf1
      ipv4:
        allocationEnd: 172.18.1.250
        allocationStart: 172.18.1.10
        cidr: 172.18.1.0/24
        routes:
        - destination: 172.18.0.0/24
          nexthop: 172.18.1.1
        - destination: 172.18.2.0/24
          nexthop: 172.18.1.1
      vlan: 31
      attachConfiguration: br-osp
    - name: storage_leaf2
      ipv4:
        allocationEnd: 172.18.2.250
        allocationStart: 172.18.2.10
        cidr: 172.18.2.0/24
        routes:
        - destination: 172.18.0.0/24
          nexthop: 172.18.2.1
        - destination: 172.18.1.0/24
          nexthop: 172.18.2.1
      vlan: 32
      attachConfiguration: br-osp
  - name: StorageMgmt
    nameLower: storage_mgmt
    mtu: 1350
    subnets:
    - name: storage_mgmt
      ipv4:
        allocationEnd: 172.19.0.250
        allocationStart: 172.19.0.10
        cidr: 172.19.0.0/24
        routes:
        - destination: 172.19.1.0/24
          nexthop: 172.19.0.1
        - destination: 172.19.2.0/24
          nexthop: 172.19.0.1
      vlan: 40
      attachConfiguration: br-osp
    - name: storage_mgmt_leaf1
      ipv4:
        allocationEnd: 172.19.1.250
        allocationStart: 172.19.1.10
        cidr: 172.19.1.0/24
        routes:
        - destination: 172.19.0.0/24
          nexthop: 172.19.1.1
        - destination: 172.19.2.0/24
          nexthop: 172.19.1.1
      vlan: 41
      attachConfiguration: br-osp
    - name: storage_mgmt_leaf2
      ipv4:
        allocationEnd: 172.19.2.250
        allocationStart: 172.19.2.10
        cidr: 172.19.2.0/24
        routes:
        - destination: 172.19.0.0/24
          nexthop: 172.19.2.1
        - destination: 172.19.1.0/24
          nexthop: 172.19.2.1
      vlan: 42
      attachConfiguration: br-osp
  - name: Tenant
    nameLower: tenant
    vip: False
    mtu: 1350
    subnets:
    - name: tenant
      ipv4:
        allocationEnd: 172.20.0.250
        allocationStart: 172.20.0.10
        cidr: 172.20.0.0/24
        routes:
        - destination: 172.20.1.0/24
          nexthop: 172.20.0.1
        - destination: 172.20.2.0/24
          nexthop: 172.20.0.1
      vlan: 50
      attachConfiguration: br-osp
    - name: tenant_leaf1
      ipv4:
        allocationEnd: 172.20.1.250
        allocationStart: 172.20.1.10
        cidr: 172.20.1.0/24
        routes:
        - destination: 172.20.0.0/24
          nexthop: 172.20.1.1
        - destination: 172.20.2.0/24
          nexthop: 172.20.1.1
      vlan: 51
      attachConfiguration: br-osp
    - name: tenant_leaf2
      ipv4:
        allocationEnd: 172.20.2.250
        allocationStart: 172.20.2.10
        cidr: 172.20.2.0/24
        routes:
        - destination: 172.20.0.0/24
          nexthop: 172.20.2.1
        - destination: 172.20.1.0/24
          nexthop: 172.20.2.1
      vlan: 52
      attachConfiguration: br-osp
```

If you write the above YAML into a file called networkconfig.yaml you can create the OpenStackNetConfig via this command:

```bash
oc create -n openstack -f networkconfig.yaml
```

## Add roles to the roles_data.yaml and reference the subnets

```yaml
...
###############################################################################
# Role: ComputeLeaf1                                                          #
###############################################################################
- name: ComputeLeaf1
  description: |
    Basic ComputeLeaf1 Node role
  # Create external Neutron bridge (unset if using ML2/OVS without DVR)
  tags:
    - external_bridge
  networks:
    InternalApi:
      subnet: internal_api_leaf1
    Tenant:
      subnet: tenant_leaf1
    Storage:
      subnet: storage_leaf1
  HostnameFormatDefault: '%stackname%-novacompute-leaf1-%index%'
...
###############################################################################
# Role: ComputeLeaf2                                                          #
###############################################################################
- name: ComputeLeaf2
  description: |
    Basic ComputeLeaf1 Node role
  # Create external Neutron bridge (unset if using ML2/OVS without DVR)
  tags:
    - external_bridge
  networks:
    InternalApi:
      subnet: internal_api_leaf2
    Tenant:
      subnet: tenant_leaf2
    Storage:
      subnet: storage_leaf2
  HostnameFormatDefault: '%stackname%-novacompute-leaf2-%index%'
...
```

Update the `tarballConfigMap` configmap to add the `roles_data.yaml` file to the tarball and update the configmap.

**NOTE**: Make sure to use `roles_data.yaml` as the file name.

## Create NIC templates for the new roles

Routes subnet information gets auto rendered to the tripleo environment file `environments/network-environment.yaml` which is used in the script rendering the ansible playbooks. In the NIC templates therefore use <NetName>Routes_<subnet_name>, e.g. StorageRoutes_storage_leaf1 to set the correct routing on the host.

### OSP16.2/train NIC templates modification

For a the ComputeLeaf1 compute role the NIC template needs to be modified to use those:

```yaml
...
  StorageRoutes_storage_leaf1:
    default: []
    description: >
      Routes for the storage network traffic.
      JSON route e.g. [{'destination':'10.0.0.0/16', 'nexthop':'10.0.0.1'}]
      Unless the default is changed, the parameter is automatically resolved
      from the subnet host_routes attribute.
    type: json
...
  InternalApiRoutes_internal_api_leaf1:
    default: []
    description: >
      Routes for the internal_api network traffic.
      JSON route e.g. [{'destination':'10.0.0.0/16', 'nexthop':'10.0.0.1'}]
      Unless the default is changed, the parameter is automatically resolved
      from the subnet host_routes attribute.
    type: json
...
  TenantRoutes_tenant_leaf1:
    default: []
    description: >
      Routes for the internal_api network traffic.
      JSON route e.g. [{'destination':'10.0.0.0/16', 'nexthop':'10.0.0.1'}]
      Unless the default is changed, the parameter is automatically resolved
      from the subnet host_routes attribute.
    type: json
...
                       get_param: StorageIpSubnet
                   routes:
                     list_concat_unique:
                      - get_param: StorageRoutes_storage_leaf1
                 - type: vlan
...
                       get_param: InternalApiIpSubnet
                   routes:
                     list_concat_unique:
                      - get_param: InternalApiRoutes_internal_api_leaf1
...
                       get_param: TenantIpSubnet
                   routes:
                     list_concat_unique:
                      - get_param: TenantRoutes_tenant_leaf1
               - type: ovs_bridge
...
```

Update the `tarballConfigMap` configmap to add the NIC templates `roles_data.yaml` file to the tarball and update the configmap.

**NOTE**: Make sure to use `roles_data.yaml` as the file name.

### OSP17.0/wallaby ansible NIC template modification

So far only OSP16.2 was tested with multiple subnet deployment and is compatible with OSP17.0 single subnet.

TBD

### Create/Update an environment file to register the NIC templates

Make sure to add the new created NIC templates to the environment file to the `resource_registry`
for the new node roles:

```yaml
resource_registry:
  OS::TripleO::Compute::Net::SoftwareConfig: net-config-two-nic-vlan-compute.yaml
  OS::TripleO::ComputeLeaf1::Net::SoftwareConfig: net-config-two-nic-vlan-compute_leaf1.yaml
  OS::TripleO::ComputeLeaf2::Net::SoftwareConfig: net-config-two-nic-vlan-compute_leaf2.yaml
```

## Deploy the overcloud using multiple subnets

At this point we can provision the overcloud.

### Create the Control Plane

```yaml
apiVersion: osp-director.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: overcloud
  namespace: openstack
spec:
  gitSecret: git-secret
  openStackClientImageURL: registry.redhat.io/rhosp-rhel8/openstack-tripleoclient:16.2
  openStackClientNetworks:
    - ctlplane
    - external
    - internal_api
    - internal_api_leaf1 # optionally the openstackclient can also be connected to subnets
  openStackClientStorageClass: host-nfs-storageclass
  passwordSecret: userpassword
  domainName: ostest.test.metalkube.org
  virtualMachineRoles:
    Controller:
      roleName: Controller
      roleCount: 1
      networks:
        - ctlplane
        - internal_api
        - external
        - tenant
        - storage
        - storage_mgmt
      cores: 6
      memory: 20
      diskSize: 40
      baseImageVolumeName: controller-base-img
      storageClass: host-nfs-storageclass
  enableFencing: False
```

### Create the computes for the leafs

```yaml
apiVersion: osp-director.openstack.org/v1beta1
kind: OpenStackBaremetalSet
metadata:
  name: computeleaf1
  namespace: openstack
spec:
  # How many nodes to provision
  count: 1
  # The image to install on the provisioned nodes
  baseImageUrl: http://192.168.111.1/images/rhel-guest-image-8.4-1168.x86_64.qcow2
  provisionServerName: openstack
  # The secret containing the SSH pub key to place on the provisioned nodes
  deploymentSSHSecret: osp-controlplane-ssh-keys
  # The interface on the nodes that will be assigned an IP from the mgmtCidr
  ctlplaneInterface: enp7s0
  # Networks to associate with this host
  networks:
        - ctlplane
        - internal_api_leaf1
        - external
        - tenant_leaf1
        - storage_leaf1
  roleName: ComputeLeaf1
  passwordSecret: userpassword
```

```yaml
apiVersion: osp-director.openstack.org/v1beta1
kind: OpenStackBaremetalSet
metadata:
  name: computeleaf2
  namespace: openstack
spec:
  # How many nodes to provision
  count: 1
  # The image to install on the provisioned nodes
  baseImageUrl: http://192.168.111.1/images/rhel-guest-image-8.4-1168.x86_64.qcow2
  provisionServerName: openstack
  # The secret containing the SSH pub key to place on the provisioned nodes
  deploymentSSHSecret: osp-controlplane-ssh-keys
  # The interface on the nodes that will be assigned an IP from the mgmtCidr
  ctlplaneInterface: enp7s0
  # Networks to associate with this host
  networks:
        - ctlplane
        - internal_api_leaf2
        - external
        - tenant_leaf2
        - storage_leaf2
  roleName: ComputeLeaf2
  passwordSecret: userpassword
```

### Render playbooks and apply them

Define an OpenStackConfigGenerator to generate ansible playbooks for the OSP cluster deployment as in `Deploying OpenStack once you have the OSP Director Operator installed` and specify the roles generated roles file.

### Run the software deployment

As described before in `Run the software deployment` check, apply, register the overcloud nodes to required repositories  and run the sofware deployment from inside the openstackclient pod.

# Backup / Restore

## Operator

OSP-D Operator provides an API to create and restore backups of its current CR, ConfigMap and Secret configurations.  This API consists of two CRDs:

* `OpenStackBackupRequest`
* `OpenStackBackup`

The `OpenStackBackupRequest` CRD is used to initiate the creation or restoration of a backup, while the `OpenStackBackup` CRD is used to actually store the CR, ConfigMap and Secret data that belongs to the operator.
This allows for several benefits:

* By representing a backup as a single `OpenStackBackup` CR, the user does not have to manually export/import each piece of the operator's configuration
* The operator is aware of the state of all resources, and will do its best to not backup configuration that is currently in an incomplete or bad state
* The operator knows exactly which CRs, ConfigMaps and Secrets it needs to create a complete backup
* In the near future, the operator will be further extended to automatically create these backups if so desired

### Backup Process

1. To initiate the creation of a new `OpenStackBackup`, create an `OpenStackBackupRequest` with `mode` set to `save` in its spec.  For example:

```yaml
apiVersion: osp-director.openstack.org/v1beta1
kind: OpenStackBackupRequest
metadata:
  name: openstackbackupsave
  namespace: openstack
spec:
  mode: save
  additionalConfigMaps: []
  additionalSecrets: []
```

Spec fields are as follows:
* The `mode: save` indicates that this is a request to create a backup.
* The `additionalConfigMaps` and `additionalSecrets` lists may be used to include supplemental ConfigMaps and Secrets of which the operator is otherwise unaware (i.e. ConfigMaps and Secrets manually created for certain purposes).  
  As noted above, however, the operator will still attempt to include all ConfigMaps and Secrets associated with the various CRs (`OpenStackControlPlane`, `OpenStackBaremetalSet`, etc) in the namespace, without requiring the user 
  to include them in these additional lists.

2. Once the `OpenStackBackupRequest` has been created, monitor its status:

```bash
oc get -n openstack osbackuprequest openstackbackupsave
```

Something like this should appear:

```bash
NAME                     OPERATION   SOURCE   STATUS      COMPLETION TIMESTAMP
openstackbackupsave      save                 Quiescing         
```

The `Quiescing` state indicates that the operator is waiting for provisioning state of all OSP-D operator CRs to reach their "finished" equivalent.  The time required for this will vary based on the quantity 
of OSP-D operator CRs and the happenstance of their current provisioning state.  NOTE: It is possible that the operator will never fully quiesce due to errors and/or "waiting" states in existing CRs.  To see 
which CRDs/CRs are preventing quiesence, investigate the operator logs.  For example:

```bash
oc logs <OSP-D operator pod> -c manager -f

...

2022-01-11T18:26:15.180Z	INFO	controllers.OpenStackBackupRequest	Quiesce for save for OpenStackBackupRequest openstackbackupsave is waiting for: [OpenStackBaremetalSet: compute, OpenStackControlPlane: overcloud, OpenStackVMSet: controller]
```

If the `OpenStackBackupRequest` enters the `Error` state, look at its full contents to see the error that was encountered (`oc get -n openstack openstackbackuprequest <name> -o yaml`).

3. When the `OpenStackBackupRequest` has been honored by creating and saving an `OpenStackBackup` representing the current OSP-D operator configuration, it will enter the `Saved` state.  For example:

```bash
oc get -n openstack osbackuprequest

NAME                     OPERATION   SOURCE   STATUS   COMPLETION TIMESTAMP
openstackbackupsave      save                 Saved    2022-01-11T19:12:58Z
```

The associated `OpenStackBackup` will have been created as well.  For example:

```bash
oc get -n openstack osbackup

NAME                                AGE
openstackbackupsave-1641928378      6m7s
```

### Restore Process

1. To initiate the restoration of an `OpenStackBackup`, create an `OpenStackBackupRequest` with `mode` set to `restore` in its spec.  For example:

```yaml
apiVersion: osp-director.openstack.org/v1beta1
kind: OpenStackBackupRequest
metadata:
  name: openstackbackuprestore
  namespace: openstack
spec:
  mode: restore
  restoreSource: openstackbackupsave-1641928378
```

Spec fields are as follows:
* The `mode: restore` indicates that this is a request to restore an existing `OpenStackBackup`.  
* The `restoreSource` indicates which `OpenStackBackup` should be restored.

With `mode` set to `restore`, the OSP-D operator will take the contents of the `restoreSource` `OpenStackBackup` and attempt to apply them against the _existing_ CRs, ConfigMaps and Secrets currently
present within the namespace.  Thus it will overwrite any existing OSP-D operator resources in the namespace with the same names as those in the `OpenStackBackup`, and will create new resources for
those not currently found in the namespace.  If desired, `mode` can be set to `cleanRestore` to completely wipe the existing OSP-D operator resources within the namespace before attempting a 
restoration, such that all resources within the `OpenStackBackup` are created completely anew.

2. Once the `OpenStackBackupRequest` has been created, monitor its status:

```bash
oc get -n openstack osbackuprequest openstackbackuprestore
```

Something like this should appear to indicate that all resources from the `OpenStackBackup` are being applied against the cluster:

```bash
NAME                     OPERATION  SOURCE                           STATUS     COMPLETION TIMESTAMP
openstackbackuprestore   restore    openstackbackupsave-1641928378   Loading   
```

Then, once all resources have been loaded, the operator will begin reconciling to attempt to provision all resources:

```bash
NAME                     OPERATION  SOURCE                           STATUS       COMPLETION TIMESTAMP
openstackbackuprestore   restore    openstackbackupsave-1641928378   Reconciling   
```

If the `OpenStackBackupRequest` enters the `Error` state, look at its full contents to see the error that was encountered (`oc get -n openstack openstackbackuprequest <name> -o yaml`).

3. When the `OpenStackBackupRequest` has been honored by fully restoring the `OpenStackBackup`, it will enter the `Restored` state.  For example:

```bash
oc get -n openstack osbackuprequest

NAME                     OPERATION  SOURCE                           STATUS     COMPLETION TIMESTAMP
openstackbackuprestore   restore    openstackbackupsave-1641928378   Restored   2022-01-12T13:48:57Z
```

At this point, all resources contained with the chosen `OpenStackBackup` should be restored and fully provisioned.

# Day2 Operations

## Change resources on virtual machines

If required it is possible to change CPU/RAM of an openstackvmset configured via the openstackcontrolplane. The workflow is as follows:

* change/patch the virtualMachineRole within the virtualMachineRoles list of the openstackcontrolplane CR

E.g. to change the controller virtualMachineRole to have 8 cores and 22GB of RAM:

```bash
oc patch -n openstack osctlplane overcloud --type='json' -p='[{"op": "add", "path": "/spec/virtualMachineRoles/controller/cores", "value": 8 }]'
oc patch -n openstack osctlplane overcloud --type='json' -p='[{"op": "add", "path": "/spec/virtualMachineRoles/controller/memory", "value": 22 }]'

oc get osvmset
NAME         CORES   RAM   DESIRED   READY   STATUS        REASON
controller   8       22    1         1       Provisioned   All requested VirtualMachines have been provisioned
```

* schedule a restart of the virtual machines, one at a time, to get the change reflected inside the virtual machine (**Important** it is required to power off/on the virtual machine). The recommended way is to do a graceful shutdown from inside the virtual machine and use `virtctl start <VM>` to power the VM back on.
