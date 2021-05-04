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
- openstackclient: creates a pod used to run TripleO deployment commands
- openstackplaybookgenerator: automatically generate Ansible playbooks for deployment when you scale up or make changes to custom ConfigMaps for deployment
- openstackclient: creates a pod used to run TripleO deployment commands

Installation
------------

## Prerequisite:
- OCP 4.6 installed
- OpenShift Virtualization 2.6+

## Install the OSP Director Operator
The OSP Director Operator is installed and managed via the OLM [Operator Lifecycle Manager](https://github.com/operator-framework/operator-lifecycle-manager). OLM is installed automatically with your OpenShift installation. To obtain the latest OSP Director Operator snapshot you need to create the appropriate CatalogSource, OperatorGroup, and Subscription to drive the installation with OLM:

### Create the "openstack" Namespace
```bash
oc create project openstack
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
      value: openstack
  source: osp-director-operator-index
  sourceNamespace: openstack
  name: osp-director-operator
  startingCSV: osp-director-operator.v0.0.1
  channel: alpha
```

We have a script to automate the installation here with OLM for a specific tag: [script to automate the installation](https://github.com/openstack-k8s-operators/osp-director-operator/blob/master/scripts/deploy-with-olm.sh)

NOTE: At some point in the future we may integrate automatically into OperatorHub so that OSP Director Operator is available automatically in your OCP installations default OLM Catalog sources.

## Creating a RHEL data volume (optional)

It may be desirable to create a base RHEL data volume prior to deploying OpenStack.  This will greatly increase the speed at which controller VMs are provisioned via OpenShift Virtualization.  Certain OSP Director Operator CRDs allow the user to supply the name of a pre-existing data volume within the cluster for use as the base RHEL image in OpenShift Virtualization virtual machines, rather than providing a remote URL that must be downloaded and converted during provisioning.  The approach to doing this is as follows:

1) Install the KubeVirt CLI tool, `virtctl`:
```
sudo subscription-manager repos --enable=cnv-2.6-for-rhel-8-x86_64-rpms
sudo dnf install -y kubevirt-virtctl
```
2) Download the RHEL QCOW2 you wish to use.  For example:
```
curl -O http://download.eng.bos.redhat.com/brewroot/packages/rhel-guest-image/8.4/916/images/rhel-guest-image-8.4-916.x86_64.qcow2
```
3) If your local machine cannot resolve hostnames for within the cluster, add the following to your `/etc/hosts`:
```
<cluster ingress VIP>     cdi-uploadproxy-openshift-cnv.apps.<cluster name>.<domain name>
```
4) Upload the image to OpenShift Virtualization via `virtctl`:
```
virtctl image-upload dv openstack-base-img -n openstack --size=40Gi --image-path=<local path to image> --storage-class <desired storage class> --insecure
```
For the `storage-class` above, pick one you want to use from those shown in:
```
oc get storageclass
```
5) Now, when you create the `OpenStackControlPlane` below (as well as for individual `OpenStackVmSet`s), you may substitute `baseImageVolumeName` for `baseImageUrl` in the CR:
```yaml
...
spec:
...
  baseImageVolumeName: openstack-base-img
...
```
This will cause OpenShift Virtualization to clone that base image volume and use it for the VM(s), which is significantly faster than downloading and converting it on-the-fly.

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
    bridgeName: br-osp
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

2) Create a [ConfigMaps](https://kubernetes.io/docs/concepts/configuration/configmap/) which define any custom Heat environments and Heat templates used for TripleO network configuration. Any adminstrator defined Heat environment files can be provided in the ConfigMap and will be used as a convention in later steps used to create the Heat stack for Overcloud deployment. As a convention each OSP Director Installation will use 2 ConfigMaps named 'tripleo-deploy-config-custom' and 'tripleo-net-config' to provide this information.

A good example of ConfigMaps that can be used can be found in our [dev-tools](https://github.com/openstack-k8s-operators/osp-director-dev-tools) GitHub project.

A "Tarball Config Map" can be used  to provide (binary) tarballs which are extracted in the tripleo-heat-templates when playbooks are generated. You will want to store things like the following examples in a config map containing custom tarballs:
-[Net-Config files](https://github.com/openstack-k8s-operators/osp-director-dev-tools/tree/master/ansible/files/osp/net_config).

-[Net-Config environment](https://github.com/openstack-k8s-operators/osp-director-dev-tools/blob/master/ansible/files/osp/tripleo_deploy/flat/network-environment.yaml)

-[Tripleo Deploy custom files](https://github.com/openstack-k8s-operators/osp-director-dev-tools/tree/master/ansible/templates/osp/tripleo_deploy) (NOTE: these are Ansible templates and need to have variables replaced to be used directly!)

Once you customize the above template/examples for your environment you can create configmaps for both the 'tripleo-deploy-config-custom' and 'tripleo-net-config'(tarballs) ConfigMaps by using these example commands on the files containing each respective configmap type (one directory for each type of configmap):

```bash
# create the configmap for tripleo-deploy-config-custom
oc create configmap -n openstack tripleo-deploy-config-custom --from-file=tripleo-deploy-config-custom/ --dry-run=client -o yaml | oc apply -f -

# create the configmap containing a tarball of t-h-t network config files. NOTE: these files may overwrite default t-h-t files so keep this in mind when naming them.
cd <dir with net config files>
tar -cvzf net-config.tar.gz *.yaml
oc create configmap -n openstack tripleo-net-config --from-file=net-config.tar.gz
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

4) Define your OpenStackControlPlane custom resource. The OpenStackControlPlane custom resource provides a cental place to create and scale VMs used for the OSP Controllers along with any additional vmsets for your deployment. At least 1 Controller VM is required for a basic demo installation and per OSP High Availability guidelines 3 Controller VMs are recommended.

Note: If the rhel-guest-image is used as base to deploy the OpenStackControlPlane virtual machines, make sure to remove the net.ifnames=0 kernel parameter form the image to have the biosdev network interface naming. This can be done like:

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
  openStackClientStorageClass: host-nfs-storageclass
  passwordSecret: userpassword
  virtualMachineRoles:
    controller:
      roleName: Controller
      roleCount: 3
      networks:
        - ctlplane
      cores: 6
      memory: 12
      diskSize: 40
      baseImageURL: http://host/images/rhel-guest-image-8.3-417.x86_64.qcow2
      storageClass: host-nfs-storageclass
```

If you write the above YAML into a file called openstackcontrolplane.yaml you can create the OpenStackControlPlane via this command:

```bash
oc create -f openstackcontrolplane.yaml
```

5) Define an OpenStackBaremetalSet to scale out OSP Compute hosts. The OpenStackBaremetal resource can be used to define and scale Compute resources and optionally be used to define and scale out baremetal hosts for other types of TripleO roles. The example below defines a single Compute host to be created.

Note: If the rhel-guest-image is used as base to deploy the OpenStackBaremetalSet compute nodes, make sure to remove the net.ifnames=0 kernel parameter form the image to have the biosdev network interface naming. This can be done like:

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
  replicas: {{ osp_compute_count }}
  # The image to install on the provisioned nodes. NOTE: needs to be accessible on the OpenShift Metal3 provisioning network.
  baseImageUrl: http://host/images/rhel-image-8.3-417.x86_64.qcow2
  # NOTE: these are automatically created via the OpenStackControlplane CR above
  deploymentSSHSecret: osp-controlplane-ssh-keys
  # The interface on the nodes that will be assigned an IP from the mgmtCidr
  ctlplaneInterface: eth2
  # Networks to associate with this host
  networks:
    - ctlplane
  roleName: Compute
  passwordSecret: userpassword
```

If you write the above YAML into a file called compute.yaml you can create the OpenStackBaremetalSet via this command:

```bash
oc create -f compute.yaml
```

6) Define an OpenStackPlaybookGenerator to generate ansible playbooks for the OSP cluster deployment.

```yaml
apiVersion: osp-director.openstack.org/v1beta1
kind: OpenStackPlaybookGenerator
metadata:
  name: default
  namespace: openstack
spec:
  imageURL: quay.io/openstack-k8s-operators/tripleo-deploy:16.2_20210309.1
  openstackClientName: openstackclient
  heatEnvConfigMap: tripleo-deploy-config-custom
  tarballConfigMap: tripleo-net-config
```

If you write the above YAML into a file called compute.yaml you can create the OpenStackBaremetalSet via this command:

```bash
oc create -f generator.yaml
```

The osplaybookgenerator created above will automatically generate playbooks any time you scale or modify the ConfigMaps for your OSP deployment. Generating these playbooks takes several minutes. You can monitor the osplaybookgenerator's status condition for it to finish.

7) Wait for any resources virtual machine or baremetal host resources to finish provisioning. FIXME: need to add status info to our CR's so that we can add wait conditions on these resources.

8) Login to the 'openstackclient' pod and deploy the OSP software via TripleO commands. At this point all baremetal and virtualmachine resources have been provisioned within the OCP cluster. Additionally an automatically generated k8s ConfigMap called 'tripleo-deploy-config' contains all the necissary IP and hostname information required for TripleO's Heat to deploy the cluster. Run the following commands in your cluster to deploy a sample TripleO cluster

```bash
oc rsh openstackclient
bash
cd /home/cloud-admin
unset OS_CLOUD
openstack overcloud roles generate -o /home/cloud-admin/roles_data.yaml Controller Compute
cp tripleo-deploy.sh tripleo-deploy-custom.sh
sed -i 's/ROLESFILE/~\/roles_data.yaml/' tripleo-deploy-custom.sh
# render ansible playbooks
./tripleo-deploy-custom.sh -r
# run ansible driven OpenStack deployment
./tripleo-deploy-custom.sh -p
```

This will create an ephemeral heat stack which is used to export Ansible playbooks. These Ansible playbooks are then used to deploy OSP on the configured hosts.


Deploy Ceph via tripleo using ComputeHCI
----------------------------------------

It is possible to deploy tripleo's Hyper-Converged Infrastructure where compute nodes also act as Ceph OSD nodes.
The workflow to install Ceph via tripleo would be:

## Control Plane

Until ceph-ansible gets added to the default openstackclient container image, make sure to use `quay.io/openstack-k8s-operators/tripleo-deploy:ceph-ansible`
as the `openStackClientImageURL` in the openstackcontrolplane CR.

## Baremetalset

Have compute nodes with extra disks to be used as OSDs and create a baremetalset for the ComputeHCI role which has
the storagemgmt network in addition to the default compute networks and the `IsHCI` parameter set to true.

Note: If the rhel-guest-image is used as base to deploy the OpenStackBaremetalSet compute nodes, make sure to remove the net.ifnames=0 kernel parameter form the image to have the biosdev network interface naming. This can be done like:

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
  baseImageUrl: http://host/images/rhel-image-8.3-417.x86_64.qcow2
  provisionServerName: openstack
  # The secret containing the SSH pub key to place on the provisioned nodes
  deploymentSSHSecret: osp-controlplane-ssh-keys
  # The interface on the nodes that will be assigned an IP from the mgmtCidr
  ctlplaneInterface: eth2
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

* update the Net-Config to have the storagemgmt network for the ComputeHCI network config template
* add Ceph related deployment parameters to the Tripleo Deploy custom configMap, e.g.

```yaml
parameter_defaults:
  # needed for now because of the repo used to create tripleo-deploy image
  CephAnsibleRepo: "rhelosp-ceph-4-tools"
  CephAnsiblePlaybookVerbosity: 3
  CinderEnableIscsiBackend: false
  CinderEnableRbdBackend: true
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
    journal_size: 512
  CephAnsibleExtraConfig:
    is_hci: true
  CephConfigOverrides:
    rgw_swift_enforce_content_length: true
    rgw_swift_versioning_enabled: true
```

Once you customize the above template/examples for your environment you can create/update configmaps for both the 'tripleo-deploy-config-custom' and 'tripleo-net-config' ConfigMaps by using these example commands on the files containing each respective configmap type (one directory for each type of configmap):

```bash
# create the configmap for tripleo-deploy-config-custom
oc create configmap -n openstack tripleo-deploy-config-custom --from-file=tripleo-deploy-config-custom/ --dry-run -o yaml | oc apply -f -

# create the configmap for netconfig
oc create configmap -n openstack tripleo-net-config --from-file=net-config/ --dry-run -o yaml | oc apply -f -
```

## Render playbooks and apply them

* In openstackclient create the roles file for the overcloud, like Controller + ComputeHCI

```bash
unset OS_CLOUD
openstack overcloud roles generate -o /home/cloud-admin/roles_data_hci.yaml Controller ComputeHCI
```

* create the tripleo-deploy-custom.sh deployment script and update to use the custom roles file


```bash
cd
cp tripleo-deploy.sh tripleo-deploy-custom.sh
sed -i 's/ROLESFILE/~\/roles_data_hci.yaml/' tripleo-deploy-custom.sh
```

* add the `ceph-ansible.yaml` environment file and to the `openstack tripleo deploy` command

```bash
     -e /usr/share/openstack-tripleo-heat-templates/environments/docker-ha.yaml \
+    -e /usr/share/openstack-tripleo-heat-templates/environments/ceph-ansible/ceph-ansible.yaml \
     -e ~/config/deployed-server-map.yaml \
```

* for now add skipping opendev-validation-ceph when run ansible playbooks

```bash
   time ansible-playbook -i inventory.yaml \
     --private-key /home/cloud-admin/.ssh/id_rsa \
+    --skip-tags opendev-validation-ceph \
     --become deploy_steps_playbook.yaml
```

* render the ansible playbooks

```bash
./tripleo-deploy-custom.sh -r
```

* Install the pre-requisites on overcloud systems for ceph-ansible

```bash
ansible -i tripleo-deploy/overcloud-ansible/inventory.yaml overcloud -a "sudo sudo dnf -y install python3 lvm2"
```

* Run the playbooks

```bash
./tripleo-deploy-custom.sh -p
```
