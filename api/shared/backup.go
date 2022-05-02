/*
Copyright 2020 Red Hat

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

package shared

// BackupMode - whether the backup should be saved or restored
type BackupMode string

const (
	// BackupSave - save current operator config
	BackupSave BackupMode = "save"
	// BackupRestore - restore operator config contained in this backup
	BackupRestore BackupMode = "restore"
	// BackupCleanRestore - restore operator config contained in this backup after first deleting current config
	BackupCleanRestore BackupMode = "cleanRestore"
)

// BackupState - the state of this openstack network
type BackupState string

const (
	// BackupWaiting - the backup/restore is blocked by prerequisite objects
	BackupWaiting BackupState = "Waiting"
	// BackupQuiescing - the backup/restore is waiting for controllers to complete pending operations
	BackupQuiescing BackupState = "Quiescing"
	// BackupSaving - the backup is saving the current config of the operator
	BackupSaving BackupState = "Saving"
	// BackupSaved - the backup contains the saved config of the operator
	BackupSaved BackupState = "Saved"
	// BackupSaveError - the backup failed to save the operator config for some reason
	BackupSaveError BackupState = "Save Error"
	// BackupCleaning - the backup is waiting to restore until cleaning is completed
	BackupCleaning BackupState = "Cleaning"
	// BackupLoading - the backup is being loaded into the operator to prepare for restoring via reconciliation
	BackupLoading BackupState = "Loading"
	// BackupReconciling - the backup is being restored via reconciliation as the config of the operator
	BackupReconciling BackupState = "Reconciling"
	// BackupRestored - the backup was restored as the config of the operator
	BackupRestored BackupState = "Restored"
	// BackupRestoreError - the backup restore failed for some reason
	BackupRestoreError BackupState = "Restore Error"
)
