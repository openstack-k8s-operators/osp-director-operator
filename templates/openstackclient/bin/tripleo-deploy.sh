#!/bin/bash
set -eu

mkdir -p /home/cloud-admin/tripleo-deploy/validations
if [ ! -L /var/log/validations ]; then
  sudo ln -s /home/cloud-admin/tripleo-deploy/validations /var/log/validations
fi

init() {
  if [ ! -d /home/cloud-admin/playbooks ]; then
    git clone $GIT_URL /home/cloud-admin/playbooks
  fi
  pushd /home/cloud-admin/playbooks > /dev/null
  git fetch -af
  export LATEST_BRANCH=$(git branch --sort=-committerdate -lr | head -n1 |  sed -e 's|^ *||g')
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
  pushd /home/cloud-admin/playbooks > /dev/null
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
  if git branch | grep " tripleo_deploy_working$" > /dev/null; then
    git checkout tripleo_deploy_working >/dev/null
    git reset --hard refs/tags/latest
  else
    git checkout -b tripleo_deploy_working latest
  fi

  cd tripleo-ansible*

  # TODO: for now disable opendev-validation-ceph
  # The check fails because the lvm2 package is not installed in openstackclient container image image
  # and ansible_facts include packages from undercloud.
  time ansible-playbook -i inventory.yaml \
    --private-key /home/cloud-admin/.ssh/id_rsa \
    --skip-tags opendev-validation-ceph \
    --become deploy_steps_playbook.yaml

  cp /etc/openstack/clouds.yaml ~/tripleo-deploy/
  popd > /dev/null

}

accept() {
  init
  pushd /home/cloud-admin/playbooks > /dev/null
  git tag -d latest
  git push -f --delete origin refs/tags/latest
  git tag latest refs/remotes/$LATEST_BRANCH
  git push origin --tags
  popd > /dev/null
}

usage() { echo "Usage: $0 [-d] [-p]" 1>&2; exit 1; }

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
