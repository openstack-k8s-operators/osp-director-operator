#!/bin/bash

set -eux

render() {
  # disable running dhcp on all interfaces, setting disable_configure_safe_defaults in the interface template does not work
  sudo sed -i '/^set -eux/a disable_configure_safe_defaults=true' /usr/share/openstack-tripleo-heat-templates/network/scripts/run-os-net-config.sh

  rm -rf /home/cloud-admin/tripleo-deploy/overcloud-ansible*
  if [ ! -L /var/log/validations ]; then
    sudo ln -s /home/cloud-admin/tripleo-deploy/validations /var/log/validations
  fi

  unset OS_CLOUD

  sudo openstack tripleo deploy \
    --templates /usr/share/openstack-tripleo-heat-templates \
    -r ROLESFILE \
    -n /usr/share/openstack-tripleo-heat-templates/network_data.yaml \
    -e /usr/share/openstack-tripleo-heat-templates/overcloud-resource-registry-puppet.yaml \
    -e /usr/share/openstack-tripleo-heat-templates/environments/network-isolation.yaml \
    -e /usr/share/openstack-tripleo-heat-templates/environments/deployed-server-environment.yaml \
    -e /usr/share/openstack-tripleo-heat-templates/environments/docker-ha.yaml \
{{- range $key, $value := .TripleoDeployFiles }}
    -e ~/config/{{ $key }} \
{{- end }}
{{- range $key, $value := .TripleoCustomDeployFiles }}
    -e ~/config-custom/{{ $key }} \
{{- end }}
    --stack overcloud \
    --output-dir ~/tripleo-deploy \
    --standalone \
    --local-ip $(ip a s net1 | sed -En -e 's/.*inet ([0-9.]+).*/\1/p') \
    --deployment-user $(id -u -n) \
    --output-only

  cd ~/tripleo-deploy
  output_dir=$(ls -dtr overcloud-ansible-* | tail -1)
  ln -sf ${output_dir} overcloud-ansible
  cd ${output_dir}
  # we run with --standalone, therefore have to remove transport=local from ansible.cfg
  sed -i '/transport/d' ansible.cfg

  # For standalone role tripleo_deploy sets root for the ansible_ssh_user in the inventory.
  # Change it to cloud-admin
  sed -i 's/ansible_ssh_user: root/ansible_ssh_user: cloud-admin/g' ~/tripleo-deploy/overcloud-ansible/inventory.yaml

}

play() {

  cd ~/tripleo-deploy/overcloud-ansible
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
        r)
            render
            ;;
        p)
            play
            ;;
        *)
            usage
            exit 0
            ;;
    esac
done
