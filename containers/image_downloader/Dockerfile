FROM centos:centos7 as build

RUN yum install -y qemu-img

WORKDIR /

ADD ./entrypoint.sh /
ENTRYPOINT ["/entrypoint.sh"]
