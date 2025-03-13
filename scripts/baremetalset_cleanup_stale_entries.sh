#!/bin/bash
# USAGE: ./script.sh <ROLE name - case sensitive> <list of compute names to remove>
#        e.g. ./script.sh Compute compute-2 compute-3
set -eu -o pipefail

function cleanup()
{
    #
    # Stop the kube proxy
    #
    kill $PROXY_PID
}

function usage()
{
    echo "Usage: $0 <ROLE name - case sensitive> <list of compute names to remove>"
    echo "  e.g. $0 Compute compute-2 compute-3"
    exit 1
}

if [  $# -le 1 ]; then
    usage
fi

ROLE=$1
COMPUTES="${@:2}"

if [ -z "$ROLE" ]; then
    usage
fi

if [ -z "$COMPUTES" ]; then
    usage
fi

#
# Disable the operator while updating the status
#
CSV_NAME=$(oc get csv -n openstack  --output=jsonpath={.items..metadata.name})
oc patch -n openstack csv/$CSV_NAME --type='json' \
--patch='[{"op":"replace", "path":"/spec/install/spec/deployments/0/spec/replicas", "value":0}]'

oc wait pod -l control-plane=controller-manager --for=delete -n openstack --timeout=60s

#
# Start kube proxy to access the api
#
oc proxy &
PROXY_PID=$!
trap cleanup EXIT

#
# Handle status updates
#
for COMPUTE in $COMPUTES; do
    #
    # Remove compute reservations from OpenStackBaremetalset status
    #
    EXIST=$(oc get osbms "${ROLE,,}" -n openstack -o json | jq ".status.baremetalHosts.\"$COMPUTE\" | select(.!=null)")
    if [ ! -z "$EXIST" ]; then
        echo patching osbms ${ROLE,,} status
        curl -XPATCH -H "Accept: application/json" -H "Content-Type: application/json-patch+json" \
            --data "[{\"op\": \"remove\", \"path\": \"/status/baremetalHosts/$COMPUTE\"}]" \
            localhost:8001/apis/osp-director.openstack.org/v1beta1/namespaces/openstack/openstackbaremetalsets/${ROLE,,}/status
    fi

    #
    # Remove compute reservations from OpenStackIPset status
    #
    EXIST=$(oc get osipset "${ROLE,,}" -n openstack -o json | jq ".status.hosts.\"$COMPUTE\" | select(.!=null)")
    if [ ! -z "$EXIST" ]; then
        echo patching osipset ${ROLE,,} status
        curl -XPATCH -H "Accept: application/json" -H "Content-Type: application/json-patch+json" \
            --data "[{\"op\": \"remove\", \"path\": \"/status/hosts/$COMPUTE\"}]" \
            localhost:8001/apis/osp-director.openstack.org/v1beta1/namespaces/openstack/openstackipsets/${ROLE,,}/status
    fi

    #
    # Remove compute reservations from all OpenStackNet status
    #
    for OSNET in $(oc get osnet -n openstack --output=jsonpath={.items..metadata.name}) ; do
        EXIST=$(oc get osnet $OSNET -n openstack -o json | jq ".status.reservations.\"$COMPUTE\" | select(.!=null)")
        if [ ! -z "$EXIST" ]; then
            echo patching osnet $OSNET status
            curl -XPATCH -H "Accept: application/json" -H "Content-Type: application/json-patch+json" \
                --data "[{\"op\": \"remove\", \"path\": \"/status/reservations/$COMPUTE\"}]" \
                localhost:8001/apis/osp-director.openstack.org/v1beta1/namespaces/openstack/openstacknets/${OSNET}/status
        fi
    done

    #
    # Remove compute reservations from OpenStackNetConfig
    #
    OSNETCFG=$(oc get osnetconfig -n openstack  --output=jsonpath={.items..metadata.name})
    EXIST=$(oc get osnetconfig $OSNETCFG -n openstack -o json | jq ".status.hosts.\"$COMPUTE\" | select(.!=null)")
    if [ ! -z "$EXIST" ]; then
        echo patching osnetcfg $OSNETCFG status
        curl -XPATCH -H "Accept: application/json" -H "Content-Type: application/json-patch+json" \
            --data "[{\"op\": \"remove\", \"path\": \"/status/hosts/$COMPUTE\"}]" \
            localhost:8001/apis/osp-director.openstack.org/v1beta1/namespaces/openstack/openstacknetconfigs/${OSNETCFG}/status
    fi
done

#
# Enable operator again to start webhook, which allows to update the spec
#
oc patch -n openstack csv/$CSV_NAME --type='json' \
--patch='[{"op":"replace", "path":"/spec/install/spec/deployments/0/spec/replicas", "value":1}]'

# need a sleep to the resource to show up
sleep 10
oc wait pod -l control-plane=controller-manager --for condition=ready -n openstack --timeout=60s

# need a sleep to give the webhook time to come up
sleep 10

#
# Handle spec updates for osnet
#
for COMPUTE in $COMPUTES; do
    #
    # Remove compute reservations from all OpenStackNet spec
    #
    for OSNET in $(oc get osnet -n openstack --output=jsonpath={.items..metadata.name}) ; do
        INDEX=$(oc get osnet $OSNET -n openstack -o json | jq ".spec.roleReservations.$ROLE.reservations | try map(.hostname == \"$COMPUTE\") | index(true) | select(.!=null)")
        if [ ! -z "$INDEX" ]; then
            echo patching osnet $OSNET spec index $INDEX
            oc patch osnet $OSNET --type=json -p="[{\"op\": \"remove\", \"path\": \"/spec/roleReservations/$ROLE/reservations/$INDEX\"}]" -n openstack
        fi
    done
done
