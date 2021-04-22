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

Installation
------------

## Prerequisite:
- OCP 4.6 installed
- OpenShift Virtualization 2.5

## Install the OSP Director Operator
The OSP Director Operator is installed and managed via the OLM [Operator Lifecycle Manager](https://github.com/operator-framework/operator-lifecycle-manager). OLM is installed automatically with your OpenShift installation. To obtain the latest OSP Director Operator snapshot you need to create the appropriate CatalogSource, OperatorGroup, and Subscription to drive the installation with OLM:

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

A good example of ConfigMaps that can be used can be found in our [dev-tools](https://github.com/openstack-k8s-operators/osp-director-dev-tools) GitHub project. Direct links for each the sample files can be found below:

-[Net-Config files](https://github.com/openstack-k8s-operators/osp-director-dev-tools/tree/master/ansible/files/osp/net_config)

-[Net-Config environment](https://github.com/openstack-k8s-operators/osp-director-dev-tools/blob/master/ansible/files/osp/tripleo_deploy/flat/network-environment.yaml)

-[Tripleo Deploy custom files](https://github.com/openstack-k8s-operators/osp-director-dev-tools/tree/master/ansible/templates/osp/tripleo_deploy) (NOTE: these are Ansible templates and need to have variables replaced to be used directly!)

Once you customize the above template/examples for your environment you can create configmaps for both the 'tripleo-deploy-config-custom' and 'tripleo-net-config' ConfigMaps by using these example commands on the files containing each respective configmap type (one directory for each type of configmap):

```bash
# create the configmap for tripleo-deploy-config-custom
oc create configmap -n openstack tripleo-deploy-config-custom --from-file=tripleo-deploy-config-custom/ --dry-run=client -o yaml | oc apply -f -

# create the configmap for netconfig
oc create configmap -n openstack tripleo-net-config --from-file=net-config/ --dry-run=client -o yaml | oc apply -f -
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
      baseImageURL: http://download.devel.redhat.com/brewroot/packages/rhel-guest-image/8.3/417/images/rhel-guest-image-8.3-417.x86_64.qcow2
      storageClass: host-nfs-storageclass
```

If you write the above YAML into a file called openstackcontrolplane.yaml you can create the OpenStackControlPlane via this command:

```bash
oc create -f openstackcontrolplane.yaml
```

5) Define an OpenStackBaremetalSet to scale out OSP Compute hosts. The OpenStackBaremetal resource can be used to define and scale Compute resources and optionally be used to define and scale out baremetal hosts for other types of TripleO roles. The example below defines a single Compute host to be created.

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
  baseImageUrl: http://172.22.0.1/images/{{ osp_controller_base_image_url | basename }}
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

5) Wait for any resources virtual machine or baremetal host resources to finish provisioning. FIXME: need to add status info to our CR's so that we can add wait conditions on these resources.

6) Login to the 'openstackclient' pod and deploy the OSP software via TripleO commands. At this point all baremetal and virtualmachine resources have been provisioned within the OCP cluster. Additionally an automatically generated k8s ConfigMap called 'tripleo-deploy-config' contains all the necissary IP and hostname information required for TripleO's Heat to deploy the cluster. Run the following commands in your cluster to deploy a sample TripleO cluster

```bash
oc rsh openstackclient
cd /home/cloud-admin
bash tripleo-deploy.sh
```

This will create an ephemeral heat stack which is used to export Ansible playbooks. These Ansible playbooks are then used to deploy OSP on the configured hosts.
