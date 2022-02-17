# At a minimum you need to set GIT_COMMIT, RELEASE_VERSION, and OPERATOR_IMG_WITH_DIGEST to run this script.

#GIT_COMMIT=2b4fb608d3490be64c42fe073dd525a618fb4082
#RELEASE_VERSION=1.3.0
#OPERATOR_IMG_WITH_DIGEST=registry.redhat.io/rhosp-rhel8/osp-director-operator:1.3.0

OPERATOR_IMG_WITH_DIGEST=${OPERATOR_IMG_WITH_DIGEST:?"Please set the OPERATOR_IMG_WITH_DIGEST that you want to use for this rebase."}
GIT_COMMIT=${GIT_COMMIT:?"Please set the GIT_COMMIT that you want to use for the rebase (from the release branch)."}
RELEASE_VERSION=${RELEASE_VERSION:?"Please set the VERSION that you want to use for the rebase."}

UPSTREAM_BRANCH=${UPSTREAM_BRANCH:-"v1.3.x"}
DOWNSTREAM_BRANCH=${DOWNSTREAM_BRANCH:-"rhos-16.2-rhel-8"}

if ! podman login --get-login registry.redhat.io &> /dev/null; then
  echo "Please run podman login registry.redhat.io before running this script."
  exit 1
fi

set -ex

TMP_DIR=$(mktemp -d)
mkdir -p "$TMP_DIR"
cd $TMP_DIR
wget https://github.com/operator-framework/operator-sdk/releases/download/v1.19.0/operator-sdk_linux_amd64
mv operator-sdk_linux_amd64 operator-sdk
chmod 755 operator-sdk
export PATH=$TMP_DIR/:$PATH
operator-sdk version

git clone https://github.com/openstack-k8s-operators/osp-director-operator.git upstream
cd "$TMP_DIR/upstream"
git checkout -b build remotes/origin/$UPSTREAM_BRANCH

#FIXME: ideally this would occur after the source is imported downstream via automation
VERSION=$RELEASE_VERSION IMG=$OPERATOR_IMG_WITH_DIGEST make bundle 

cd "$TMP_DIR"

#checkout from code eng
#git clone "ssh://$USER@code.engineering.redhat.com/osp-director-operator" downstream && mkdir -p osp-director-operator/.git/hooks/ && scp -p $USER@code.engineering.redhat.com:hooks/commit-msg "osp-director-operator/.git/hooks/"
git clone "https://gitlab.cee.redhat.com/openstack-midstream/osp-director-operator.git" downstream
cd downstream
git checkout -b build remotes/origin/$DOWNSTREAM_BRANCH
git remote add $USER "git@gitlab.cee.redhat.com:$USER/osp-director-operator.git"

sed -e "s|commit:.*|commit: $GIT_COMMIT|" -i upstream_sources.yml
git add upstream_sources.yml

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

# Convert any tags to digests within the CSV (for offline/air gapped environments)
#for csv_image in $(cat bundle/manifests/osp-director-operator.clusterserviceversion.yaml | grep "image:" | sed -e "s|.*image:||" | sort -u); do
#  base_image=$(echo $csv_image | cut -f 1 -d':')
#  tag_image=$(echo $csv_image | cut -f 2 -d':')
#  if [[ "$base_image:$tag_image" == "controller:latest" ]]; then
#    sed -e "s|$base_image:$tag_image|$IMG|g" -i bundle/manifests/osp-director-operator.clusterserviceversion.yaml
#  else
#    digest_image=$(skopeo inspect docker://$base_image:$tag_image | jq '.Digest' -r)
#    echo "$base_image:$tag_image becomes $base_image@$digest_image."
#    sed -e "s|$base_image:$tag_image|$base_image@$digest_image|g" -i bundle/manifests/osp-director-operator.clusterserviceversion.yaml
#  fi
#done

echo "'cd $TMP_DIR/downstream' and 'git commit' and 'git push $USER' to push changes to gitlab..."
