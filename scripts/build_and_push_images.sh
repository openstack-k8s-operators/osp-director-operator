#!/bin/bash
set -ex

#
# Builds and pushes operator, bundle and index images for a given version of an operator
#
# NOTE: Requires make, podman, opm and operator-sdk to be installed!
#

VERSION=${1:-"17.0.1"}
REPO=${2:-"quay.io/openstack-k8s-operators"}
OP_NAME=${3:-"osp-director-operator"}
IMG="$REPO/$OP_NAME":$VERSION
IMAGE_TAG_BASE="$REPO/$OP_NAME"
INDEX_IMG="$REPO/$OP_NAME-index:$VERSION"

BUNDLE_IMG="$REPO/$OP_NAME-bundle":$VERSION

# Base operator image
make manager
make manifests
make generate
IMG=${IMG} make docker-build docker-push

rm -Rf bundle
rm -Rf bundle.Dockerfile

# Generate bundle manifests
VERSION=${VERSION} IMG=${IMG} make bundle

# HACKs for webhook deployment to work around: https://bugzilla.redhat.com/show_bug.cgi?id=1921000
# TODO: Figure out how to do this via Kustomize so that it's automatically rolled into the make
#       commands above
sed -i '/^    webhookPath:.*/a #added\n    containerPort: 4343\n    targetPort: 4343' bundle/manifests/osp-director-operator.clusterserviceversion.yaml
sed -i 's/deploymentName: webhook/deploymentName: osp-director-operator-controller-manager/g' bundle/manifests/osp-director-operator.clusterserviceversion.yaml

# Build bundle image
VERSION=${VERSION} IMAGE_TAG_BASE=${IMAGE_TAG_BASE} make bundle-build

# Push bundle image
VERSION=${VERSION} IMAGE_TAG_BASE=${IMAGE_TAG_BASE} make bundle-push
#opm alpha bundle validate --tag ${BUNDLE_IMG} -b podman

# Index image
opm index add --bundles ${BUNDLE_IMG} --tag ${INDEX_IMG} -u podman --pull-tool podman
podman push ${INDEX_IMG}
