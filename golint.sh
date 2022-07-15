#!/bin/bash
set -ex

# Set to "" if lint errors should not fail the job (default golint behaviour)
# "-set_exit_status" otherwise
LINT_EXIT_STATUS="-set_exit_status"

export GOFLAGS="-mod=mod"

go get -u -d golang.org/x/lint/golint
go install golang.org/x/lint/golint

golint ${LINT_EXIT_STATUS} ./...
