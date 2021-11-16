ARG BASE_IMAGE=centos:centos7

FROM $BASE_IMAGE

ARG PKG_CMD=yum
ARG REMOTE_SOURCE=
# Buildah expects the entrypoint.sh to be relative to pwd, podman expects
# the file to be relative to the Dockerfile location
ARG ENTRYPOINT_PATH=entrypoint.sh

RUN ${PKG_CMD} install -y qemu-img

ARG IMAGE_COMPONENT="osp-director-downloader-container"
ARG IMAGE_NAME="osp-director-downloader"
ARG IMAGE_VERSION="1.1.0"
ARG IMAGE_SUMMARY="OSP Director Image Downloader"
ARG IMAGE_DESC="This image includes the osp-director-downloader"
ARG IMAGE_TAGS="openstack director"

### DO NOT EDIT LINES BELOW
# Auto generated using CI tools from
# https://github.com/openstack-k8s-operators/openstack-k8s-operators-ci

# Labels required by upstream and osbs build system
LABEL com.redhat.component="${IMAGE_COMPONENT}" \
      name="${IMAGE_NAME}" \
      version="${IMAGE_VERSION}" \
      summary="${IMAGE_SUMMARY}" \
      io.k8s.name="${IMAGE_NAME}" \
      io.k8s.description="${IMAGE_DESC}" \
      io.openshift.tags="${IMAGE_TAGS}"
### DO NOT EDIT LINES ABOVE

WORKDIR /

COPY $REMOTE_SOURCE/$ENTRYPOINT_PATH /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
