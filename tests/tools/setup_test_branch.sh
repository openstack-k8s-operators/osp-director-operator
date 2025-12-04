#!/bin/bash

# Helper script to create a test branch in your Azure DevOps repository
# This is needed to properly test the Azure DevOps compatibility fix

set -e

echo "═══════════════════════════════════════════════════════════"
echo "Azure DevOps Test Branch Setup"
echo "═══════════════════════════════════════════════════════════"
echo ""

# Check if TEST_GIT_URL is set
if [ -z "$TEST_GIT_URL" ]; then
    echo "❌ ERROR: TEST_GIT_URL environment variable is not set"
    echo ""
    echo "Please set it to your Azure DevOps repository URL:"
    echo "  export TEST_GIT_URL=\"git@ssh.dev.azure.com:v3/org/project/repo\""
    exit 1
fi

echo "Using repository: $TEST_GIT_URL"
echo ""

# Create temporary directory
TEMP_DIR=$(mktemp -d)
echo "Creating temporary directory: $TEMP_DIR"

# Clone the repository
echo "Cloning repository..."
cd "$TEMP_DIR"
git clone "$TEST_GIT_URL" test-repo
cd test-repo

# Create test branch
echo ""
echo "Creating test branch..."
git checkout -b osp-director-test-branch

# Create a test file
echo "test content for OSP Director Operator Azure DevOps compatibility" > test-file.txt
git add test-file.txt
git commit -m "Test commit for OSP Director Operator Azure DevOps fix testing"

# Push to remote
echo ""
echo "Pushing test branch to Azure DevOps..."
git push origin osp-director-test-branch

echo ""
echo "✓ Test branch created successfully!"
echo ""
echo "Branch name: osp-director-test-branch"
echo ""
echo "You can now run the reproducer scripts:"
echo "  go run reproducer_azure_devops.go"
echo ""
echo "To clean up the test branch later:"
echo "  git push origin --delete osp-director-test-branch"
echo ""
echo "Cleaning up temporary directory..."
cd /
rm -rf "$TEMP_DIR"

echo "✓ Done!"
