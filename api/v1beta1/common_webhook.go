package v1beta1

// Because webhooks *MUST* exist within the api/<version> package, we need to place common
// functions here that might be used across different Kinds' webhooks

import (
	"context"
	"fmt"
	"strings"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	goClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Client needed for API calls (manager's client, set by first SetupWebhookWithManager() call
// to any particular webhook)
var webhookClient goClient.Client

// checkRoleNameExists - This function is needed by webhooks for both OpenStackBaremetalSets and VMSets
// to help ensure that role names are unique across a given namespace
func checkRoleNameExists(typeMeta metav1.TypeMeta, objectMeta metav1.ObjectMeta, role string) error {
	existing, err := getRoleNames(objectMeta.Namespace)

	if err != nil {
		return err
	}

	for resourceName, roleName := range existing {
		// If the role name is already in use by another resource, this resource is invalid
		if resourceName != fmt.Sprintf("%s/%s", typeMeta.Kind, objectMeta.Name) && roleName == role {
			return fmt.Errorf("Role \"%s\" is already in use by %s", roleName, resourceName)
		}
	}

	return nil
}

func getRoleNames(namespace string) (map[string]string, error) {
	found := map[string]string{}

	// Get OpenStackBaremetalSet role names
	baremetalSetsList := &OpenStackBaremetalSetList{}
	listOpts := []goClient.ListOption{
		goClient.InNamespace(namespace),
	}

	err := webhookClient.List(context.TODO(), baremetalSetsList, listOpts...)
	if err != nil {
		return nil, err
	}

	for _, bmSet := range baremetalSetsList.Items {
		found[fmt.Sprintf("%s/%s", bmSet.Kind, bmSet.Name)] = bmSet.Spec.RoleName
	}

	// Get VMSet role names
	vmSetsList := &OpenStackVMSetList{}

	err = webhookClient.List(context.TODO(), vmSetsList, listOpts...)
	if err != nil {
		return nil, err
	}

	for _, vmSet := range vmSetsList.Items {
		found[fmt.Sprintf("%s/%s", vmSet.Kind, vmSet.Name)] = vmSet.Spec.RoleName
	}

	return found, nil
}

func checkDomainName(domainName string) error {

	// TODO: implement the same validation as freeipa validate_domain_name()
	//       in https://github.com/freeipa/freeipa/blob/master/ipalib/util.py
	if domainName != "" && len(strings.Split(domainName, ".")) < 2 {
		return fmt.Errorf("domainName must include a top-level domain and at least one subdomain")
	}
	return nil
}

func checkBackupOperationBlocksAction(namespace string, action APIAction) error {
	op, err := GetOpenStackBackupOperationInProgress(webhookClient, namespace)

	if err != nil {
		return err
	}

	if action == APIActionCreate && (op == BackupCleaning || op == BackupSaving || op == BackupReconciling) {
		// Don't allow creation of certain OSP-D-operator-specific CRDs during backup save, (restore) clean or (restore) reconcile
		err = fmt.Errorf("OSP-D operator API is disabled for creating resources while certain backup operations are in progress")
	} else if action == APIActionDelete && (op == BackupLoading || op == BackupSaving || op == BackupReconciling) {
		// Don't allow deletion of certain OSP-D-operator-specific CRDs during backup save, (restore) load or (restore) reconcile
		err = fmt.Errorf("OSP-D operator API is disabled for deleting resources while certain backup operations are in progress")
	}

	return err
}

// getAllMACReservations - get all MAC reservations in format MAC -> hostname
func getAllMACReservations(macNodeStatus map[string]OpenStackMACNodeReservation) map[string]string {
	ret := make(map[string]string)
	for hostname, status := range macNodeStatus {
		for _, mac := range status.Reservations {
			ret[mac] = hostname
		}
	}
	return ret
}

// IsUniqMAC - check if the MAC address is uniq or already present in the reservations
func IsUniqMAC(macNodeStatus map[string]OpenStackMACNodeReservation, mac string) bool {
	reservations := getAllMACReservations(macNodeStatus)
	if _, ok := reservations[mac]; !ok {
		return true
	}
	return false
}

// validateNetworks - validate that for all configured subnets an osnet exists
func validateNetworks(namespace string, networks []string) error {
	for _, subnetName := range networks {
		//
		// Get OSnet with SubNetNameLabelSelector: subnetName
		//
		labelSelector := map[string]string{
			SubNetNameLabelSelector: subnetName,
		}
		osnet, err := GetOpenStackNetWithLabel(webhookClient, namespace, labelSelector)
		if err != nil && k8s_errors.IsNotFound(err) {
			return fmt.Errorf(fmt.Sprintf("%s %s not found, validate the object network list!", osnet.GetObjectKind().GroupVersionKind().Kind, subnetName))
		} else if err != nil {
			return fmt.Errorf(fmt.Sprintf("Failed to get %s %s", osnet.Kind, subnetName))
		}
	}

	return nil
}

// validateRootDisk - validate configured rootdisk
func validateRootDisk(newDisk OpenStackVMSetDisk, currentDisk OpenStackVMSetDisk) error {
	//
	// validate DiskSize don't change
	//
	if err := diskSizeChanged(newDisk.Name, newDisk.DiskSize, currentDisk.DiskSize); err != nil {
		return err
	}

	//
	// validate StorageAccessMode don't change
	//
	if err := storageAccessModeChanged(newDisk.Name, newDisk.StorageAccessMode, currentDisk.StorageAccessMode); err != nil {
		return err
	}

	//
	// validate StorageClass don't change
	//
	if err := storageClassChanged(newDisk.Name, newDisk.StorageClass, currentDisk.StorageClass); err != nil {
		return err
	}

	//
	// validate StorageVolumeMode don't change
	//
	if err := storageVolumeModeChanged(newDisk.Name, newDisk.StorageVolumeMode, currentDisk.StorageVolumeMode); err != nil {
		return err
	}

	//
	// validate BaseImageVolumeName don't change
	//
	if err := baseImageVolumeNameChanged(newDisk.Name, newDisk.BaseImageVolumeName, currentDisk.BaseImageVolumeName); err != nil {
		return err
	}

	return nil
}

// validateAdditionalDisks - validate configured disks
func validateAdditionalDisks(newDisks []OpenStackVMSetDisk, currentDisks []OpenStackVMSetDisk) error {
	disks := map[string]OpenStackVMSetDisk{}
	curDisks := map[string]OpenStackVMSetDisk{}

	if len(currentDisks) > 0 {
		for _, curDisk := range currentDisks {
			curDisks[curDisk.Name] = curDisk
		}
	}

	for _, disk := range newDisks {
		//
		// validate Disk Name is not 'rootdisk' as this is reserved for the boot disk
		//
		if disk.Name == "rootdisk" {
			return fmt.Errorf(fmt.Sprintf("disk names must not be 'rootdisk' - %v", disk))
		}

		//
		// validate that disk name is uniq within the VM role
		//
		if _, ok := disks[disk.Name]; !ok {
			disks[disk.Name] = disk
		} else {
			return fmt.Errorf(fmt.Sprintf("disk names must be uniq within a VM role - %v : %v", disks[disk.Name], disk))
		}

		//
		// validations on CR update
		//
		if curDisk, ok := curDisks[disk.Name]; ok {
			//
			// validate DiskSize don't change
			//
			if err := diskSizeChanged(disk.Name, disk.DiskSize, curDisk.DiskSize); err != nil {
				return err
			}

			//
			// validate StorageAccessMode don't change
			//
			if err := storageAccessModeChanged(disk.Name, disk.StorageAccessMode, curDisk.StorageAccessMode); err != nil {
				return err
			}

			//
			// validate StorageClass don't change
			//
			if err := storageClassChanged(disk.Name, disk.StorageClass, curDisk.StorageClass); err != nil {
				return err
			}

			//
			// validate StorageVolumeMode don't change
			//
			if err := storageVolumeModeChanged(disk.Name, disk.StorageVolumeMode, curDisk.StorageVolumeMode); err != nil {
				return err
			}
		}

	}

	return nil
}

// diskSizeChanged -
func diskSizeChanged(diskName string, diskSize uint32, curDiskSize uint32) error {
	//
	// validate DiskSize don't change
	//
	if diskSize != curDiskSize {
		return fmt.Errorf(fmt.Sprintf("disk size must not change %s - new %v / current %v", diskName, diskSize, curDiskSize))
	}

	return nil
}

// storageAccessModeChanged -
func storageAccessModeChanged(diskName string, accessMode string, curAccessMode string) error {
	//
	// validate StorageAccessMode don't change
	//
	if accessMode != curAccessMode {
		return fmt.Errorf(fmt.Sprintf("StorageAccessMode must not change %s - new %v / current %v", diskName, accessMode, curAccessMode))
	}

	return nil
}

// storageClassChanged -
func storageClassChanged(diskName string, storageClass string, curStorageClass string) error {
	//
	// validate StorageClass don't change
	//
	if storageClass != curStorageClass {
		return fmt.Errorf(fmt.Sprintf("StorageClass must not change %s - new %v / current %v", diskName, storageClass, curStorageClass))
	}

	return nil
}

// storageVolumeModeChanged -
func storageVolumeModeChanged(diskName string, volumeMode string, curVolumeMode string) error {
	//
	// validate StorageVolumeMode don't change
	//
	if volumeMode != curVolumeMode {
		return fmt.Errorf(fmt.Sprintf("StorageVolumeMode must not change %s - new %v / current %v", diskName, volumeMode, curVolumeMode))
	}

	return nil
}

// baseImageVolumeNameChanged -
func baseImageVolumeNameChanged(diskName string, volumeName string, curVolumeName string) error {
	//
	// validate BaseImageVolumeName don't change
	//
	if volumeName != curVolumeName {
		return fmt.Errorf(fmt.Sprintf("BaseImageVolumeName must not change %s - new %v / current %v", diskName, volumeName, curVolumeName))
	}

	return nil
}
