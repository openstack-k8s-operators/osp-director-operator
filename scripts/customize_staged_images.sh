set -xe

VERSION=${1:-"0.0.1"}
REPO=${2:-"quay.io/openstack-k8s-operators"}
MY_INDEX_IMG="$REPO/osp-director-operator-index:$VERSION"
MY_BUNDLE_IMG="$REPO/osp-director-operator-bundle:$VERSION"

#example: docker://registry-proxy.engineering.redhat.com/rh-osbs/rhosp-rhel8-tech-preview-osp-director-provisioner:1.2.2}
PROVISIONER_IMG=${PROVISIONER_IMG:?"Please set the PROVISIONER_IMG"}
DOWNLOADER_IMG=${DOWNLOADER_IMG:?"Please set the DOWNLOADER_IMG"}

# example: docker://registry-proxy.engineering.redhat.com/rh-osbs/rhosp-rhel8-tech-preview-osp-director-downloader:1.2.2
DOWNLOADER_IMG=${DOWNLOADER_IMG:?"Please set the DOWNLOADER_IMG"}

# example: docker://registry-proxy.engineering.redhat.com/rh-osbs/rhosp-rhel8-tech-preview-osp-director-operator:1.2.2
OPERATOR_IMG=docker://registry-proxy.engineering.redhat.com/rh-osbs/rhosp-rhel8-tech-preview-osp-director-operator:1.2.2
OPERATOR_IMG=${OPERATOR_IMG:?"Please set the OPERATOR_IMG"}

# example: docker://registry-proxy.engineering.redhat.com/rh-osbs/rhosp-rhel8-tech-preview-osp-director-operator-bundle:1.2.2-4
BUNDLE_IMG=${BUNDLE_IMG:?"Please set the BUNDLE_IMG"}

WORK_DIR=$(mktemp -d)

# copy the existing CSV out of the staged bundle
cat > $WORK_DIR/copy-file.sh << EOF_CAT
# get the clusterserviceversion.yaml from the staged image
IMG_DIGEST=\$(podman pull -q $BUNDLE_IMG)
MOUNT_DIR=\$(podman image mount \$IMG_DIGEST)
cp \$MOUNT_DIR/manifests/osp-director-operator.clusterserviceversion.yaml $WORK_DIR/osp-director-operator.clusterserviceversion.yaml
podman unmount -a
EOF_CAT
podman unshare bash $WORK_DIR/copy-file.sh
echo $WORK_DIR/osp-director-operator.clusterserviceversion.yaml

# modify this CSV so that it contains "staged" images so we can test it

sed -e "s|registry.redhat.io/rhosp-rhel8-tech-preview/|registry-proxy.engineering.redhat.com/rh-osbs/rhosp-rhel8-tech-preview-|" -i $WORK_DIR/osp-director-operator.clusterserviceversion.yaml


cat > $WORK_DIR/Dockerfile << EOF_CAT
FROM $BUNDLE_IMG
COPY osp-director-operator.clusterserviceversion.yaml /manifests/osp-director-operator.clusterserviceversion.yaml
EOF_CAT

UPDATED_BUNDLE_IMG=$(podman build -q $WORK_DIR)
podman tag $UPDATED_BUNDLE_IMG $MY_BUNDLE_IMG
podman push $MY_BUNDLE_IMG

opm index add --bundles ${MY_BUNDLE_IMG} --tag ${MY_INDEX_IMG} -u podman --pull-tool podman
podman push ${MY_INDEX_IMG}
