#!/bin/bash

set -eux

unset OS_CLOUD
export OS_AUTH_TYPE=none
export OS_ENDPOINT=http://heat-test:8004/v1/admin

# if ROLES_FILE is set we overwrite the default t-h-t version
ROLES_FILE="{{ .RolesFile }}"
if [ -n "$ROLES_FILE" ]; then
  cp /home/cloud-admin/config-custom/$ROLES_FILE $TEMPLATES_DIR/roles_data.yaml
fi

TEMPLATES_DIR=$HOME/tripleo-deploy-test/tripleo-heat-installer-templates
rm -Rf "$TEMPLATES_DIR"
mkdir -p $TEMPLATES_DIR

cp -a /usr/share/openstack-tripleo-heat-templates/* $TEMPLATES_DIR
pushd $TEMPLATES_DIR
python3 tools/process-templates.py -r $TEMPLATES_DIR/roles_data.yaml -n /usr/share/openstack-tripleo-heat-templates/network_data.yaml

#FIXME: get rid of /usr/share/openstack-tripleo-heat-templates/ and use relative paths
# copy to editable dir config-tmp
rm -Rf ~/config-tmp
mkdir -p ~/config-tmp
cp ~/config/* ~/config-tmp
cp ~/config-custom/* ~/config-tmp
# make patch relative
sed -e "s|/usr/share/openstack\-tripleo\-heat\-templates|\.|" -i ~/config-tmp/*.yaml
# copy to our temp t-h-t dir
cp -a ~/config-tmp/* "$TEMPLATES_DIR/"

# disable running dhcp on all interfaces, setting disable_configure_safe_defaults in the interface template does not work
sudo sed -i '/^set -eux/a disable_configure_safe_defaults=true' ./network/scripts/run-os-net-config.sh

# default to standard container image prepare but user environments can override this setting
openstack tripleo container image prepare default --output-env-file container-image-prepare.yaml
openstack tripleo container image prepare \
    -e container-image-prepare.yaml \
{{- range $key, $value := .TripleoDeployFiles }}
    -e {{ $key }} \
{{- end }}
{{- range $key, $value := .TripleoCustomDeployFiles }}
    -e {{ $key }} \
{{- end }}
 -r roles_data.yaml --output-env-file=tripleo-overcloud-images.yaml

mkdir -p ~/tripleo-deploy
rm -rf ~/tripleo-deploy/overcloud-ansible*

#FIXME: need a way to generate the ~/tripleo-overcloud-passwords.yaml below
time openstack stack create --wait \
    -e $TEMPLATES_DIR/overcloud-resource-registry-puppet.yaml \
    -e $TEMPLATES_DIR/tripleo-overcloud-images.yaml \
    -e $TEMPLATES_DIR/environments/network-isolation.yaml \
    -e $TEMPLATES_DIR/environments/deployed-server-environment.yaml \
    -e $TEMPLATES_DIR/environments/docker-ha.yaml \
{{- range $key, $value := .TripleoDeployFiles }}
    -e {{ $key }} \
{{- end }}
{{- range $key, $value := .TripleoCustomDeployFiles }}
    -e {{ $key }} \
{{- end }}
    -e ~/config-custom/tripleo-overcloud-passwords.yaml \
    -t overcloud.yaml overcloud

mkdir -p /home/cloud-admin/ansible

# FIXME: there is no local 'config-download' command in OSP 16.2 (use tripleoclient config-download in OSP 17)
/usr/bin/python3 - <<"EOF_PYTHON"
from tripleoclient import utils as oooutils
from osc_lib import utils
from tripleo_common.inventory import TripleoInventory
from tripleo_common.actions import ansible
import sys
import os

API_NAME = 'tripleoclient'
API_VERSIONS = {
    '1': 'heatclient.v1.client.Client',
}
api_port='8004'
heat_client = utils.get_client_class(
    API_NAME,
    '1',
    API_VERSIONS)
client = heat_client(
    endpoint='http://heat-test:%s/v1/admin' % api_port,
    username='admin',
    password='fake',
    region_name='regionOne',
    token='fake',
)
out_dir = oooutils.download_ansible_playbooks(client, 'overcloud', output_dir='/home/cloud-admin/ansible')

inventory = TripleoInventory(
    hclient=client,
    plan_name='overcloud',
    ansible_ssh_user='cloud-admin')

extra_vars = {
    'Standalone': {
        'ansible_connection': 'local',
        'ansible_python_interpreter': sys.executable,
        }
    }
inv_path = os.path.join(out_dir, 'inventory.yaml')
inventory.write_static_inventory(inv_path, extra_vars)

# NOTE: we don't use transport=local like tripleoclient standalone
ansible.write_default_ansible_cfg(
    out_dir,
    'cloud-admin',
    ssh_private_key=None)

EOF_PYTHON
