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
oc delete openstacknetconfig --all -n openstack
oc delete osconfiggenerator --all -n openstack
oc delete nncp -n openstack --all
oc delete secret -n openstack userpassword --ignore-not-found
oc delete secret -n openstack osp-controlplane-ssh-keys osp-baremetalset-ssh-keys --ignore-not-found
oc delete events --all -n openstack

# Free any dead PVs
for i in $(oc get pv | egrep "Failed|Released" | awk {'print $1'}); do oc patch pv $i --type='json' -p='[{"op": "remove", "path": "/spec/claimRef"}]'; done
