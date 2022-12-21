/*
Copyright 2021 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package openstackconfigversion

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	git "github.com/go-git/go-git/v5"
	config "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"

	crypto_ssh "golang.org/x/crypto/ssh"

	"github.com/go-logr/logr"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// scpSyntaxRe matches the SCP-like addresses used by Git to access
// repositories by SSH.
// https://go-review.googlesource.com/c/go/+/12226/3/src/cmd/go/vcs.go
var scpSyntaxRe = regexp.MustCompile(`^([a-zA-Z0-9_]+)@([a-zA-Z0-9._-]+):(.*)$`)

// Patch is an implementation of fdiff.Patch interface
type Patch struct {
	message     string
	filePatches []diff.FilePatch
}

// FilePatches .
func (p Patch) FilePatches() []diff.FilePatch {
	return p.filePatches
}

// Message .
func (p Patch) Message() string {
	return p.message
}

// filterPatches internal function which is used to strip out '_server_id', ansible inventory, and config changes from our git diff results
func filterPatches(filePatches []diff.FilePatch) []diff.FilePatch {
	var retPatches []diff.FilePatch

	m1 := regexp.MustCompile(`_server_id:`)
	for _, fp := range filePatches {
		from, to := fp.Files()

		if len(fp.Chunks()) == 0 {
			continue
		}
		if from == nil {
			// New File is created.
			retPatches = append(retPatches, fp)
		} else if to == nil {
			// File is deleted.
			retPatches = append(retPatches, fp)

		} else if from.Path() == to.Path() {

			if strings.HasSuffix("tripleo-ansible/tripleo-ansible-inventory.yaml", from.Path()) || strings.HasSuffix("tripleo-ansible/ansible.cfg", from.Path()) {
				continue
			}

			var chunks []diff.Chunk
			for _, chunk := range fp.Chunks() {
				chunk.Type()
				s := chunk.Content()
				if len(s) == 0 {
					continue
				}

				if chunk.Type() == diff.Add {

					if !m1.Match([]byte(s)) {
						chunks = append(chunks, chunk)
					}
				}

			}
			if len(chunks) > 0 {
				retPatches = append(retPatches, fp)
			}
		} else {
			// file is renamed
			retPatches = append(retPatches, fp)
		}
	}

	return retPatches
}

// CreateKnownHosts func
func CreateKnownHosts(dialAddr string, addr net.Addr, key crypto_ssh.PublicKey) error {
	sshDir := fmt.Sprintf("%s/.ssh", os.Getenv("HOME"))
	err := os.MkdirAll(sshDir, 0700)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("create .ssh dir failed: %w", err)
	}

	knownHostsEntry := fmt.Sprintf("%s %s %s\n", strings.Split(dialAddr, ":")[0], key.Type(), base64.StdEncoding.EncodeToString(key.Marshal()))

	err = os.WriteFile(sshDir+"/known_hosts", []byte(knownHostsEntry), 0600)
	if err != nil {
		return fmt.Errorf("writing known_hosts failed: %w", err)
	}

	return nil
}

// SyncGit func
func SyncGit(
	ctx context.Context,
	inst *ospdirectorv1beta1.OpenStackConfigGenerator,
	client client.Client,
	log logr.Logger,
) (map[string]ospdirectorv1beta1.OpenStackConfigVersion, error) {

	configVersions := make(map[string]ospdirectorv1beta1.OpenStackConfigVersion)

	// Check if this Secret already exists
	foundSecret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{Name: inst.Spec.GitSecret, Namespace: inst.Namespace}, foundSecret)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			return nil, fmt.Errorf("GitRepo secret was not found.: %w", err)
		}
		return nil, err
	}

	pkey := foundSecret.Data["git_ssh_identity"]
	if err != nil {
		return nil, fmt.Errorf("parse private key failed: %w", err)
	}

	publicKeys, err := ssh.NewPublicKeys("git", pkey, "")
	if err != nil {
		return nil, fmt.Errorf("generate publickeys failed: %w", err)
	}

	gitURL := string(foundSecret.Data["git_url"])
	var repoURL *url.URL

	if m := scpSyntaxRe.FindStringSubmatch(gitURL); m != nil {
		// Match SCP-like syntax and convert it to a URL.
		// Eg, "git@github.com:user/repo" becomes
		// "ssh://git@github.com/user/repo".
		repoURL = &url.URL{
			Scheme: "ssh",
			User:   url.User(m[1]),
			Host:   m[2],
			Path:   m[3],
		}
	} else {
		repoURL, err = url.Parse(gitURL)
		if err != nil {
			return nil, fmt.Errorf("parse git_url failed: %w", err)
		}
	}

	repoHost := repoURL.Hostname()
	repoPort := "22"
	if repoURL.Port() != "" {
		repoPort = repoURL.Port()
	}
	repoAddr := fmt.Sprintf("%s:%s", repoHost, repoPort)

	log.Info(fmt.Sprintf("Git repository URL %+v", repoURL.String()))
	log.Info(fmt.Sprintf("Git repository Addr %s", repoAddr))
	log.Info(fmt.Sprintf("Git repository User %s", repoURL.User.Username()))
	log.Info(fmt.Sprintf("Git repository Path %s", repoURL.Path))

	signer, err := crypto_ssh.ParsePrivateKey(pkey)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %w", err)
	}

	auths := []crypto_ssh.AuthMethod{crypto_ssh.PublicKeys(signer)}

	sshConfig := &crypto_ssh.ClientConfig{
		User:            repoURL.User.Username(),
		Auth:            auths,
		HostKeyCallback: CreateKnownHosts,
	}

	sshClient, err := crypto_ssh.Dial("tcp", repoAddr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("connect to git_url failed: %v", err)
	}
	defer sshClient.Close()

	repo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:  repoURL.String(),
		Auth: publicKeys,
	})

	// Failed to create Git repo: URL field is required
	if err != nil {
		return nil, fmt.Errorf("Failed to create Git repo: %w", err)
	}

	// Create the remote with repository URL
	rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL.String()},
	})

	refs, err := rem.List(&git.ListOptions{
		Auth: publicKeys,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to list remote: %w", err)
	}

	m1 := regexp.MustCompile(`/`)
	for _, ref := range refs {
		if ref.Name().IsBranch() {
			if ref.Name() == "refs/heads/master" || ref.Name() == "HEAD" {
				continue
			}
			commit, err := repo.CommitObject(ref.Hash())
			if err != nil {
				return nil, fmt.Errorf("Failed to get commit object: %w", err)
			}

			latest, err := repo.Tag("latest")
			if err != nil {
				log.Info(fmt.Sprintf("Failed to get 'latest' tag: %s\n. No git diffs will be calculated.", err.Error()))
			}
			var configVersion ospdirectorv1beta1.OpenStackConfigVersion

			if latest != nil {
				commitLatest, err := repo.CommitObject(latest.Hash())
				if err != nil {
					return nil, err
				}
				patch, err := commit.PatchContext(ctx, commitLatest)

				filterPatch := Patch{
					message:     patch.Message(),
					filePatches: filterPatches(patch.FilePatches()),
				}
				if err != nil {
					return nil, err
				}
				buffer := bytes.NewBuffer(nil)
				e := diff.NewUnifiedEncoder(buffer, 0)
				err = e.Encode(filterPatch)
				if err != nil {
					return nil, err
				}
				diff := truncateDiff(buffer.String(), log)
				configVersion = ospdirectorv1beta1.OpenStackConfigVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      m1.Split(ref.Name().String(), -1)[2],
						Namespace: inst.Namespace,
					},
					Spec: ospdirectorv1beta1.OpenStackConfigVersionSpec{Hash: m1.Split(ref.Name().String(), -1)[2], Diff: diff, ConfigGeneratorName: inst.Name}}
			} else {
				configVersion = ospdirectorv1beta1.OpenStackConfigVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      m1.Split(ref.Name().String(), -1)[2],
						Namespace: inst.Namespace,
					},
					Spec: ospdirectorv1beta1.OpenStackConfigVersionSpec{Hash: m1.Split(ref.Name().String(), -1)[2], Diff: "", ConfigGeneratorName: inst.Name}}
			}
			configVersions[ref.Hash().String()] = configVersion
		}

	}

	return configVersions, nil
}

// truncateDiff  truncate the diff size to less than 512KB
func truncateDiff(diff string, log logr.Logger) string {
	if len(diff) > 524800 {
		log.Info(fmt.Sprintf("Git diff was truncated to 512KB.\n %s", diff[:524800]))
		return diff[:524800]
	}
	return diff
}
