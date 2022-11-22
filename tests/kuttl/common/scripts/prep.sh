#!/bin/bash

#
# Common script for use at the start of tests to clear pre-existing objects
#

oc delete openstackbackuprequest --all -n openstack
oc delete openstackbackup --all -n openstack
oc delete openstackcontrolplane overcloud -n openstack --ignore-not-found
oc delete openstackvmset --all -n openstack
oc delete openstackclient --all -n openstack
oc delete openstackbaremetalset --all -n openstack
oc delete openstackprovisionserver --all -n openstack

# We will need to restart the Metal3 pod because the provisioning network interface 
# will no longer be available (OSNetConfig will be deleted), but we shouldn't do that 
# if any BMH is currently provisioning/deprovisioning.  Destroying the Metal3 pod 
# (or the provisioning interface NNCP) while a BMH is in such a transitional state 
# can cause the BMH to become stuck in its current state without manual intervention.
echo "Waiting for all Metal3 BMHs to settle..."
while true; do
    ready="true"
    for i in $(oc get bmh -A | grep -v STATE | grep -v unmanaged | awk {'print $3'}); do
        if [ "$i" != "available" ] && [ "$i" != "ready" ]; then
            ready="false"
        fi
    done

    if [ "$ready" = "true" ]; then
        break
    fi

    sleep 5
done

oc delete openstacknetconfig --all -n openstack
oc delete osconfiggenerator --all -n openstack
oc delete nncp -n openstack --all
oc delete secret -n openstack userpassword --ignore-not-found
oc delete secret -n openstack osp-controlplane-ssh-keys osp-baremetalset-ssh-keys --ignore-not-found
oc delete events --all -n openstack
oc patch provisioning provisioning-configuration --type='json' -p='[{"op": "replace", "path": "/spec/provisioningInterface", "value": "enp1s0"}]'
# This will cause the existing Metal3 pod to terminate, but then the baremetal cluster operator
# will reset the replicas to 1 and create a new pod
oc scale deployment metal3 -n openshift-machine-api --replicas=0

# Free any dead PVs
for i in $(oc get pv | grep -E "Failed|Released" | awk {'print $1'}); do oc patch pv "$i" --type='json' -p='[{"op": "remove", "path": "/spec/claimRef"}]'; done
