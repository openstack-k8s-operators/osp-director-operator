ARG GOLANG_BUILDER=golang:1.13
ARG OPERATOR_BASE_IMAGE=gcr.io/distroless/static:nonroot

# Build the manager binary
FROM ${GOLANG_BUILDER} AS builder

ARG GO_BUILD_EXTRA_ARGS

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY pkg/ pkg/
COPY controllers/ controllers/
COPY templates/ templates/
COPY bindata/ bindata/
COPY cmd/webhook_clean/ cmd/webhook_clean/
RUN mkdir -p /usr/share/osp-director-operator/templates && mkdir -p /bindata/ && mkdir -p /cmd/

# Build manager
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build ${GO_BUILD_EXTRA_ARGS} -a -o manager main.go
# Build webhook cleaner
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build ${GO_BUILD_EXTRA_ARGS} -a -o osp-director-operator-webhook-clean ./cmd/webhook_clean
# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM ${OPERATOR_BASE_IMAGE}

ENV USER_UID=1001 \
    OPERATOR_BINDATA_DIR=/bindata/ \
    OPERATOR_TEMPLATES=/usr/share/osp-director-operator/templates/ \
    WATCH_NAMESPACE=openstack,openshift-machine-api

WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/templates /usr/share/osp-director-operator/templates/.
COPY --from=builder /workspace/bindata /bindata/.
COPY --from=builder /workspace/osp-director-operator-webhook-clean .
USER nonroot:nonroot

ENTRYPOINT ["/manager"]
