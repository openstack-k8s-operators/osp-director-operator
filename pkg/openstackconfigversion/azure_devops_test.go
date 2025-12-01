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
	"errors"
	"os"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	crypto_ssh "golang.org/x/crypto/ssh"
)

// TestAzureDevOpsCompatibility tests that the Azure DevOps compatibility fix works
// This test demonstrates the issue where Clone works but CommitObject fails with "Object not found"
// when using Azure DevOps repositories without the ThinPack capability workaround
//
// ENVIRONMENT VARIABLES REQUIRED:
//
//	TEST_GIT_URL          - The Azure DevOps git repository URL to test
//	                        Example: "git@ssh.dev.azure.com:v3/org/project/repo"
//	TEST_SSH_KEY_PATH     - Path to SSH private key for git authentication
//	                        Example: "/home/user/.ssh/id_rsa"
//
// The test will be SKIPPED if either environment variable is not set or if the SSH key
// file cannot be read.
//
// RUNNING THE TEST:
//
//	export TEST_GIT_URL="git@ssh.dev.azure.com:v3/org/project/repo"
//	export TEST_SSH_KEY_PATH="/path/to/ssh/private/key"
//	go test -v -run TestAzureDevOpsCompatibility
//
// Or with make:
//
//	export TEST_GIT_URL="git@ssh.dev.azure.com:v3/org/project/repo"
//	export TEST_SSH_KEY_PATH="/path/to/ssh/private/key"
//	make test
func TestAzureDevOpsCompatibility(t *testing.T) {
	// Check for required environment variables
	privateKeyPath := os.Getenv("TEST_SSH_KEY_PATH")
	if privateKeyPath == "" {
		t.Skip("Skipping test: TEST_SSH_KEY_PATH environment variable not set")
	}

	gitURL := os.Getenv("TEST_GIT_URL")
	if gitURL == "" {
		t.Skip("Skipping test: TEST_GIT_URL environment variable not set")
	}

	pkey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		t.Skipf("Skipping test: cannot read SSH key: %v", err)
	}

	endpoint, err := transport.NewEndpoint(gitURL)
	if err != nil {
		t.Fatalf("parse git url failed: %s", err)
	}

	publicKeys, err := ssh.NewPublicKeys(endpoint.User, pkey, "")
	if err != nil {
		t.Fatalf("generate publickeys failed: %s", err)
	}
	publicKeys.HostKeyCallback = crypto_ssh.InsecureIgnoreHostKey()

	// First attempt - this may fail with Azure DevOps
	repo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:  gitURL,
		Auth: publicKeys,
	})

	// If Clone fails with ErrEmptyUploadPackRequest, set ThinPack capability and retry
	if err != nil {
		t.Logf("Clone failed: %v", err)
		if errors.Is(err, transport.ErrEmptyUploadPackRequest) {
			t.Log("Setting ThinPack capability for Azure DevOps compatibility")
			transport.UnsupportedCapabilities = []capability.Capability{
				capability.ThinPack,
			}

			repo, err = git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
				URL:  gitURL,
				Auth: publicKeys,
			})
			if err != nil {
				t.Fatalf("Clone failed after setting ThinPack: %v", err)
			}
		} else {
			t.Fatalf("Clone failed: %v", err)
		}
	}

	t.Log("Clone successful")

	// Create remote and list refs
	rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{gitURL},
	})

	refs, err := rem.List(&git.ListOptions{Auth: publicKeys})
	if err != nil {
		t.Fatalf("Failed to list remote: %v", err)
	}

	t.Logf("Found %d refs", len(refs))

	// Try to get commit objects for branches
	for _, ref := range refs {
		if ref.Name().IsBranch() {
			// Skip default branches
			if ref.Name() == "refs/heads/master" || ref.Name() == "refs/heads/main" || ref.Name() == "HEAD" {
				continue
			}

			t.Logf("Checking ref: %s", ref.Name())

			// This is where the "Object not found" error would occur without the ThinPack fix
			commit, err := repo.CommitObject(ref.Hash())
			if err != nil {
				t.Errorf("Failed to get commit for %s: %v", ref.Name(), err)
				continue
			}

			t.Logf("Successfully retrieved commit for %s: %s", ref.Name(), commit.Hash.String()[:8])
		}
	}
}
