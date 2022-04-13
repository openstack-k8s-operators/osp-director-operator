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

package v1beta1

import (
	"context"

	goClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
)

// OpenStackBackupOverridesReconcile - Should a controller pause reconciliation for a particular resource given potential backup operations?
func OpenStackBackupOverridesReconcile(client goClient.Client, namespace string, resourceReady bool) (bool, error) {
	var backupRequests *OpenStackBackupRequestList

	backupRequests, err := GetOpenStackBackupRequestsWithLabel(client, namespace, map[string]string{})

	if err != nil {
		return true, err
	}

	for _, backup := range backupRequests.Items {
		// If this backup is quiescing...
		// - If this CR has reached its "finished" state, end this reconcile
		// If this backup is saving or loading...
		// - End this reconcile
		if backup.Status.CurrentState == shared.BackupSaving ||
			backup.Status.CurrentState == shared.BackupLoading ||
			(backup.Status.CurrentState == shared.BackupQuiescing && resourceReady) {
			return true, nil
		}
	}

	return false, nil
}

// GetOpenStackBackupOperationInProgress - If there is a backup or restore in progress, returns a string indicating the operation
func GetOpenStackBackupOperationInProgress(client goClient.Client, namespace string) (shared.BackupState, error) {

	backups, err := GetOpenStackBackupRequestsWithLabel(client, namespace, map[string]string{})

	if err != nil {
		return "", err
	}

	for _, backup := range backups.Items {
		if backup.Status.CurrentState == shared.BackupCleaning || backup.Status.CurrentState == shared.BackupSaving || backup.Status.CurrentState == shared.BackupQuiescing ||
			backup.Status.CurrentState == shared.BackupReconciling || backup.Status.CurrentState == shared.BackupLoading {
			return backup.Status.CurrentState, nil
		}
	}

	return "", nil
}

// GetOpenStackBackupRequestsWithLabel - Return a list of all OpenStackBackupRequestss in the namespace that have (optional) labels
func GetOpenStackBackupRequestsWithLabel(client goClient.Client, namespace string, labelSelector map[string]string) (*OpenStackBackupRequestList, error) {
	osBackupRequestList := &OpenStackBackupRequestList{}

	listOpts := []goClient.ListOption{
		goClient.InNamespace(namespace),
	}

	if len(labelSelector) > 0 {
		labels := goClient.MatchingLabels(labelSelector)
		listOpts = append(listOpts, labels)
	}

	if err := client.List(context.Background(), osBackupRequestList, listOpts...); err != nil {
		return nil, err
	}

	return osBackupRequestList, nil
}

// GetOpenStackBackupsWithLabel - Return a list of all OpenStackBackups in the namespace that have (optional) labels
func GetOpenStackBackupsWithLabel(client goClient.Client, namespace string, labelSelector map[string]string) (*OpenStackBackupList, error) {
	osBackupList := &OpenStackBackupList{}

	listOpts := []goClient.ListOption{
		goClient.InNamespace(namespace),
	}

	if len(labelSelector) > 0 {
		labels := goClient.MatchingLabels(labelSelector)
		listOpts = append(listOpts, labels)
	}

	if err := client.List(context.Background(), osBackupList, listOpts...); err != nil {
		return nil, err
	}

	return osBackupList, nil
}
