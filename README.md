OSP Director Operator
=====================

Description
-----------


The OSP Director Operator creates a set of Custom Resource Definitions on top of OpenShift to manage resources normally created by the TripleO's Undercloud. These CRDs are split into two types for hardware provisioning and software configuration.


Hardware Provisioning CRDs
--------------------------
- openstackbaremetalset: create sets of baremetal hosts for a specific TripleO role (Compute, Storage, etc.)
- openstackcontrolplane: A CRD used to create the OpenStack control plane and manage associated openstackvmsets
- openstackipset: Contains a set of IPs for a given network and role. Used internally to manage IP addresses.
- openstacknet: Create networks which are used to assign IPs to the vmset and baremetalset resources below
- openstackprovisionservers: used to serve custom images for baremetal provisioning with Metal3
- openstackvmset: create sets of VMs using OpenShift Virtualization for a specific TripleO role (Controller, Database, NetworkController, etc.)

Software Configuration CRDs
---------------------------
- openstackplaybookgenerator: automatically generate Ansible playbooks for deployment when you scale up or make changes to custom ConfigMaps for deployment
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

## Creating a RHEL data volume (optional)

It may be desirable to create a base RHEL data volume prior to deploying OpenStack.  This will greatly increase the speed at which controller VMs are provisioned via OpenShift Virtualization.  Certain OSP Director Operator CRDs allow the user to supply the name of a pre-existing data volume within the cluster for use as the base RHEL image in OpenShift Virtualization virtual machines, rather than providing a remote URL that must be downloaded and converted during provisioning.  The approach to doing this is as follows:

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
    virt-customize -a bms-image.qcow2 --run-command 'sed -i -e "s/^\(kernelopts=.*\)net.ifnames=0 \(.*\)/\1\2/" /boot/grub2/grubenv'
    ```
4) If your local machine cannot resolve hostnames for within the cluster, add the following to your `/etc/hosts`:
    ```
    <cluster ingress VIP>     cdi-uploadproxy-openshift-cnv.apps.<cluster name>.<domain name>
    ```
5) Upload the image to OpenShift Virtualization via `virtctl`:
    ```
    virtctl image-upload dv openstack-base-img -n openstack --size=40Gi --image-path=<local path to image> --storage-class <desired storage class> --insecure
    ```
    For the `storage-class` above, pick one you want to use from those shown in:
    ```
    oc get storageclass
    ```
## Deploying OpenStack once you have the OSP Director Operator installed

1) Define your OpenStackNet custom resource. At least 1 network is required for the ctlplane. Optionally you may define multiple networks to be used with TripleO's network isolation architecture. In addition to IP address assignment the OpenStackNet includes information that is used to define the network configuration policy used to attach any VM's to this network via OpenShift Virtualization. The following is an example of a simple IPv4 ctlplane network which uses linux bridge for its host configuration.
    ```yaml
    apiVersion: osp-director.openstack.org/v1beta1
    kind: OpenStackNet
    metadata:
      name: ctlplane
    spec:
      cidr: 192.168.25.0/24
      allocationStart: 192.168.25.100
      allocationEnd: 192.168.25.250
      gateway: 192.168.25.1
      attachConfiguration:
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
              name: br-osp
              state: up
              type: linux-bridge
    ```

    If you write the above YAML into a file called ctlplane-network.yaml you can create the OpenStackNet via this command:

    ```bash
    oc create -n openstack -f ctlplane-network.yaml
    ```

    To use network isolation using VLAN add the vlan ID to the spec of the network definition
    ```yaml
    apiVersion: osp-director.openstack.org/v1beta1
    kind: OpenStackNet
    metadata:
      name: internalapi
    spec:
      cidr: 172.16.2.0/24
      vlan: 20
      allocationStart: 172.16.2.4
      allocationEnd: 172.16.2.250
      attachConfiguration:
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
    ```

    When using VLAN for network isolation with linux-bridge
    - a Node Network Configuration Policy gets created for the bridge interface specified in the osnet CR, which uses nmstate to configure the bridge on the worker node
    - for each network a Network Attach Definition gets created which defines the Multus CNI plugin configuration. Specifying the vlan ID on the Network Attach Definition enables the bridge vlan-filtering.
    - for each network a dedicated interface gets attached to the virtual machine. Therefore the network template for the OSVMSet is a multi-nic network template

2) Create [ConfigMaps](https://kubernetes.io/docs/concepts/configuration/configmap/) which define any custom Heat environments, Heat templates and custom roles file (name must be `roles_data.yaml`) used for TripleO network configuration. Any adminstrator defined Heat environment files can be provided in the ConfigMap and will be used as a convention in later steps used to create the Heat stack for Overcloud deployment. As a convention each OSP Director Installation will use 2 ConfigMaps named `heat-env-config` and `tripleo-tarball-config` to provide this information. The `heat-env-config` configmap holds all deployment environment files where each file gets added as `-e file.yaml` to the `openstack stack create` command. A good example is:

    - [Tripleo Deploy custom files](https://github.com/openstack-k8s-operators/osp-director-dev-tools/tree/master/ansible/templates/osp/tripleo_deploy)
        **NOTE**: these are Ansible templates and need to have variables replaced to be used directly!
        **NOTE**: all references in the environment files need to be relative to the t-h-t root where the tarball gets extracted!

    A "Tarball Config Map" can be used to provide (binary) tarballs which are extracted in the tripleo-heat-templates when playbooks are generated. Each tarball should contain a directory of files relative to the root of a t-h-t directory. You will want to store things like the following examples in a config map containing custom tarballs:

    - [Net-Config files](https://github.com/openstack-k8s-operators/osp-director-dev-tools/tree/master/ansible/files/osp/net_config).

    - [Net-Config environment](https://github.com/openstack-k8s-operators/osp-director-dev-tools/blob/master/ansible/templates/osp/tripleo_deploy/vlan/network-environment.yaml.j2)

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

        **NOTE**: FIP traffic does not pass to a VLAN tenant network with ML2/OVN and DVR. DVR is enabled by default. If you need VLAN tenant networks with OVN, you can disable DVR. To disable DVR, inlcude the following lines in an environment file:

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
      openStackClientImageURL: quay.io/openstack-k8s-operators/tripleo-deploy:16.2_20210309.1
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
          diskSize: 40
          baseImageVolumeName: openstack-base-img
          storageClass: host-nfs-storageclass
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
      ctlplaneInterface: enp2s0
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

6) (optional) Create roles file
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

7) Define an OpenStackPlaybookGenerator to generate ansible playbooks for the OSP cluster deployment.
    ```yaml
    apiVersion: osp-director.openstack.org/v1beta1
    kind: OpenStackPlaybookGenerator
    metadata:
      name: default
      namespace: openstack
    spec:
      imageURL: quay.io/openstack-k8s-operators/tripleo-deploy:16.2_20210521.1
      gitSecret: git-secret
      heatEnvConfigMap: heat-env-config
      tarballConfigMap: tripleo-tarball-config
      # (optional) for debugging it is possible to set the interactive mode.
      # In this mode the playbooks won't get rendered automatically. Just the environment to start the rendering gets created
      # interactive: true
    ```

    If you write the above YAML into a file called generator.yaml you can create the OpenStackPlaybookGenerator via this command:

    ```bash
    oc create -f generator.yaml
    ```

    The osplaybookgenerator created above will automatically generate playbooks any time you scale or modify the ConfigMaps for your OSP deployment. Generating these playbooks takes several minutes. You can monitor the osplaybookgenerator's status condition for it to finish.

8) Login to the 'openstackclient' pod and deploy the OSP software via the rendered ansible playbooks. At this point all baremetal and virtualmachine resources have been provisioned within the OCP cluster.

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

    b) register the overcloud systems to required channels

    The command in step a) to accept the current available rendered playbooks contain the latest inventory file of the overcloud and can be used to register the overcloud nodes to the required repositories for deployment. Use the procedure as described in [5.9. Running Ansible-based registration manually](https://access.redhat.com/documentation/en-us/red_hat_openstack_platform/16.1/html-single/advanced_overcloud_customization/index#running-ansible-based-registration-manually-portal) do do so.

    TODO: update link to 16.2 release when available

    ```bash
    oc rsh openstackclient
    bash
    cd /home/cloud-admin

    <create the ansible playbook for the overcloud nodes - e.g. rhsm.yaml>

    # register the overcloud nodes to required repositories
    ansible-playpook -i /home/cloud-admin/playbooks/tripleo-ansible/inventory.yaml ./rhsm.yaml
    ```

    c) run ansible driven OpenStack deployment

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
  ctlplaneInterface: enp8s0
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

* Define an OpenStackPlaybookGenerator to generate ansible playbooks for the OSP cluster deployment as in `Deploying OpenStack once you have the OSP Director Operator installed` and specify the roles generated roles file.

**NOTE**: Make sure to use `quay.io/openstack-k8s-operators/rhosp16-openstack-tripleoclient:16.2_20210521.1` or later for the osplaybookgenerator `imageURL`.

## Run the software deployment

* Wait for the OpenStackPlaybookGenerator to finish the playbook rendering job

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

b) register the overcloud systems to required channels

The command in step a) to accept the current available rendered playbooks contain the latest inventory file of the overcloud and can be used to register the overcloud nodes to the required repositories for deployment. Use the procedure as described in [5.9. Running Ansible-based registration manually](https://access.redhat.com/documentation/en-us/red_hat_openstack_platform/16.1/html-single/advanced_overcloud_customization/index#running-ansible-based-registration-manually-portal) do do so.

**NOTE**: update link to 16.2 release when available

```bash
oc rsh openstackclient
bash
cd /home/cloud-admin

<create the ansible playbook for the overcloud nodes - e.g. rhsm.yaml>

# register the overcloud nodes to required repositories
ansible-playpook -i /home/cloud-admin/playbooks/tripleo-ansible/inventory.yaml ./rhsm.yaml
```

c) Install the pre-requisites on overcloud systems for ceph-ansible

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
* the corresponding OSIPSet for the node gets deleted
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
* the corresponding OSIPSet for the node got deleted
* the IPreservation entry in the OSNet resources is flagged as deleted

This results in the following behavior
* the IP is not free for use for another role
* if a new node gets scaled into the same role it will reuse the hostnames starting with lowest id suffix (if there are multiple) _and_ corresponding IP reservation
* if the OSBaremetalset or OSVMset resource gets deleted, all IP reservations for the role get deleted and are free to be used by other nodes

## (optional) Cleanup OpenStack resources

If the VM did host any OSP service which should be removed, delete the service using the corresponding openstack command.
