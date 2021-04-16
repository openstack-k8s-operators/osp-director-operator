ARG GOLANG_BUILDER=golang:1.14
ARG OPERATOR_BASE_IMAGE=gcr.io/distroless/static:nonroot

# Build the manager binary
FROM ${GOLANG_BUILDER} AS builder

#Arguments required by OSBS build system
ARG CACHITO_ENV_FILE=/remote-source/cachito.env

ARG REMOTE_SOURCE=.
ARG REMOTE_SOURCE_DIR=/remote-source
ARG REMOTE_SOURCE_SUBDIR=
ARG DEST_ROOT=/dest-root

ARG GO_BUILD_EXTRA_ARGS=

COPY $REMOTE_SOURCE $REMOTE_SOURCE_DIR
WORKDIR ${REMOTE_SOURCE_DIR}/${REMOTE_SOURCE_SUBDIR}

RUN mkdir -p ${DEST_ROOT}/usr/local/bin/

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN if [ ! -f $CACHITO_ENV_FILE ]; then go mod download ; fi

# Build manager
RUN if [ -f $CACHITO_ENV_FILE ] ; then source $CACHITO_ENV_FILE ; fi ; CGO_ENABLED=0  GO111MODULE=on go build ${GO_BUILD_EXTRA_ARGS} -a -o ${DEST_ROOT}/manager main.go

RUN cp -r templates ${DEST_ROOT}/templates

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM ${OPERATOR_BASE_IMAGE}

ARG DEST_ROOT=/dest-root
ARG USER_ID=nonroot:nonroot

LABEL com.redhat.component="osp-director-operator-container" \
      name="osp-director-operator" \
      version="1.0" \
      summary="OSP Director Operator" \
      io.k8s.name="osp-director-operator" \
      io.k8s.description="This image includes the osp-director-operator" \
      io.openshift.tags="cn-openstack openstack"

ENV USER_UID=1001 \
    OPERATOR_TEMPLATES=/usr/share/osp-director-operator/templates/ \
    WATCH_NAMESPACE=openstack,openshift-machine-api,openshift-sriov-network-operator

WORKDIR /

# Install operator binary to WORKDIR
COPY --from=builder ${DEST_ROOT}/manager .

# Install templates
COPY --from=builder ${DEST_ROOT}/templates ${OPERATOR_TEMPLATES}

USER ${USER_ID}

ENV PATH="/:${PATH}"

ENTRYPOINT ["/manager"]
