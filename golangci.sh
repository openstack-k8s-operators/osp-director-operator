#!/bin/bash
set -ex

export GOFLAGS="-mod=mod"

# Install golangci
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | bash -x -s --
#curl -o install.sh https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh
#bash -x install.sh -d "latest"
#bash -x install.sh -d "v1.47.2"

#go get -u -d golang.org/x/lint/golint
#go install golang.org/x/lint/golint

GOGC=10 GOLANGCI_LINT_CACHE=/tmp/golangci-cache ./bin/golangci-lint run --timeout=2m -v
