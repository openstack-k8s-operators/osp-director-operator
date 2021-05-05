#!/bin/bash

set -eux

unset OS_CLOUD
export OS_AUTH_TYPE=none
export OS_ENDPOINT="http://{{ .HeatServiceName }}:8004/v1/admin"

HEAT_COUNT=0
until openstack stack list &> /dev/null || [ "$HEAT_COUNT" -gt 180 ]; do
  HEAT_COUNT=$(($HEAT_COUNT + 1))
  echo "waiting for Heat API to startup..."
  sleep 2
done

# create a temporary scratch directory to assemble the Heat templates
TEMPLATES_DIR=$HOME/tripleo-deploy-scratch/tripleo-heat-installer-templates
rm -Rf "$TEMPLATES_DIR"
mkdir -p $TEMPLATES_DIR

cp -a /usr/share/openstack-tripleo-heat-templates/* $TEMPLATES_DIR
pushd $TEMPLATES_DIR

# extract any tar files into the $TEMPLATES_DIR
{{- if .TripleoTarballFiles }}
{{- range $key, $value := .TripleoTarballFiles }}
tar -xvf /home/cloud-admin/tht-tars/{{ $key }}
{{- end }}
{{- end }}

# copy to editable dir config-tmp
rm -Rf ~/config-tmp
mkdir -p ~/config-tmp
cp ~/config/* ~/config-tmp
cp ~/config-custom/* ~/config-tmp
#FIXME: get rid of /usr/share/openstack-tripleo-heat-templates/ and use relative paths in dev-tools!
sed -e "s|/usr/share/openstack\-tripleo\-heat\-templates|\.|" -i ~/config-tmp/*.yaml
# copy to our temp t-h-t dir
cp -a ~/config-tmp/* "$TEMPLATES_DIR/"

# if ROLES_FILE is set we overwrite the default t-h-t version (FIXME: can this be removed now that we support tarballs?)
ROLES_FILE="{{ .RolesFile }}"
if [ -n "$ROLES_FILE" ]; then
  cp /home/cloud-admin/config-custom/$ROLES_FILE $TEMPLATES_DIR/roles_data.yaml
fi

python3 tools/process-templates.py -r $TEMPLATES_DIR/roles_data.yaml -n $TEMPLATES_DIR/network_data.yaml

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
    endpoint='http://{{ .HeatServiceName }}:%s/v1/admin' % api_port,
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

# copy to our persistent volume mount so it shows up in the 'openstackclient' pod
cp -a /home/cloud-admin/ansible/* /var/cloud-admin/ansible/
