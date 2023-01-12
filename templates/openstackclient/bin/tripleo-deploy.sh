#!/usr/bin/env bash
set -eu
umask 022

export PWD=/home/cloud-admin
CONFIG_VERSION=${CONFIG_VERSION:?"Please set CONFIG_VERSION."}
OSP_VERSION=${OSP_VERSION:?"Please set OSP_VERSION."}
sudo bash -c 'mkdir -p /var/run/tripleo-deploy && chown '$(whoami)' /var/run/tripleo-deploy'
RUNDIR="/var/run/tripleo-deploy/$CONFIG_VERSION"
mkdir -p $RUNDIR
PGIDFILE=$RUNDIR/pgid
trap "rm -f $PGIDFILE" EXIT
# Assume PGID==$PPID which is the case when run via oc exec
# Alternatively could do something like:
# PGID=$(python3 -c 'import os; print(os.getpgid(os.getpid()))')
PGID=$PPID
echo $PGID > $PGIDFILE


WORKDIR="/home/cloud-admin/work/$CONFIG_VERSION"
mkdir -p $WORKDIR

# FIXME can this be shared
mkdir -p ~/tripleo-deploy/validations
if [ ! -L /var/log/validations ]; then
  sudo ln -s ~/tripleo-deploy/validations /var/log/validations
fi

GIT_HOST=$(echo $GIT_URL | sed -e 's|^git@\(.*\):.*|\1|g')
GIT_USER=$(echo $GIT_URL | sed -e 's|^git@.*:\(.*\)/.*|\1|g')

export GIT_SSH_COMMAND="ssh -i $WORKDIR/git_id_rsa -l git -o StrictHostKeyChecking=no"
echo $GIT_ID_RSA | sed -e 's|- |-\n|' | sed -e 's| -|\n-|'  > $WORKDIR/git_id_rsa
chmod 600 $WORKDIR/git_id_rsa

git config --global user.email "dev@null.io"
git config --global user.name "OSP Director Operator"

set_env() {
  echo -e "Exporting environment variables"
  export ANSIBLE_SSH_ARGS="-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o ControlMaster=auto -o ControlPersist=30m -o ServerAliveInterval=64 -o ServerAliveCountMax=1024 -o Compression=no -o TCPKeepAlive=yes -o VerifyHostKeyDNS=no -o ForwardX11=no -o ForwardAgent=yes -o PreferredAuthentications=publickey -T"
  export ANSIBLE_DISPLAY_FAILED_STDERR="True"
  export ANSIBLE_FORKS="24"
  export ANSIBLE_TIMEOUT="600"
  export ANSIBLE_GATHER_TIMEOUT="45"
  export ANSIBLE_SSH_RETRIES="3"
  export ANSIBLE_PIPELINING="True"
  export ANSIBLE_SCP_IF_SSH="True"
  export ANSIBLE_REMOTE_USER="cloud-admin"
  export ANSIBLE_STDOUT_CALLBACK="tripleo_dense"
  export ANSIBLE_CALLBACK_WHITELIST="tripleo_dense,tripleo_profile_tasks,tripleo_states"
  export ANSIBLE_RETRY_FILES_ENABLED="False"
  export ANSIBLE_HOST_KEY_CHECKING="False"
  export ANSIBLE_TRANSPORT="smart"
  export ANSIBLE_CACHE_PLUGIN_TIMEOUT="7200"
  export ANSIBLE_INJECT_FACT_VARS="False"
  export ANSIBLE_VARS_PLUGIN_STAGE="inventory"
  export ANSIBLE_GATHER_SUBSET="!all,min"
  export ANSIBLE_GATHERING="smart"
  export ANSIBLE_LOG_PATH="$WORKDIR/ansible.log"
  export ANSIBLE_PRIVATE_KEY_FILE="/home/cloud-admin/.ssh/id_rsa"
  export ANSIBLE_BECOME="True"
  export ANSIBLE_LIBRARY="/usr/share/ansible/tripleo-plugins/modules:/usr/share/ansible/plugins/modules:/usr/share/ceph-ansible/library:/usr/share/ansible-modules:/usr/share/ansible/library"
  export ANSIBLE_LOOKUP_PLUGINS="/usr/share/ansible/tripleo-plugins/lookup:/usr/share/ansible/plugins/lookup:/usr/share/ceph-ansible/plugins/lookup:/usr/share/ansible/lookup_plugins"
  export ANSIBLE_CALLBACK_PLUGINS="/usr/share/ansible/tripleo-plugins/callback:/usr/share/ansible/plugins/callback:/usr/share/ceph-ansible/plugins/callback:/usr/share/ansible/callback_plugins"
  export ANSIBLE_ACTION_PLUGINS="/usr/share/ansible/tripleo-plugins/action:/usr/share/ansible/plugins/action:/usr/share/ceph-ansible/plugins/actions:/usr/share/ansible/action_plugins"
  export ANSIBLE_FILTER_PLUGINS="$WORKDIR/filter:/usr/share/ansible/tripleo-plugins/filter:/usr/share/ansible/plugins/filter:/usr/share/ceph-ansible/plugins/filter:/usr/share/ansible/filter_plugins"
  export ANSIBLE_ROLES_PATH="/usr/share/ansible/tripleo-roles:/usr/share/ansible/roles:/usr/share/ceph-ansible/roles:/etc/ansible/roles:/usr/share/ansible/roles"
  export LANG="en_US.UTF-8"
  export HISTCONTROL="ignoredups"
  export HISTSIZE="1000"
}

init() {
  if [ ! -d $WORKDIR/playbooks ]; then
    git clone $GIT_URL $WORKDIR/playbooks
  fi
  pushd $WORKDIR/playbooks > /dev/null
  git fetch -af
  popd > /dev/null
}

play() {
  init
  set_env
  pushd $WORKDIR/playbooks > /dev/null

  if [ ! -d $WORKDIR/playbooks/tripleo-ansible ]; then
    echo "Playbooks directory don't exist! Run the following command to accept and tag the new playbooks first:"
    echo "  $0 -a"
    echo ""
    echo "Then re-run '$0 -p' to run the new playbooks."
    exit
  fi

  cd tripleo-ansible

  PLAYBOOK_ARG=$WORKDIR/playbooks/tripleo-ansible/${PLAYBOOK:-"deploy_steps_playbook.yaml"}
  LIMIT_ARG=""
  if [ -n "${LIMIT:-}" ]; then
    LIMIT_ARG="--limit ${LIMIT}"
  fi
  TAGS_ARG=""
  if [ -n "${TAGS:-}" ]; then
    TAGS_ARG="--tags ${TAGS}"
  fi
  # TODO: for now disable opendev-validation
  # e.g. The check fails because the lvm2 package is not installed in openstackclient container image image
  # and ansible_facts include packages from undercloud.
  SKIP_TAGS_ARG="--skip-tags opendev-validation"
  if [ -n "${SKIP_TAGS:-}" ]; then
    SKIP_TAGS_ARG+=",${SKIP_TAGS}"
  fi

  # Replace deploy identifier with unique value unless SKIP_DEPOY_IDENTIFIER set
  DEPLOY_IDENTIFIER=""
  if [ "${SKIP_DEPLOY_IDENTIFIER:-false}" != "true" ]; then
    DEPLOY_IDENTIFIER=deploy_$(date +%s%N)
  fi
  for i in $(grep -rH 'OSP_DIRECTOR_OPERATOR_DEPLOY_IDENTIFIER' $WORKDIR/playbooks/tripleo-ansible/ | cut -d : -f 1 | sort -u); do
    sed -i -e 's/OSP_DIRECTOR_OPERATOR_DEPLOY_IDENTIFIER/'${DEPLOY_IDENTIFIER}'/g' $i
  done

  STACK_ACTION_ARG=""
  if [ "${OSP_VERSION}" = "16.2" ]; then
    # We need stack_action to be set correctly in 16.2 as the network_config and octavia roles in tripleo-ansible
    # rely on it.
    # As we can't use the existance of a heat stack, instead use the existance of /var/lib/tripleo-config on any
    # overcloud host (created by the tripleo-bootstrap role) to determine if this is a "CREATE" or an "UPDATE"
    cat <<EOF > set_stack_action_playbook.yaml
{{`---
- hosts: overcloud
  name: Determine the correct stack action for 16.2 deployments
  gather_facts: false
  become: false
  tasks:
    - stat:
        path: /var/lib/tripleo-config
      register: deploy_exists
      ignore_unreachable: yes
    - set_fact:
        deploy_exists: "{{ hostvars['undercloud'].deploy_exists|default('0')|int + 1 }}"
      when: deploy_exists.stat.exists|default(False)
      delegate_to: undercloud
      delegate_facts: yes
    - copy:
        dest: stack_action_override.yaml
        content: |
          stack_action: {{ 'UPDATE' if hostvars['undercloud'].deploy_exists|default('0')|int > 0 else 'CREATE' }}
      delegate_to: undercloud
      run_once: yes
EOF`}}

    ansible-playbook \
      -i $WORKDIR/playbooks/tripleo-ansible/tripleo-ansible-inventory.yaml \
      ${LIMIT_ARG} \
      set_stack_action_playbook.yaml
    STACK_ACTION_ARG="-e @stack_action_override.yaml"
  fi

  ansible-playbook \
    -i $WORKDIR/playbooks/tripleo-ansible/tripleo-ansible-inventory.yaml \
    ${LIMIT_ARG} \
    ${TAGS_ARG} \
    ${SKIP_TAGS_ARG} \
    ${STACK_ACTION_ARG} \
    ${PLAYBOOK_ARG}

  # Only created when keystone is deployed
  if [ -e /etc/openstack/clouds.yaml ]; then
    mkdir -p ~/.config/openstack
    sudo cp -f /etc/openstack/clouds.yaml ~/.config/openstack/clouds.yaml
    sudo chown cloud-admin: ~/.config/openstack/clouds.yaml
  fi

  sudo /usr/local/bin/tripleo-export-ceph "${WORKDIR}"

  popd > /dev/null

}

accept() {
  init
  pushd $WORKDIR/playbooks > /dev/null
  git tag -d latest || true
  git push -f --delete origin refs/tags/latest || true
  git tag latest remotes/origin/$CONFIG_VERSION
  git push origin --tags

  # checkout accepted code
  if git branch | grep " tripleo_deploy_working$" > /dev/null; then
    git checkout tripleo_deploy_working >/dev/null
    git reset --hard refs/tags/latest
  else
    git checkout -b tripleo_deploy_working latest
  fi

  popd > /dev/null
}

accept
play
