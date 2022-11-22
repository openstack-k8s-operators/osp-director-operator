#!/bin/bash

#
# Dumps a BMH and its credentials secret to designated locations and then deletes the BMH
#

bmhname=$(oc get -n openstack osbms compute -o json | jq -r '.status.baremetalHosts["compute-0"].hostRef')
bmhsecretname=$(oc get bmh -n openshift-machine-api ${bmhname} -o json | jq -r '.spec.bmc.credentialsName')
oc get -n openshift-machine-api bmh ${bmhname} -o json |\
jq 'del(.metadata.labels,.metadata.annotations,.metadata.generation,.metadata.resourceVersion,.metadata.uid,.spec.consumerRef,.spec.userData,.spec.networkData,.spec.image)' |\
jq '.spec.online=false' > /tmp/kuttl_bmh1.json
oc get -n openshift-machine-api secret ${bmhsecretname} -o json |\
jq 'del(.metadata.labels,.metadata.annotations,.metadata.generation,.metadata.resourceVersion,.metadata.uid,.metadata.ownerReferences)' > /tmp/kuttl_bmh1_secret.json
oc delete -n openshift-machine-api bmh ${bmhname}
