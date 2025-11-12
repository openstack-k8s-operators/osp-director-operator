package v1beta2

// Because webhooks *MUST* exist within the api/<version> package, we need to place common
// functions here that might be used across different Kinds' webhooks

import (
	"context"
	"fmt"
	"regexp"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
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
			return fmt.Errorf("role \"%s\" is already in use by %s", roleName, resourceName)
		}
	}

	return nil
}

func getRoleNames(namespace string) (map[string]string, error) {
	found := map[string]string{}

	// Get OpenStackBaremetalSet role names
	baremetalSetsList := &ospdirectorv1beta1.OpenStackBaremetalSetList{}
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
			return fmt.Errorf("disk names must not be 'rootdisk' - %v", disk)
		}

		//
		// validate Disk Name consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character
		//
		var validDiskName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
		if !validDiskName.MatchString(disk.Name) {
			return fmt.Errorf("disk names must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character - %s", disk.Name)
		}

		//
		// validate that disk name is uniq within the VM role
		//
		if _, ok := disks[disk.Name]; !ok {
			disks[disk.Name] = disk
		} else {
			return fmt.Errorf("disk names must be uniq within a VM role - %v : %v", disks[disk.Name], disk)
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
		return fmt.Errorf("disk size must not change %s - new %v / current %v", diskName, diskSize, curDiskSize)
	}

	return nil
}

// storageAccessModeChanged -
func storageAccessModeChanged(diskName string, accessMode string, curAccessMode string) error {
	//
	// validate StorageAccessMode don't change
	//
	if accessMode != curAccessMode {
		return fmt.Errorf("StorageAccessMode must not change %s - new %v / current %v", diskName, accessMode, curAccessMode)
	}

	return nil
}

// storageClassChanged -
func storageClassChanged(diskName string, storageClass string, curStorageClass string) error {
	//
	// validate StorageClass don't change
	//
	if storageClass != curStorageClass {
		return fmt.Errorf("StorageClass must not change %s - new %v / current %v", diskName, storageClass, curStorageClass)
	}

	return nil
}

// storageVolumeModeChanged -
func storageVolumeModeChanged(diskName string, volumeMode string, curVolumeMode string) error {
	//
	// validate StorageVolumeMode don't change
	//
	if volumeMode != curVolumeMode {
		return fmt.Errorf("StorageVolumeMode must not change %s - new %v / current %v", diskName, volumeMode, curVolumeMode)
	}

	return nil
}

// baseImageVolumeNameChanged -
func baseImageVolumeNameChanged(diskName string, volumeName string, curVolumeName string) error {
	//
	// validate BaseImageVolumeName don't change
	//
	if volumeName != curVolumeName {
		return fmt.Errorf("BaseImageVolumeName must not change %s - new %v / current %v", diskName, volumeName, curVolumeName)
	}

	return nil
}
