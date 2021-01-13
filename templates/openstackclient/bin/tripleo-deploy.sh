#!/bin/bash

set -eux

sed -i "/# We only get here if no errors/a \ \ \ \ \ \ \ \ \ \ \ \ \ \ \ \ rc=0" /usr/lib/python3.6/site-packages/tripleoclient/v1/tripleo_deploy.py
sed -i "s/clouds_home_dir = .*/clouds_home_dir = os.path.expanduser('~')/" /usr/lib/python3.6/site-packages/tripleoclient/utils.py
# disable running dhcp on all interfaces, setting disable_configure_safe_defaults in the interface template does not work
sed -i '/^set -eux/a disable_configure_safe_defaults=true' /usr/share/openstack-tripleo-heat-templates/network/scripts/run-os-net-config.sh

rm -rf ~/tripleo-deploy/overcloud-ansible*
unset OS_CLOUD

openstack tripleo deploy \
    --templates /usr/share/openstack-tripleo-heat-templates \
    -r /usr/share/openstack-tripleo-heat-templates/roles_data.yaml \
    -n /usr/share/openstack-tripleo-heat-templates/network_data.yaml \
    -e /usr/share/openstack-tripleo-heat-templates/overcloud-resource-registry-puppet.yaml \
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
sed -i '/transport/d' ansible.cfg
sed -i "/blockinfile/a \ \ \ \ unsafe_writes: yes" /usr/share/ansible/roles/tripleo-hosts-entries/tasks/main.yml

# change ansible_ssh_user to cloud-admin, todo deployment-user should be also set in the inventroy as ansible_ssh_user
sed -i 's/ansible_ssh_user: root/ansible_ssh_user: cloud-admin/g' ~/tripleo-deploy/overcloud-ansible/inventory.yaml

time ansible-playbook -i inventory.yaml --become deploy_steps_playbook.yaml

cp /etc/openstack/clouds.yaml ~/tripleo-deploy/
