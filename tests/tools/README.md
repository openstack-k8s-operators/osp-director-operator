# Azure DevOps Git Repository Compatibility Fix

This directory contains test tools and documentation for the Azure DevOps git repository compatibility fix.

## Issue Summary

When using Azure DevOps as a git repository with the OSP Director Operator, users encountered issues where git operations would fail with "Object not found" errors.

## Problem Description

### Root Cause

Azure DevOps has protocol incompatibilities with go-git's default capability settings. Specifically:

1. **Go-git default capabilities**: `[multi_ack, multi_ack_detailed, thin-pack]`
2. **Issue**: Azure DevOps may accept these capabilities but transfer incomplete git objects
3. **Result**: `Clone()` succeeds, but `CommitObject()` fails with `ErrObjectNotFound`

### Two Failure Scenarios

**Scenario 1: Clone Fails**
- Clone fails immediately with `ErrEmptyUploadPackRequest`
- Easy to detect and handle at Clone time

**Scenario 2: Clone Succeeds, CommitObject Fails**
- Clone succeeds but transfers incomplete objects
- Error only appears when trying to access commit objects
- More subtle and harder to detect

## The Solution

### Error-Based Detection and Retry

The fix uses an error-based detection approach:

1. **Try the operation** with default capabilities
2. **Detect errors** when they occur (Clone or CommitObject)
3. **Apply the fix**: Set `transport.UnsupportedCapabilities = [ThinPack]`
4. **Retry** the entire operation

### Implementation Details

```go
// At Clone time (Scenario 1)
repo, err := git.Clone(...)
if err != nil && errors.Is(err, transport.ErrEmptyUploadPackRequest) {
    transport.UnsupportedCapabilities = []capability.Capability{capability.ThinPack}
    repo, err = git.Clone(...) // Retry
}

// At CommitObject time (Scenario 2)
commit, err := repo.CommitObject(ref.Hash())
if err != nil && errors.Is(err, plumbing.ErrObjectNotFound) {
    // Check if fix already applied
    if !azureDevOpsFixApplied {
        transport.UnsupportedCapabilities = []capability.Capability{capability.ThinPack}
        return SyncGit(...) // Retry entire operation
    }
}
```

### Key Technical Details

1. **Global State**: `transport.UnsupportedCapabilities` is a global variable in go-git
2. **Persistence**: Once set, it affects all subsequent git operations in the process
3. **Recursive Retry**: On CommitObject failure, we retry the entire `SyncGit()` operation
4. **Idempotent**: Check prevents infinite retry loops

### Why This Approach?

**Advantages:**
- ✅ No URL pattern matching needed
- ✅ Only applies fix when actually required
- ✅ Self-correcting behavior
- ✅ Works with any git server (not just Azure DevOps)
- ✅ Error-driven (clear cause and effect)

**Trade-offs:**
- One-time retry cost (~2x time on first failure)
- Relies on global mutable state
- Recursive call might not be immediately obvious

## Testing Tools

### Reproducer

**File**: `reproducer_azure_devops.go`

A standalone reproducer for testing and validating the Azure DevOps compatibility fix.

**Usage:**
```bash
cd tests/tools

# Set environment variables
export TEST_GIT_URL="git@ssh.dev.azure.com:v3/org/project/repo"
export TEST_SSH_KEY_PATH="/path/to/ssh/private/key"

# Run the reproducer
go run reproducer_azure_devops.go
```

**What It Tests:**
1. Clones a git repository (may fail with Azure DevOps)
2. Attempts to access commit objects
3. Detects "Object not found" errors
4. Applies the ThinPack capability fix
5. Retries the operation successfully

**Expected Output (Azure DevOps repository):**
```
Testing Azure DevOps compatibility with: git@ssh.dev.azure.com:v3/org/project/repo
Cloning repository...
✓ Clone successful
Testing branch: refs/heads/main
✗ CommitObject failed with ErrObjectNotFound
  Applying Azure DevOps fix: setting UnsupportedCapabilities = [ThinPack]

Retrying git operations with ThinPack fix...
Cloning repository...
✓ Clone successful
Testing branch (retry): refs/heads/main
✓ CommitObject succeeded after retry

✓ All operations completed successfully!
```

**Expected Output (standard git server):**
```
Testing Azure DevOps compatibility with: git@github.com:org/repo.git
Cloning repository...
✓ Clone successful
Testing branch: refs/heads/main
✓ CommitObject succeeded

✓ All operations completed successfully!
```

### Test Setup Helper

**File**: `setup_test_branch.sh`

A helper script to create a test branch in your Azure DevOps repository. The bug manifests most reliably when accessing non-default branches.

**Usage:**
```bash
cd tests/tools

export TEST_GIT_URL="git@ssh.dev.azure.com:v3/org/project/repo"
./setup_test_branch.sh
```

This script will:
1. Clone your Azure DevOps repository to a temporary directory
2. Create a new branch named `azure-devops-test-branch`
3. Add a test commit
4. Push the branch to the remote
5. Clean up the temporary directory

### Unit Test

**File**: `pkg/openstackconfigversion/azure_devops_test.go`

A unit test that validates the Azure DevOps compatibility fix.

**Usage:**
```bash
# Set environment variables
export TEST_GIT_URL="git@ssh.dev.azure.com:v3/org/project/repo"
export TEST_SSH_KEY_PATH="/path/to/ssh/private/key"

# Run the test
go test -v -run TestAzureDevOpsCompatibility ./pkg/openstackconfigversion/

# Or with make
make test
```

The test will be skipped if the environment variables are not set.

## Implementation

### Modified Files

**`pkg/openstackconfigversion/git_util.go`**
- Added error detection for `ErrObjectNotFound` at CommitObject
- Recursive retry with capability fix
- Support for both `master` and `main` default branches
- Logging for debugging capability state
- Uses `errors.Is()` for proper error type checking

### New Files

**`pkg/openstackconfigversion/azure_devops_test.go`**
- Unit test for Azure DevOps compatibility
- Skips gracefully if environment variables not set
- Comprehensive documentation on running the test

**`tests/tools/reproducer_azure_devops.go`**
- Standalone reproducer for validation
- Demonstrates error detection and retry flow

**`tests/tools/setup_test_branch.sh`**
- Helper script to create test branches
- Simplifies test environment setup

**`tests/tools/README.md`** (this file)
- Comprehensive documentation of the issue, fix, and tools

## Technical Notes

### Global State Considerations

`transport.UnsupportedCapabilities` is a global package variable in go-git. This means:

1. **Persistence**: Changes persist across function calls
2. **Process-wide**: Affects all git operations in the process
3. **Concurrency**: In the operator context, reconciliations are serialized, so this is safe
4. **Observable**: We log the state at the start of each `SyncGit()` call for debugging

### Performance Impact

- **First failure**: ~2x time (clone twice) - one-time cost
- **Subsequent operations**: Normal speed (fix persists in process)
- **Non-Azure DevOps**: Zero overhead (no errors, no retry)

### Error Checking

The implementation uses proper Go error checking:
```go
errors.Is(err, transport.ErrEmptyUploadPackRequest)  // For Clone failures
errors.Is(err, plumbing.ErrObjectNotFound)           // For CommitObject failures
```

This is more robust than string matching and follows Go best practices.

## Production Impact

### Before Fix
- ❌ Azure DevOps repositories fail with "Object not found"
- ❌ Configuration version sync fails
- ❌ No automatic recovery

### After Fix
- ✅ Azure DevOps repositories work automatically
- ✅ Self-healing on error detection
- ✅ Clear logging for debugging
- ✅ No configuration changes required from users
- ✅ Works with all git servers

## Migration Notes

Existing deployments using Azure DevOps will see:
- Automatic detection and fix on first sync
- Log messages indicating the workaround was applied
- No manual intervention required
- No configuration changes needed

## References

- **JIRA**: OSPRH-19930
- **go-git Issue**: https://github.com/go-git/go-git/pull/613
- **Implementation**: `pkg/openstackconfigversion/git_util.go`

## Additional Resources

For operators and developers:
- See the implementation in `pkg/openstackconfigversion/git_util.go`
- Review the unit test in `pkg/openstackconfigversion/azure_devops_test.go`
- Use the reproducer in this directory for validation
- Logs from the operator will show when the fix is applied
