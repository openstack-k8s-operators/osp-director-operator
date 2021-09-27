# At a minimum you need to set GIT_COMMIT, RELEASE_VERSION, and OPERATOR_TAG to run this script.

#GIT_COMMIT=95832c4987ea253664620146b3ea6cd78868e1d8
#RELEASE_VERSION=1.0.0
#OPERATOR_TAG=1.0.0-4

GIT_COMMIT=${GIT_COMMIT:?"Please set the GIT_COMMIT that you want to use for the rebase (from the release branch)."}
RELEASE_VERSION=${RELEASE_VERSION:?"Please set the VERSION that you want to use for the rebase."}

UPSTREAM_BRANCH=${UPSTREAM_BRANCH:-"osp16_tech_preview"}
DOWNSTREAM_BRANCH=${DOWNSTREAM_BRANCH:-"rhos-16.2-rhel-8"}

if ! podman login --get-login registry.redhat.io &> /dev/null; then
  echo "Please run podman login registry.redhat.io before running this script."
  exit 1
fi

set -e

if [ -z "$OPERATOR_IMG" ]; then
   # get the latest release from registry.redhat.io if OPERATOR_IMG isn't set
   # FIXME: the tagging format is constantly changing right now. Would be good to figure out how to automatically set this
   #OPERATOR_TAG=$(skopeo list-tags docker://registry.redhat.io/rhosp-rhel8-tech-preview/osp-director-operator | jq -r '.Tags' | grep -v 'v1' | jq -r 'sort_by(.) | .[-1]')
   #OPERATOR_TAG=$(podman image search --list-tags registry.redhat.io/rhosp-rhel8-tech-preview/osp-director-operator --format json | jq -r '.Tags' | grep -v 'v1' | jq -r 'sort_by(.) | .[-1]')
   # should end up looking like this OPERATOR_TAG="1.0.0-1"
   OPERATOR_TAG=${OPERATOR_TAG?"Please set the OPERATOR_TAG to use. Should be the latest(last) released tag. Example: 1.0.0-4"}
   # NOTE: we increment the last -<num> here to guess at what the next osp-director-operator release build will be
   # so that we can add this URL into the OpenShift OLM/CSV file in our bundle image
   # A better way to do this would be to automatically substitute this in the downstream automation
   BUILD_ID=$(echo $OPERATOR_TAG | sed -e "s|.*-||")
   BUILD_ID_PLUS_1=$(($BUILD_ID + 1))
   OPERATOR_IMG="registry.redhat.io/rhosp-rhel8-tech-preview/osp-director-operator:$(echo $OPERATOR_TAG | sed -e 's|\(.*-\).*|\1|')$BUILD_ID_PLUS_1"
fi
echo $OPERATOR_IMG

TMP_DIR=$(mktemp -d)
mkdir -p "$TMP_DIR"
cd $TMP_DIR
wget https://github.com/operator-framework/operator-sdk/releases/download/v1.5.1/operator-sdk_linux_amd64
mv operator-sdk_linux_amd64 operator-sdk
chmod 755 operator-sdk
export PATH=$TMP_DIR/:$PATH
operator-sdk version

git clone https://github.com/openstack-k8s-operators/osp-director-operator.git upstream
cd "$TMP_DIR/upstream"
git checkout -b $UPSTREAM_BRANCH remotes/origin/$UPSTREAM_BRANCH

#FIXME: ideally this would occur after the source is imported downstream via automation
VERSION=$RELEASE_VERSION IMG=$OPERATOR_IMG make bundle 

cd "$TMP_DIR"

#checkout from code eng
git clone "ssh://$USER@code.engineering.redhat.com/osp-director-operator" downstream && mkdir -p osp-director-operator/.git/hooks/ && scp -p $USER@code.engineering.redhat.com:hooks/commit-msg "osp-director-operator/.git/hooks/"
cd downstream
git checkout -b $DOWNSTREAM_BRANCH remotes/origin/$DOWNSTREAM_BRANCH

sed -e "s|commit:.*|commit: $GIT_COMMIT|" -i upstream_sources.yml

cd $TMP_DIR/downstream/distgit/containers/osp-director-operator-bundle

sed -e "s|version:.*|version=\"$RELEASE_VERSION\"|" -i Dockerfile.in

git rm -r bundle
cp -a $TMP_DIR/upstream/bundle .
git add bundle

# HACKs for webhook deployment to work around: https://bugzilla.redhat.com/show_bug.cgi?id=1921000
# TODO: Figure out how to do this via Kustomize so that it's automatically rolled into the make
#       commands above
sed -i '/^    webhookPath:.*/a #added\n    containerPort: 4343\n    targetPort: 4343' bundle/manifests/osp-director-operator.clusterserviceversion.yaml
sed -i 's/deploymentName: webhook/deploymentName: osp-director-operator-controller-manager/g' bundle/manifests/osp-director-operator.clusterserviceversion.yaml

echo "'cd $TMP_DIR/downstream' and 'git commit' and 'git-review' to push changes to code.eng..."
