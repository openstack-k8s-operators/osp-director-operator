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

umask 0022
CHOWN_UID=$(id -u)
CHOWN_GID=$(id -g)

sudo chown $CHOWN_UID:$CHOWN_GID /home/cloud-admin
sudo chmod 00755 /home/cloud-admin

sudo cp -a /etc/hostname /mnt/etc/hostname
if [ -v FQDN ]; then
  sudo tee /mnt/etc/hostname >/dev/null <<<"$FQDN"
fi

# if the pvc is an empty volume, copy the existing hosts file to it
if [ ! -f /mnt/etc/hosts ]; then
  sudo cp -a /etc/hosts /mnt/etc/
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
  sudo cp /mnt/ssh-config/* /home/cloud-admin/.ssh/
  sudo chmod 600 /home/cloud-admin/.ssh/id_rsa
  sudo chown -R $CHOWN_UID:$CHOWN_GID /home/cloud-admin/.ssh
fi

if [ -d /mnt/ca-certs ]; then
  sudo touch /run/ca-certs-epoch
  sudo cp -v /mnt/ca-certs/* /etc/pki/ca-trust/source/anchors/
  sudo update-ca-trust
  sudo bash -c 'find /etc/pki/ca-trust -newer /run/ca-certs-epoch -print0 | tar -c --null -T - | tar -C /var/lib/kolla/src -xvf -'
fi

if [ "$IPA_SERVER" != "" ]; then

  sudo touch /run/ipa-epoch

  if [ ! -f /var/lib/ipa-client/sysrestore/sysrestore.index ]; then
    # Ensure hostname is correct for ipa-client install
    sudo bash -c "cat /mnt/etc/hostname > /etc/hostname"
    # Ensure hostname -f and python socket.getfqdn() return the FQDN
    SHORT_HOSTNAME=$(hostname -s)
    LONG_HOSTNAME=$(hostname -f)
    if [ "$LONG_HOSTNAME" != "$FQDN" ]; then
      sudo sed -i -e "s/^\([0-9.]\+\)\s\+\(.*\)${SHORT_HOSTNAME}\$/\1\t\2${FQDN} ${SHORT_HOSTNAME}/" /mnt/etc/hosts
      sudo bash -c "cat /mnt/etc/hosts > /etc/hosts"
    fi

    sudo truncate -s 0 /run/openstackclient_ipa_install.yaml
    sudo chmod 600 /run/openstackclient_ipa_install.yaml
    sudo chown $CHOWN_UID:$CHOWN_GID /run/openstackclient_ipa_install.yaml

    cat <<EOF >> /run/openstackclient_ipa_install.yaml
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

    ansible-playbook -e ipa_server_user=${IPA_SERVER_USER} -e ipa_realm=${IPA_REALM} -e ipa_server_hostname=${IPA_SERVER} -e ipa_domain=${IPA_DOMAIN} /run/openstackclient_ipa_install.yaml
  fi

  # Fetch the latest IPA CA cert
  sudo ipa-certupdate

  # Add all files modified by ipa client install or cert update to kolla src
  # TODO: could use overlayfs mounts instead?
  sudo bash -c 'find /etc/krb5* /etc/sssd /etc/authselect /etc/novajoin /etc/openldap /etc/ipa /etc/pki /var/lib/ipa-client /var/lib/authselect /var/log/ipaclient-install.log -newer /run/ipa-epoch -print0 | tar -c --null -T - | tar -C /var/lib/kolla/src -xvf -'
fi
