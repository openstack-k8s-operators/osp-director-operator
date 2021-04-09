#!/bin/bash

set -eux

# in case of --output-only no rc is set when successful
sudo sed -i "/# We only get here if no errors/a \ \ \ \ \ \ \ \ \ \ \ \ \ \ \ \ rc=0" /usr/lib/python3.6/site-packages/tripleoclient/v1/tripleo_deploy.py
# disable running dhcp on all interfaces, setting disable_configure_safe_defaults in the interface template does not work
sudo sed -i '/^set -eux/a disable_configure_safe_defaults=true' /usr/share/openstack-tripleo-heat-templates/network/scripts/run-os-net-config.sh

mkdir -p ~/tripleo-deploy
rm -rf ~/tripleo-deploy/overcloud-ansible*
unset OS_CLOUD

sudo openstack tripleo deploy \
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
    --local-ip 192.168.25.101 \
    --deployment-user $(id -u -n) \
    --output-only
