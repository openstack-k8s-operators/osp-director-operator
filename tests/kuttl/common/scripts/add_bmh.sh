#!/bin/bash

#
# Adds (or really, re-adds) a new BMH if YAML representing one is 
# present in a designated location on the filesystem
#

if [ -f /tmp/kuttl_bmh1.json ]; then
    oc apply -n openshift-machine-api -f /tmp/kuttl_bmh1_secret.json
    oc apply -n openshift-machine-api -f /tmp/kuttl_bmh1.json
    rm -rf /tmp/kuttl_bmh1_secret.json
    rm -rf /tmp/kuttl_bmh1.json
fi
