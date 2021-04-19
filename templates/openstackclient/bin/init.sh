#!/bin//bash
#
# Copyright 2020 Red Hat Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License. You may obtain
# a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
# WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
# License for the specific language governing permissions and limitations
# under the License.
set -ex

# if the pvc is an empty volume, copy the existing hosts file to it
if [ ! -f /mnt/etc/hosts ]; then
  cp /etc/hosts /mnt/etc/
fi

mkdir -p /home/cloud-admin/tripleo-deploy/validations
rm -rf /home/cloud-admin/tripleo-deploy/overcloud-ansible*

# add cloud-admin ssh keys to EmptyDir Vol mount to /root/.ssh in openstackclient
sudo mkdir -p /root/.ssh
sudo cp /mnt/ssh-config/* /root/.ssh/
sudo chmod 600 /root/.ssh/id_rsa
sudo chown -R root: /root/.ssh

# add cloud-admin ssh keys to /home/cloud-admin/.ssh in openstackclient
mkdir -p /home/cloud-admin/.ssh
cp /mnt/ssh-config/* /home/cloud-admin/.ssh/
chmod 600 /home/cloud-admin/.ssh/id_rsa
chown -R cloud-admin: /home/cloud-admin/.ssh