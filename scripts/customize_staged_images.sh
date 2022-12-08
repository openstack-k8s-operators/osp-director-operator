# Script to modify the image references in the staged image bundle so they can be
# installed easily tested from the registry-proxy.engineering.redhat.com staging repo internally.
set -xe

VERSION=${1:-"0.0.1"}
REPO=${2:-"quay.io/openstack-k8s-operators"}
INDEX_IMG="$REPO/osp-director-operator-index:$VERSION"
NEW_BUNDLE_IMG="$REPO/osp-director-operator-bundle:$VERSION"

# example: docker://registry-proxy.engineering.redhat.com/rh-osbs/rhosp-rhel8-osp-director-operator-bundle:1.3.0-4
BUNDLE_IMG=${BUNDLE_IMG:?"Please set the BUNDLE_IMG"}

REPLACE_URL=${REPLACE_URL:-"registry.redhat.io/rhosp-rhel8"}
WITH_URL=${WITH_URL:-"registry-proxy.engineering.redhat.com/rh-osbs/rhosp-rhel8"}

WORK_DIR=$(mktemp -d)

# build a new "staged" bundle image. THIS ONE IS FOR INTERNAL STAGED TESTING ONLY.
cat > $WORK_DIR/Dockerfile << EOF_CAT
FROM $BUNDLE_IMG as bundle

FROM golang:1.18 AS editor
COPY --from=bundle /manifests/osp-director-operator.clusterserviceversion.yaml /osp-director-operator.clusterserviceversion.yaml

RUN sed -e "s|$REPLACE_URL/osp-director-downloader|${WITH_URL}-osp-director-downloader|" -i /osp-director-operator.clusterserviceversion.yaml
RUN sed -e "s|$REPLACE_URL/osp-director-agent|${WITH_URL}-osp-director-agent|" -i /osp-director-operator.clusterserviceversion.yaml
RUN sed -e "s|$REPLACE_URL/osp-director-operator|${WITH_URL}-osp-director-operator|" -i /osp-director-operator.clusterserviceversion.yaml

FROM $BUNDLE_IMG
COPY --from=editor /osp-director-operator.clusterserviceversion.yaml /manifests/osp-director-operator.clusterserviceversion.yaml
EOF_CAT

UPDATED_BUNDLE_IMG=$(podman build -q $WORK_DIR)
podman tag $UPDATED_BUNDLE_IMG $NEW_BUNDLE_IMG
podman push $NEW_BUNDLE_IMG

# Create a new index for testing the "staged" bundle image built above
opm index add --bundles ${NEW_BUNDLE_IMG} --tag ${INDEX_IMG} -u podman --pull-tool podman
podman push ${INDEX_IMG}
