#!/usr/bin/env bash

set -eux

REPO=github.com/openstack-k8s-operators/osp-director-operator
WHAT=agent
GOFLAGS=${GOFLAGS:-}
GLDFLAGS=${GLDFLAGS:-}

GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)

# Go to the root of the repo
cdup="$(git rev-parse --show-cdup)" && test -n "$cdup" && cd "$cdup"

if [ -z ${VERSION_OVERRIDE+a} ]; then
	echo "Using version from git..."
	VERSION_OVERRIDE=$(git describe --abbrev=8 --dirty --always)
fi

GLDFLAGS+="-X ${REPO}/pkg/version.Raw=${VERSION_OVERRIDE}"

if [ -z ${BIN_PATH+a} ]; then
	export BIN_PATH=build/_output/${GOOS}/${GOARCH}
fi

mkdir -p ${BIN_PATH}

CGO_ENABLED=1

echo "Building ${REPO}/cmd/${WHAT}"
go build -o ${BIN_PATH}/${WHAT} ./containers/agent
