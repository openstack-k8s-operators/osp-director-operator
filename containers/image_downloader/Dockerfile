ARG IMAGE_DOWNLOADER_BASE=centos:centos7

FROM ${IMAGE_DOWNLOADER_BASE} as build

ARG PKG_CMD=yum
ARG REMOTE_SOURCE=
# Buildah expects the entrypoint.sh to be relative to pwd, podman expects
# the file to be relative to the Dockerfile location
ARG ENTRYPOINT_PATH=entrypoint.sh

RUN ${PKG_CMD} install -y qemu-img

LABEL   com.redhat.component="osp-director-downloader-container" \
        name="osp-director-downloader" \
        version="1.0" \
        summary="OSP Director Image Downloader" \
        io.k8s.name="osp director image downloader" \
        io.k8s.description="This image includes the osp-director-downloader" \
        io.openshift.tags="cn-openstack openstack"

WORKDIR /

COPY ${REMOTE_SOURCE}/${ENTRYPOINT_PATH} /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
