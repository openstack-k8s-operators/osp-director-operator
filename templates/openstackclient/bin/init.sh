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

if [ -v FQDN ]; then
  echo "$FQDN" > /mnt/etc/hostname
else
  cp /etc/hostname /mnt/etc/hostname
fi

# if the pvc is an empty volume, copy the existing hosts file to it
if [ ! -f /mnt/etc/hosts ]; then
  cp /etc/hosts /mnt/etc/
fi

mkdir -p /home/cloud-admin/tripleo-deploy/validations
rm -rf /home/cloud-admin/tripleo-deploy/overcloud-ansible*

if [ -d /mnt/ssh-config ]; then
  # add cloud-admin ssh keys to EmptyDir Vol mount to /root/.ssh in openstackclient
  sudo mkdir -p /root/.ssh
  sudo cp /mnt/ssh-config/* /root/.ssh/
  sudo chmod 600 /root/.ssh/id_rsa
  sudo chown -R root: /root/.ssh

  # add cloud-admin ssh keys to /home/cloud-admin/.ssh in openstackclient
  mkdir -p /home/cloud-admin/.ssh
  cp /mnt/ssh-config/* /home/cloud-admin/.ssh/
  chmod 600 /home/cloud-admin/.ssh/id_rsa
  chmod 600 /home/cloud-admin/.ssh/git_id_rsa
  chown -R cloud-admin: /home/cloud-admin/.ssh
fi

if [ -v GIT_URL ]; then
  GIT_HOST=$(echo $GIT_URL | sed -e 's|^git@\(.*\):.*|\1|g')
  GIT_USER=$(echo $GIT_URL | sed -e 's|^git@.*:\(.*\)/.*|\1|g')

  cat <<EOF >> /home/cloud-admin/.ssh/config


Host $GIT_HOST
    User $GIT_USER
    IdentityFile /home/cloud-admin/.ssh/git_id_rsa
    StrictHostKeyChecking no
EOF

  git config --global user.email "dev@null.io"
  git config --global user.name "OSP Director Operator"
fi

if [ -d /mnt/ca-certs ]; then
  sudo touch /run/ca-certs-epoch
  sudo cp -v /mnt/ca-certs/* /etc/pki/ca-trust/source/anchors/
  sudo update-ca-trust
  sudo bash -c 'find /etc/pki/ca-trust -newer /run/ca-certs-epoch -print0 | tar -c --null -T - | tar -C /var/lib/kolla/src -xvf -'
fi

if [ "$IPA_SERVER" != "" -a ! -f /var/lib/ipa-client/sysrestore/sysrestore.index ]; then

  # Ensure hostname is correct for ipa-client install
  sudo bash -c "cat /mnt/etc/hostname > /etc/hostname"
  # Ensure hostname -f and python socket.getfqdn() return the FQDN
  SHORT_HOSTNAME=$(hostname -s)
  LONG_HOSTNAME=$(hostname -f)
  if [ "$LONG_HOSTNAME" != "$FQDN" ]; then
    sed -i -e "s/^\([0-9.]\+\)\s\+\(.*\)${SHORT_HOSTNAME}\$/\1\t\2${FQDN} ${SHORT_HOSTNAME}/" /mnt/etc/hosts
    sudo bash -c "cat /mnt/etc/hosts > /etc/hosts"
  fi

  cat <<EOF > /home/cloud-admin/openstackclient_ipa_install.yaml
{{`---
- hosts: localhost
  become: true
  environment:
    IPA_USER: "{{ ipa_server_user }}"
    IPA_HOST: "{{ ipa_server_hostname }}"
    IPA_PASS: "{{ ipa_server_password }}"
  vars:
    undercloud_fqdn: "{{ lookup('env', 'FQDN') }}"
    ipa_server_password: "{{ lookup('env', 'IPA_SERVER_PASSWORD') }}"
    ipa_domain: "{{ lookup('env', 'IPA_DOMAIN') }}"
  tasks:
    - name: IPA Client install
      command: "ipa-client-install -U --force-join --no-ntp --no-nisdomain --no-sshd --no-ssh --no-sudo --no-dns-sshfp --principal='{{ ipa_server_user }}' --domain='{{ ipa_domain }}' --realm='{{ ipa_realm }}' --server '{{ ipa_server_hostname }}'"
      args:
        stdin: "{{ ipa_server_password }}"
    - name: Disable sssd
      command: "authselect select minimal"
    - name: kinit to get admin credentials
      command: kinit "{{ ipa_server_user }}@{{ ipa_realm }}"
      args:
        stdin: "{{ ipa_server_password }}"
      register: kinit
      changed_when: kinit.rc == 0
      no_log: true

    - name: setup the undercloud and get keytab
      include_role:
        name: tripleo_ipa_setup
EOF`}}

  sudo touch /run/ipa-epoch
  ansible-playbook -e ipa_server_user=${IPA_SERVER_USER} -e ipa_realm=${IPA_REALM} -e ipa_server_hostname=${IPA_SERVER} -e ipa_domain=${IPA_DOMAIN} /home/cloud-admin/openstackclient_ipa_install.yaml

  # Add all files modified by ipa client install to kolla src
  # TODO: could use overlayfs mounts instead?
  sudo bash -c 'find /etc/krb5* /etc/sssd /etc/authselect /etc/novajoin /etc/openldap /etc/ipa /etc/pki /var/lib/ipa-client /var/lib/authselect /var/log/ipaclient-install.log -newer /run/ipa-epoch -print0 | tar -c --null -T - | tar -C /var/lib/kolla/src -xvf -'
fi
