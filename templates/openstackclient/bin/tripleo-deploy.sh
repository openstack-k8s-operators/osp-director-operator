#!/usr/bin/env bash
set -eu

mkdir -p /home/cloud-admin/tripleo-deploy/validations
if [ ! -L /var/log/validations ]; then
  sudo ln -s /home/cloud-admin/tripleo-deploy/validations /var/log/validations
fi

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
  export ANSIBLE_LOG_PATH="/home/cloud-admin/ansible.log"
  export ANSIBLE_PRIVATE_KEY_FILE="/home/cloud-admin/.ssh/id_rsa"
  export ANSIBLE_BECOME="True"
  export ANSIBLE_LIBRARY="/usr/share/ansible/tripleo-plugins/modules:/usr/share/ansible/plugins/modules:/usr/share/ceph-ansible/library:/usr/share/ansible-modules:/usr/share/ansible/library"
  export ANSIBLE_LOOKUP_PLUGINS="/usr/share/ansible/tripleo-plugins/lookup:/usr/share/ansible/plugins/lookup:/usr/share/ceph-ansible/plugins/lookup:/usr/share/ansible/lookup_plugins"
  export ANSIBLE_CALLBACK_PLUGINS="/usr/share/ansible/tripleo-plugins/callback:/usr/share/ansible/plugins/callback:/usr/share/ceph-ansible/plugins/callback:/usr/share/ansible/callback_plugins"
  export ANSIBLE_ACTION_PLUGINS="/usr/share/ansible/tripleo-plugins/action:/usr/share/ansible/plugins/action:/usr/share/ceph-ansible/plugins/actions:/usr/share/ansible/action_plugins"
  export ANSIBLE_FILTER_PLUGINS="/home/cloud-admin/filter:/usr/share/ansible/tripleo-plugins/filter:/usr/share/ansible/plugins/filter:/usr/share/ceph-ansible/plugins/filter:/usr/share/ansible/filter_plugins"
  export ANSIBLE_ROLES_PATH="/usr/share/ansible/tripleo-roles:/usr/share/ansible/roles:/usr/share/ceph-ansible/roles:/etc/ansible/roles:/usr/share/ansible/roles"
  export LANG="en_US.UTF-8"
  export HISTCONTROL="ignoredups"
  export HISTSIZE="1000"
}

init() {
  if [ ! -d /home/cloud-admin/playbooks ]; then
    git clone $GIT_URL /home/cloud-admin/playbooks
  fi
  pushd /home/cloud-admin/playbooks > /dev/null
  git fetch -af
  # exclude any '-> HEAD'... lines if they exist in the supplied git repo
  export LATEST_BRANCH=$(git branch --sort=-committerdate -lr | grep -v ' -> ' | head -n1 |  sed -e 's|^ *||g')
  if [ -z "$LATEST_BRANCH" ]; then
    echo "The Git repo is empty, try again once the OpenStackPlaybookGenerator finishes and pushes a commit."
    exit 1
  fi
  if ! git tag | grep ^latest$ &>/dev/null; then
    echo "Initializing 'latest' git tag with refs/remotes/$LATEST_BRANCH"
    git tag latest refs/remotes/$LATEST_BRANCH
    git push origin --tags
  fi
  popd > /dev/null
}

diff() {
  init
  pushd /home/cloud-admin/playbooks > /dev/null
  # NOTE: this excludes _server_id lines which get modified each time Heat runs
  # NOTE: we ignore the inventory.yaml as it gets randomly sorted each time
  # NOTE: we ignore ansible.cfg which changes each time
  git difftool -y --extcmd="diff --recursive --unified=1 --color --ignore-matching-lines='_server_id:' $@" refs/tags/latest $LATEST_BRANCH -- . ':!*inventory.yaml' ':!*ansible.cfg'
  popd > /dev/null
}

play() {
  init
  set_env
  pushd /home/cloud-admin/playbooks > /dev/null

  if [ ! -d /home/cloud-admin/playbooks/tripleo-ansible ]; then
    echo "Playbooks directory don't exist! Run the following command to accept and tag the new playbooks first:"
    echo "  $0 -a"
    echo ""
    echo "Then re-run '$0 -p' to run the new playbooks."
    exit
  fi

  if ! git diff --exit-code --no-patch refs/tags/latest remotes/$LATEST_BRANCH; then
    echo "New playbooks detected. Run the following command to view a diff of the changes:"
    echo "  $0 -d"
    echo ""
    echo "To accept and retag the new playbooks run:"
    echo "  $0 -a"
    echo ""
    echo "Then re-run '$0 -p' to run the new playbooks."
    exit
  fi

  cd tripleo-ansible

  # TODO: for now disable opendev-validation
  # e.g. The check fails because the lvm2 package is not installed in openstackclient container image image
  # and ansible_facts include packages from undercloud.
  ansible-playbook \
    -i /home/cloud-admin/playbooks/tripleo-ansible/tripleo-ansible-inventory.yaml \
    --skip-tags opendev-validation \
    /home/cloud-admin/playbooks/tripleo-ansible/deploy_steps_playbook.yaml

  mkdir -p ~/.config/openstack
  cp -f /etc/openstack/clouds.yaml ~/.config/openstack/clouds.yaml
  popd > /dev/null

}

accept() {
  init
  pushd /home/cloud-admin/playbooks > /dev/null
  git tag -d latest
  git push -f --delete origin refs/tags/latest
  git tag latest refs/remotes/$LATEST_BRANCH
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

usage() { echo "Usage: $0 [-d] [-p] [-a]" 1>&2; exit 1; }

[[ $# -eq 0 ]] && usage

while getopts ":dpa" arg; do
    case "${arg}" in
        d)
            diff
            ;;
        p)
            play
            ;;
        a)
            accept
            ;;
        *)
            usage
            ;;
    esac
done
