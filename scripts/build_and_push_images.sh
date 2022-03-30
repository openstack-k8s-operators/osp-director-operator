#!/bin/bash
set -ex

#
# Builds and pushes operator, bundle and index images for a given version of an operator
#
# NOTE: Requires make, podman, opm and operator-sdk to be installed!
#

if ! podman login --get-login registry.redhat.io &> /dev/null; then
  echo "Please run podman login registry.redhat.io before running this script."
  exit 1
fi

VERSION=${1:-"17.0.1"}
REPO=${2:-"quay.io/openstack-k8s-operators"}
OP_NAME=${3:-"osp-director-operator"}
IMG="$REPO/$OP_NAME":$VERSION
IMAGE_TAG_BASE="$REPO/$OP_NAME"
INDEX_IMG="$REPO/$OP_NAME-index:$VERSION"

BUNDLE_IMG="$REPO/$OP_NAME-bundle:$VERSION"

AGENT_IMAGE="$OP_NAME-agent"
AGENT_IMG_BASE="$REPO/$AGENT_IMAGE"
AGENT_IMG="$AGENT_IMG_BASE:$VERSION"

DOWNLOADER_IMAGE="osp-director-downloader"
DOWNLOADER_IMG_BASE="$REPO/$DOWNLOADER_IMAGE"
DOWNLOADER_IMG="$DOWNLOADER_IMG_BASE:$VERSION"

CLUSTER_BUNDLE_FILE="bundle/manifests/osp-director-operator.clusterserviceversion.yaml"

# Base operator image
make build
make manifests
make generate
make IMG=${IMG} docker-build docker-push

# Agent image
make IMG=${AGENT_IMG} DOCKERFILE="Dockerfile.agent" docker-build docker-push

# Downloader image
make IMG=${DOWNLOADER_IMG} DOCKERFILE="containers/image_downloader/Dockerfile" DOCKER_BUILD_DIR="containers/image_downloader" docker-build docker-push

rm -Rf bundle
rm -Rf bundle.Dockerfile

# Generate bundle manifests
VERSION=${VERSION} IMG=${IMG} make bundle

# Replace AGENT_IMAGE_URL_DEFAULT in CSV
AGENT_IMG_WITH_DIGEST="${AGENT_IMG_BASE}@"$(skopeo inspect docker://${AGENT_IMG} | jq '.Digest' -r)
sed -z -e 's!\(AGENT_IMAGE_URL_DEFAULT\n\s\+value: \)\S\+!\1'${AGENT_IMG_WITH_DIGEST}'!' -i "${CLUSTER_BUNDLE_FILE}"

# Replace DOWNLOADER_IMAGE_URL_DEFAULT in CSV
DOWNLOADER_IMG_WITH_DIGEST="${DOWNLOADER_IMG_BASE}@"$(skopeo inspect docker://${DOWNLOADER_IMG} | jq '.Digest' -r)
sed -z -e 's!\(DOWNLOADER_IMAGE_URL_DEFAULT\n\s\+value: \)\S\+!\1'${DOWNLOADER_IMG_WITH_DIGEST}'!' -i "${CLUSTER_BUNDLE_FILE}"

# HACKs for webhook deployment to work around: https://bugzilla.redhat.com/show_bug.cgi?id=1921000
# TODO: Figure out how to do this via Kustomize so that it's automatically rolled into the make
#       commands above
sed -i '/^    webhookPath:.*/a #added\n    containerPort: 4343\n    targetPort: 4343' ${CLUSTER_BUNDLE_FILE}
sed -i 's/deploymentName: webhook/deploymentName: osp-director-operator-controller-manager/g' ${CLUSTER_BUNDLE_FILE}

# Convert any tags to digests within the CSV (for offline/air gapped environments)
for csv_image in $(cat ${CLUSTER_BUNDLE_FILE} | grep "image:" | sed -e "s|.*image:||" | sort -u); do
  base_image=$(echo $csv_image | cut -f 1 -d':')
  tag_image=$(echo $csv_image | cut -f 2 -d':')
  if [[ "$base_image:$tag_image" == "controller:latest" ]]; then
    sed -e "s|$base_image:$tag_image|$IMG|g" -i ${CLUSTER_BUNDLE_FILE}
  elif [[ "$base_image" == */"${AGENT_IMAGE}" ]]; then
    sed -e "s|$base_image:$tag_image|$AGENT_IMG_WITH_DIGEST|g" -i "${CLUSTER_BUNDLE_FILE}"
  elif [[ "$base_image" == */"${DOWNLOADER_IMAGE}" ]]; then
    sed -e "s|$base_image:$tag_image|$DOWNLOADER_IMG_WITH_DIGEST|g" -i "${CLUSTER_BUNDLE_FILE}"
  else
    digest_image=$(skopeo inspect docker://$base_image:$tag_image | jq '.Digest' -r)
    if [[ "$digest_image" == "" ]]; then
	echo "Failed to get image digest for docker://$base_image:$tag_image"
    else
        echo "$base_image:$tag_image becomes $base_image@$digest_image."
        sed -e "s|$base_image:$tag_image|$base_image@$digest_image|g" -i ${CLUSTER_BUNDLE_FILE}
    fi
  fi
done

# Build bundle image
VERSION=${VERSION} IMAGE_TAG_BASE=${IMAGE_TAG_BASE} make bundle-build

# Push bundle image
VERSION=${VERSION} IMAGE_TAG_BASE=${IMAGE_TAG_BASE} make bundle-push
#opm alpha bundle validate --tag ${BUNDLE_IMG} -b podman

# Index image
opm index add --bundles ${BUNDLE_IMG} --tag ${INDEX_IMG} -u podman --pull-tool podman
podman push ${INDEX_IMG}
