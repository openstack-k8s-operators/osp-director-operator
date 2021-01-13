#!/usr/bin/env bash
set -e

GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)

WHAT=admission-controller

if [ -z ${BIN_PATH+a} ]; then
	export BIN_PATH=build/_output/${GOOS}/${GOARCH}
fi

mkdir -p ${BIN_PATH}

echo "Building ${WHAT}"
go build -o ${BIN_PATH}/${WHAT} ./containers/admission_controller