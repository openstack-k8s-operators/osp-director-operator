#!/bin/bash
set -x

TARGET_NAMESPACE=${TARGET_NAMESPACE:-"openstack"}
INDEX_IMAGE=${INDEX_IMAGE:-"quay.io/dprince/osp-director-operator-index:0.0.3"}
CSV_VERSION=${CSV_VERSION:-"0.0.3"}

if [ `oc get catalogsource -n "${TARGET_NAMESPACE}" --no-headers 2> /dev/null | grep osp-director | wc -l` -eq 0 ]; then
echo "Creating CatalogSource"
        cat <<EOF | oc create -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: osp-director-operator-index
  namespace: $TARGET_NAMESPACE
spec:
  sourceType: grpc
  image: $INDEX_IMAGE
EOF
fi

echo "Wait for the catalogSource to be available"
oc wait deploy "osp-director-operator-index" --for condition=available -n ${TARGET_NAMESPACE} --timeout="360s"

echo "Waiting for packagemanifest 'osp-director-operator' to be created in namespace '${TARGET_NAMESPACE}'"
sleep 10
RETRIES="${RETRIES:-20}"
for i in $(seq 1 $RETRIES); do
    oc get packagemanifest -n "${TARGET_NAMESPACE}" "osp-director-operator" && break
    sleep $i
    if [ "$i" -eq "${RETRIES}" ]; then
      echo "packagemanifest 'osp-director-operator' was never created in namespace '${TARGET_NAMESPACE}'"
      exit 1
    fi
done

if [ `oc get operatorgroup -n "${TARGET_NAMESPACE}" --no-headers 2> /dev/null | wc -l` -eq 0 ]; then
echo "Creating OperatorGroup"
cat <<EOF | oc create -f -
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: "osp-director-operator-group"
  namespace: "$TARGET_NAMESPACE"
spec:
  targetNamespaces:
  - "$TARGET_NAMESPACE"
EOF
fi

echo "Creating Subscription"
cat <<EOF | oc create -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: osp-director-operator-subscription
  namespace: $TARGET_NAMESPACE
spec:
  source: osp-director-operator-index
  sourceNamespace: $TARGET_NAMESPACE
  name: osp-director-operator
  startingCSV: osp-director-operator.v$CSV_VERSION
  channel: alpha
EOF
