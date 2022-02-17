#!/bin/bash
set -e

CLUSTER_BUNDLE_FILE="bundle/manifests/osp-director-operator.clusterserviceversion.yaml"

echo "Creating OSP director operator bundle"
cd ..
echo "${GITHUB_SHA}"
echo "${BASE_IMAGE}"
echo "${AGENT_IMAGE}"
skopeo --version

echo "Calculating image digest for docker://${REGISTRY}/${BASE_IMAGE}:${GITHUB_SHA}"
DIGEST=$(skopeo inspect docker://${REGISTRY}/${BASE_IMAGE}:${GITHUB_SHA} | jq '.Digest' -r)
# Output: 
# Calculating image digest for docker://quay.io/openstack-k8s-operators/osp-director-operator:d03f2c1c362c04fc5ef819f92a218f9ea59bbd0c
# Digest: sha256:1d5b578fd212f8dbd03c0235f1913ef738721766f8c94236af5efecc6d8d8cb1
echo "Digest: ${DIGEST}"

RELEASE_VERSION=$(grep "^VERSION" Makefile | awk -F'?= ' '{ print $2 }')
OPERATOR_IMG_WITH_DIGEST="${REGISTRY}/${BASE_IMAGE}@${DIGEST}"

echo "New Operator Image with Digest: $OPERATOR_IMG_WITH_DIGEST"
echo "Release Version: $RELEASE_VERSION"

echo "Creating bundle image..."
VERSION=$RELEASE_VERSION IMG=$OPERATOR_IMG_WITH_DIGEST make bundle

echo "Applying work-arounds..."
sed -i '/^    webhookPath:.*/a #added\n    containerPort: 4343\n    targetPort: 4343' "${CLUSTER_BUNDLE_FILE}"
sed -i 's/deploymentName: webhook/deploymentName: osp-director-operator-controller-manager/g' "${CLUSTER_BUNDLE_FILE}"

# Replace AGENT_IMAGE_URL_DEFAULT in CSV

AGENT_IMG_BASE="${REGISTRY}/${AGENT_IMAGE}"
AGENT_IMG="${AGENT_IMG_BASE}:${GITHUB_SHA}"
AGENT_IMG_WITH_DIGEST="${AGENT_IMG_BASE}@"$(skopeo inspect docker://${AGENT_IMG} | jq '.Digest' -r)
sed -z -e 's!\(AGENT_IMAGE_URL_DEFAULT\n\s\+value: \)\S\+!\1'${AGENT_IMG_WITH_DIGEST}'!' -i "${CLUSTER_BUNDLE_FILE}"

echo "Bundle file images:"
cat "${CLUSTER_BUNDLE_FILE}" | grep "image:"
grep -A1 IMAGE_URL_DEFAULT "${CLUSTER_BUNDLE_FILE}"

# We do not want to exit here. Some images are in different registries, so
# error will be reported to the console.
set +e
for csv_image in $(cat "${CLUSTER_BUNDLE_FILE}" | grep "image:" | sed -e "s|.*image:||" | sort -u); do
  digest_image=""
  echo "CSV line: ${csv_image}"

  # case where @ is in the csv_image image
  if [[ "$csv_image" =~ .*"@".* ]]; then
    delimeter='@'
  else
    delimeter=':'
  fi

  base_image=$(echo $csv_image | cut -f 1 -d${delimeter})
  tag_image=$(echo $csv_image | cut -f 2 -d${delimeter})

  if [[ "$base_image:$tag_image" == "controller:latest" ]]; then
    echo "$base_image:$tag_image becomes $OPERATOR_IMG_WITH_DIGEST"
    sed -e "s|$base_image:$tag_image|$OPERATOR_IMG_WITH_DIGEST|g" -i "${CLUSTER_BUNDLE_FILE}"
  elif [[ "$base_image" == */"${AGENT_IMAGE}" ]]; then
    echo "$base_image:$tag_image becomes $AGENT_IMG_WITH_DIGEST"
    sed -e "s|$base_image:$tag_image|$AGENT_IMG_WITH_DIGEST|g" -i "${CLUSTER_BUNDLE_FILE}"
  else
    digest_image=$(skopeo inspect docker://${base_image}${delimeter}${tag_image} | jq '.Digest' -r)

    if [ ! -z "$digest_image" ]; then
      echo "Base image: $base_image"
      echo "$base_image${delimeter}$tag_image becomes $base_image@$digest_image"
      sed -i "s|$base_image$delimeter$tag_image|$base_image@$digest_image|g" "${CLUSTER_BUNDLE_FILE}"
    fi
  fi
done

echo "Resulting bundle file images:"
cat "${CLUSTER_BUNDLE_FILE}" | grep "image:"
grep -A1 IMAGE_URL_DEFAULT "${CLUSTER_BUNDLE_FILE}"
