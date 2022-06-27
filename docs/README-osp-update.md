# OSP 16.2 Minor Update Procedure

OSP minor updates are very similar to the normal update procedures described in [Keeping Red Hat OpenStack Platform Updated](https://access.redhat.com/documentation/en-us/red_hat_openstack_platform/16.2/html-single/keeping_red_hat_openstack_platform_updated/index). This document assumes the reader is already familiar with this process and just outlines the significant differences.

## Preparing for a minor update

### Overcloud RPM repository and container image update

Refer to Sections 1.2/1.3/1.4/1.5/1.6 in [Keeping Red Hat OpenStack Platform Updated](https://access.redhat.com/documentation/en-us/red_hat_openstack_platform/16.2/html-single/keeping_red_hat_openstack_platform_updated/index).

Ansible playbooks are run via the openstackclient pod instead of the undercloud . This already contains an ansible inventory file for the overcloud nodes.

To run a playbook on the pod:
```
oc cp rhsm.yaml openstackclient:/home/cloud-admin/rhsm.yaml
oc rsh openstackclient
$ ansible-playbook -i /home/cloud-admin/ctlplane-ansible-inventory /home/cloud-admin/rhsm.yaml
```

The updated RhsmVars and ContainerImagePrepare heat parameter_defaults should be updated in the custom heat environment configmap.

### Disable overcloud fencing

Refer to Sections 1.8 in [Keeping Red Hat OpenStack Platform Updated](https://access.redhat.com/documentation/en-us/red_hat_openstack_platform/16.2/html-single/keeping_red_hat_openstack_platform_updated/index).

```
oc rsh openstackclient
$ ssh controller-0.ctlplane "sudo pcs property set stonith-enabled=false"
```

## Updating the ~~Undercloud~~ openstackclient pod

There is no undercloud to update. Instead the openstackclient pod image needs to be updated to the correct version.

Update the following values in the CSV to the new OSP minor version image urls:
OPENSTACKCLIENT_IMAGE_URL_DEFAULT
HEAT_API_IMAGE_URL_DEFAULT
HEAT_ENGINE_IMAGE_URL_DEFAULT
MARIADB_IMAGE_URL_DEFAULT
RABBITMQ_IMAGE_URL_DEFAULT

```
oc project openstack
oc edit csv
```

Delete any existing ephemeral heat instances. New instances will be created when required using the new image:

```
oc delete openstackephemeralheat --all
```

Remove the current imageURL from openstackclient custom resource to update the pod to the new image:
```
oc get openstackclient openstackclient -o json \
        | jq 'del( .spec.imageURL )' \
        | oc replace -f -
```

## Updating the overcloud

Refer to Chapter 3 in [Keeping Red Hat OpenStack Platform Updated](https://access.redhat.com/documentation/en-us/red_hat_openstack_platform/16.2/html-single/keeping_red_hat_openstack_platform_updated/index).

The process is almost identical. The significant difference is that the `openstack overcloud update` commands must be replace with the equivalent OSP director operator config generator and deployment resources.

### Overcloud update preparation

Modify the existing heat parameter config map or create a new heat parameter config map for the duration of the update to disable fencing:
```
parameter_defaults:
  EnableFencing: false
```

Create an openstack-config-generator resource, including the `lifecycle/update-prepare.yaml` heat environment file:
```
$ cat <<EOF > osconfiggenerator-update-prepare.yaml
apiVersion: osp-director.openstack.org/v1beta1
kind: OpenStackConfigGenerator
metadata:
  name: "update"
  namespace: openstack
spec:
  gitSecret: git-secret
  heatEnvs:
    - lifecycle/update-prepare.yaml
  heatEnvConfigMap: heat-env-config-update
  tarballConfigMap: tripleo-tarball-config-update
EOF
$ oc apply -f osconfiggenerator-update-prepare.yaml
```

### Container image preparation

The `openstack overcloud external-update run --tags container_image_prepare` command is replaced with an osdeploy job:

```
$ cat <<EOF > osdeploy-container-image-prepare.yaml
apiVersion: osp-director.openstack.org/v1beta1
kind: OpenStackDeploy
metadata:
  name: container_image_prepare
spec:
  configVersion: <config_version>
  configGenerator: update
  mode: external-update
  advancedSettings:
    tags:
      - container_image_prepare
EOF
$ oc apply -f osdeploy-container-image-prepare.yaml
```

### Updating Controller nodes

The `openstack overcloud update run --limit Controller` command is replaced with an osdeploy job:

```
$ cat <<EOF > osdeploy-controller-update.yaml
apiVersion: osp-director.openstack.org/v1beta1
kind: OpenStackDeploy
metadata:
  name: controller_update
spec:
  configVersion: <config_version>
  configGenerator: update
  mode: update
  advancedSettings:
    limit: Controller

EOF
$ oc apply -f osdeploy-controller-update.yaml
```

Etc... etc... for all of the required `openstack overcloud update` commands.

### Finalizing the update

The finalize the update re-enable fencing in the heat parameters configmap, regenerate the the default config (ensure `lifecycle/update-prepare.yaml` is not included in heatEnvs), and run the default deployment job.

And update related configgenerators/configversion/deployment resources can be removed.

## Rebooting the Overcloud

Refer to Sections 4 in [Keeping Red Hat OpenStack Platform Updated](https://access.redhat.com/documentation/en-us/red_hat_openstack_platform/16.2/html-single/keeping_red_hat_openstack_platform_updated/index).