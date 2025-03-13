#!/bin/bash
set -x

TARGET_NAMESPACE=${TARGET_NAMESPACE:-"default"}
CSV_VERSION=${CSV_VERSION:-"0.0.1"}

oc delete -n ${TARGET_NAMESPACE} csv osp-director-operator.v${CSV_VERSION}
oc delete -n ${TARGET_NAMESPACE} subscription osp-director-operator-subscription
oc delete -n ${TARGET_NAMESPACE} catalogsource osp-director-operator-index
oc delete -n ${TARGET_NAMESPACE} operatorgroup osp-director-operator-group
