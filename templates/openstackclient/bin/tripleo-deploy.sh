#!/bin/bash

set -eux

mkdir -p /home/cloud-admin/tripleo-deploy/validations
if [ ! -L /var/log/validations ]; then
  sudo ln -s /home/cloud-admin/tripleo-deploy/validations /var/log/validations
fi

play() {
  if [ ! -d /home/cloud-admin/playbooks ]; then
    git clone $GIT_URL /home/cloud-admin/playbooks
  fi
  pushd /home/cloud-admin/playbooks
  git checkout latest || git checkout -b latest origin/latest
  git pull origin latest

  cd tripleo-ansible*

  # TODO: for now disable opendev-validation-ceph
  # The check fails because the lvm2 package is not installed in openstackclient container image image
  # and ansible_facts include packages from undercloud.
  time ansible-playbook -i inventory.yaml \
    --private-key /home/cloud-admin/.ssh/id_rsa \
    --skip-tags opendev-validation-ceph \
    --become deploy_steps_playbook.yaml

  cp /etc/openstack/clouds.yaml ~/tripleo-deploy/

}

play
