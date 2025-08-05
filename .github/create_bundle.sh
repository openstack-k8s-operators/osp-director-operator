#!/bin/bash
set -e

CLUSTER_BUNDLE_FILE="bundle/manifests/osp-director-operator.clusterserviceversion.yaml"

echo "Creating OSP director operator bundle"
cd ..
echo "${GITHUB_SHA}"
echo "${BASE_IMAGE}"
echo "${AGENT_IMAGE}"
echo "${DOWNLOADER_IMAGE}"
echo "${OSP_RELEASE}"
skopeo --version || true

if [[ -z "${OPERATOR_IMG_WITH_DIGEST}" ]]; then
  echo "Calculating image digest for docker://${REGISTRY}/${BASE_IMAGE}:${GITHUB_SHA}"
  DIGEST=$(skopeo inspect docker://${REGISTRY}/${BASE_IMAGE}:${GITHUB_SHA} | jq '.Digest' -r)
  # Output:
  # Calculating image digest for docker://quay.io/openstack-k8s-operators/osp-director-operator:d03f2c1c362c04fc5ef819f92a218f9ea59bbd0c
  # Digest: sha256:1d5b578fd212f8dbd03c0235f1913ef738721766f8c94236af5efecc6d8d8cb1
  echo "Digest: ${DIGEST}"

  OPERATOR_IMG_WITH_DIGEST="${REGISTRY}/${BASE_IMAGE}@${DIGEST}"
fi

if [[ -z "${RELEASE_VERSION}" ]]; then
  RELEASE_VERSION=$(grep "^VERSION" Makefile | awk -F'?= ' '{ print $2 }')-${OSP_RELEASE}
fi

echo "New Operator Image with Digest: $OPERATOR_IMG_WITH_DIGEST"
echo "Release Version: $RELEASE_VERSION"

echo "Creating bundle image..."
OSP_RELEASE=${OSP_RELEASE} VERSION=$RELEASE_VERSION IMG=$OPERATOR_IMG_WITH_DIGEST make bundle

echo "Bundle file images:"
cat "${CLUSTER_BUNDLE_FILE}" | grep "image:"
grep -A1 IMAGE_URL_DEFAULT "${CLUSTER_BUNDLE_FILE}"

# Replace AGENT_IMAGE_URL_DEFAULT in CSV
if [[ -z "${AGENT_IMG_WITH_DIGEST}" ]]; then
  AGENT_IMG_BASE="${REGISTRY}/${AGENT_IMAGE}"
  AGENT_IMG="${AGENT_IMG_BASE}:${GITHUB_SHA}"
  AGENT_DIGEST=$(skopeo inspect docker://${AGENT_IMG} | jq '.Digest' -r)
  if [[ -z "$AGENT_DIGEST" ]]; then
    echo "ERROR: skopeo inspect failed for ${AGENT_IMG}"
    exit 1
  fi
  AGENT_IMG_WITH_DIGEST="${AGENT_IMG_BASE}@${AGENT_DIGEST}"
fi
sed -z -e 's!\(AGENT_IMAGE_URL_DEFAULT\n\s\+value: \)\S\+!\1'${AGENT_IMG_WITH_DIGEST}'!' -i "${CLUSTER_BUNDLE_FILE}"

# Replace DOWNLOADER_IMAGE_URL_DEFAULT in CSV
if [[ -z "${DOWNLOADER_IMG_WITH_DIGEST}" ]]; then
  DOWNLOADER_IMG_BASE="${REGISTRY}/${DOWNLOADER_IMAGE}"
  DOWNLOADER_IMG="${DOWNLOADER_IMG_BASE}:${GITHUB_SHA}"
  DOWNLOADER_DIGEST=$(skopeo inspect docker://${DOWNLOADER_IMG} | jq '.Digest' -r)
  if [[ -z "$DOWNLOADER_DIGEST" ]]; then
    echo "ERROR: skopeo inspect failed for ${DOWNLOADER_IMG}"
    exit 1
  fi
  DOWNLOADER_IMG_WITH_DIGEST="${DOWNLOADER_IMG_BASE}@${DOWNLOADER_DIGEST}"
fi
sed -z -e 's!\(DOWNLOADER_IMAGE_URL_DEFAULT\n\s\+value: \)\S\+!\1'${DOWNLOADER_IMG_WITH_DIGEST}'!' -i "${CLUSTER_BUNDLE_FILE}"

echo "Applying work-arounds..."
sed -i '/^    webhookPath:.*/a #added\n    containerPort: 4343\n    targetPort: 4343' "${CLUSTER_BUNDLE_FILE}"
sed -i 's/deploymentName: webhook/deploymentName: osp-director-operator-controller-manager/g' "${CLUSTER_BUNDLE_FILE}"

# We do not want to exit here. Some images are in different registries, so
# error will be reported to the console.
set +e
for csv_image in $(cat "${CLUSTER_BUNDLE_FILE}" | grep -E "(image:|value:)" | sed -e "s|.*image:||;s|.*value:||" | sort -u); do
  digest_image=""
  echo "CSV line: ${csv_image}"
  
  if ! [[ "$csv_image" =~ .*:.* ]]; then
    # Matching for "value:" means we get matches that aren't image pullspecs.
    echo "$csv_image does not look like an image, skipping it."
    continue
  fi

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
  elif [[ "$base_image" == */"${DOWNLOADER_IMAGE}" ]]; then
    echo "$base_image:$tag_image becomes $DOWNLOADER_IMG_WITH_DIGEST"
    sed -e "s|$base_image:$tag_image|$DOWNLOADER_IMG_WITH_DIGEST|g" -i "${CLUSTER_BUNDLE_FILE}"
  else
    # Check for an env var containing the full image pullspec with digest.
    # Useful for hermetic builds that can't use the network when generating bundles.
    # "quay.io/openstack-k8s-operators/kube-rbac-proxy" -> RELATED_IMAGE_KUBE_RBAC_PROXY_PULLSPEC
    # 1. Take only the image name, e.g. kube-rbac-proxy
    image_name="${base_image##*/}"
    # 2. Convert it to uppercase, e.g. KUBE-RBAC-PROXY
    image_name="${image_name^^}"
    # 3. Replace dashes with underscores, e.g. KUBE_RBAC_PROXY
    image_name="${image_name//-/_}"
    # 4. Build up the env var name to check, e.g. RELATED_IMAGE_KUBE_RBAC_PROXY_PULLSPEC
    envvar_with_digest=RELATED_IMAGE_${image_name}_PULLSPEC
    image_with_digest=${!envvar_with_digest}
    if [[ -z "${image_with_digest}" ]]; then
      # No env var found, fall back to querying the registry
      digest_image=$(skopeo inspect docker://${base_image}${delimeter}${tag_image} | jq '.Digest' -r)
      image_with_digest=${base_image}@${digest_image}
    fi
    echo "Base image: $base_image"
    if [ -n "${image_with_digest}" ]; then
      echo "$base_image${delimeter}$tag_image becomes $image_with_digest"
      sed -i "s|$base_image$delimeter$tag_image|$image_with_digest|g" "${CLUSTER_BUNDLE_FILE}"
    else
      echo "$base_image${delimeter}$tag_image not changed"
    fi
  fi
done

echo "Resulting bundle file images:"
cat "${CLUSTER_BUNDLE_FILE}" | grep "image:"
grep -A1 IMAGE_URL_DEFAULT "${CLUSTER_BUNDLE_FILE}"
