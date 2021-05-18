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
  pushd /home/cloud-admin/playbooks
  git fetch -af
  export LATEST_BRANCH=$(git branch --sort=-committerdate -lr | head -n1 |  sed -e 's|^ *||g')
  if [ -z "$LATEST_BRANCH" ]; then
    echo "The Git repo is empty, try again once the OpenStackPlaybookGenerator finishes and pushes a commit."
  fi
  if ! git tag | grep ^latest$; then
    git tag latest refs/remotes/$LATEST_BRANCH
    git push origin --tags
  fi
  popd
}

diff() {
  init
  pushd /home/cloud-admin/playbooks
  # NOTE: this excludes _server_id lines which get modified each time Heat runs
  # NOTE: we ignore the inventory.yaml as it gets randomly sorted each time
  # NOTE: we ignore ansible.cfg which changes each time
  git difftool -y --extcmd="diff --recursive --unified=1 --color --ignore-matching-lines='_server_id:' $@" refs/tags/latest $LATEST_BRANCH -- . ':!*inventory.yaml' ':!*ansible.cfg'
  popd
}

play() {
  init
  pushd /home/cloud-admin/playbooks
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
  if git branch | grep " latest$"; then
    git checkout latest
    git reset --hard latest latest
  else
    git checkout -b latest latest
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

}

accept() {
  init
  pushd /home/cloud-admin/playbooks
  git tag -d latest
  git push -f --delete origin refs/tags/latest
  git tag latest refs/remotes/$LATEST_BRANCH
  git push origin --tags
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

