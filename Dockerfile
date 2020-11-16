# golang-builder is used in OSBS build
ARG GOLANG_BUILDER=golang:1.13
ARG OPERATOR_BASE_IMAGE=registry.access.redhat.com/ubi7/ubi-minimal:latest

# Build the manager binary
FROM ${GOLANG_BUILDER} as builder

ARG REMOTE_SOURCE=.
ARG REMOTE_SOURCE_DIR=osp-director-operator
ARG REMOTE_SOURCE_SUBDIR=.
ARG DEST_ROOT=/dest-root
ARG GO_BUILD_EXTRA_ARGS="-v"

COPY $REMOTE_SOURCE $REMOTE_SOURCE_DIR
WORKDIR ${REMOTE_SOURCE_DIR}/${REMOTE_SOURCE_SUBDIR}

RUN mkdir -p ${DEST_ROOT}/usr/local/bin/

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/
COPY bindata/ ${DEST_ROOT}/bindata/
COPY templates/ ${DEST_ROOT}/templates/

# Build
RUN CGO_ENABLED=0 GO111MODULE=on go build ${GO_BUILD_EXTRA_ARGS} -a -o ${DEST_ROOT}/manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM ${OPERATOR_BASE_IMAGE}
ARG DEST_ROOT=/dest-root

LABEL   com.redhat.component="osp-director-operator-container" \
        name="osp-director-operator" \
        version="1.0" \
        summary="OSP Director Operator" \
        io.k8s.name="osp-director-operator" \
        io.k8s.description="This image includes the osp-director-operator"

# TODO: For now hard code the WATCH_NAMESPACE here
ENV USER_UID=1001 \
    OPERATOR_BINDATA_DIR=/bindata/ \
    OPERATOR_TEMPLATES=/usr/share/osp-director-operator/templates/ \
    WATCH_NAMESPACE=openstack

# install our bindata
RUN  mkdir -p ${OPERATOR_BINDATA_DIR}
COPY --from=builder ${DEST_ROOT}/bindata ${OPERATOR_BINDATA_DIR}

RUN  mkdir -p ${OPERATOR_TEMPLATES}
COPY --from=builder ${DEST_ROOT}/templates ${OPERATOR_TEMPLATES}

WORKDIR /
COPY --from=builder ${DEST_ROOT}/manager .
USER nonroot:nonroot

ENTRYPOINT ["/manager"]
