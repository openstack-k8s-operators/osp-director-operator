#!/bin/bash

set -eux

play() {

  cd ~/ansible/*
  # TODO: for now disable opendev-validation-ceph
  # The check fails because the lvm2 package is not installed in openstackclient container image image
  # and ansible_facts include packages from undercloud.
  time ansible-playbook -i inventory.yaml \
    --private-key /home/cloud-admin/.ssh/id_rsa \
    --skip-tags opendev-validation-ceph \
    --become deploy_steps_playbook.yaml

  cp /etc/openstack/clouds.yaml ~/tripleo-deploy/

}

usage() { echo "Usage: $0 [-r] [-p]" 1>&2; exit 1; }

while getopts ":rp" arg; do
    case "${arg}" in
        p)
            play
            ;;
        *)
            usage
            exit 0
            ;;
    esac
done
