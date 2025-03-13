#!/bin/bash
set -e

echo "Creating OSP director operator index image"
echo "${REGISTRY}"
echo "${GITHUB_SHA_TAG}"
echo "${INDEX_IMAGE}"
echo "${INDEX_IMAGE_TAG}"
echo "${BUNDLE_IMAGE}"

echo "opm index add --bundles ${REGISTRY}/${BUNDLE_IMAGE}:${GITHUB_SHA_TAG} --tag ${REGISTRY}/${INDEX_IMAGE}:${GITHUB_SHA_TAG} -u podman --pull-tool podman"
opm index add --bundles "${REGISTRY}/${BUNDLE_IMAGE}:${GITHUB_SHA_TAG}" --tag "${REGISTRY}/${INDEX_IMAGE}:${GITHUB_SHA_TAG}" -u podman --pull-tool podman

echo "podman tag ${REGISTRY}/${INDEX_IMAGE}:${GITHUB_SHA_TAG} ${REGISTRY}/${INDEX_IMAGE}:${INDEX_IMAGE_TAG}"
podman tag "${REGISTRY}/${INDEX_IMAGE}:${GITHUB_SHA_TAG}" "${REGISTRY}/${INDEX_IMAGE}:${INDEX_IMAGE_TAG}"
