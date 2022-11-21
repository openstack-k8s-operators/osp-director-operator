package v1beta1

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetBmhHosts -
func GetBmhHosts(
	ctx context.Context,
	c client.Client,
	namespace string,
	labelSelector map[string]string,
) (*metal3v1alpha1.BareMetalHostList, error) {

	bmhHostsList := &metal3v1alpha1.BareMetalHostList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	if len(labelSelector) > 0 {
		labels := client.MatchingLabels(labelSelector)
		listOpts = append(listOpts, labels)
	}

	err := c.List(ctx, bmhHostsList, listOpts...)
	if err != nil {
		return bmhHostsList, err
	}

	return bmhHostsList, nil
}

// GetDeletionAnnotatedBmhHosts -
func GetDeletionAnnotatedBmhHosts(
	ctx context.Context,
	c client.Client,
	namespace string,
	labelSelector map[string]string,
) ([]string, error) {
	baremetalHostList, err := GetBmhHosts(ctx, c, namespace, labelSelector)
	if err != nil {
		return []string{}, err
	}

	return getDeletionAnnotatedBmhHosts(baremetalHostList), nil
}

func getDeletionAnnotatedBmhHosts(
	baremetalHostList *metal3v1alpha1.BareMetalHostList,
) []string {
	annotatedBMHs := []string{}

	// Find deletion annotated BaremetalHosts belonging to this OpenStackBaremetalSet
	for _, baremetalHost := range baremetalHostList.Items {

		// Get list of OSP hostnames from HostRemovalAnnotation annotated BMHs
		if val, ok := baremetalHost.Annotations[shared.HostRemovalAnnotation]; ok && (strings.ToLower(val) == "yes" || strings.ToLower(val) == "true") {
			annotatedBMHs = append(annotatedBMHs, baremetalHost.GetName())
		}
	}

	return annotatedBMHs
}

// VerifyBaremetalSetScaleUp -
func VerifyBaremetalSetScaleUp(log logr.Logger, instance *OpenStackBaremetalSet, allBmhs *metal3v1alpha1.BareMetalHostList, existingBmhs *metal3v1alpha1.BareMetalHostList) ([]string, error) {
	// How many new BaremetalHost allocations do we need (if any)?
	newBmhsNeededCount := instance.Spec.Count - len(existingBmhs.Items)
	availableBaremetalHosts := []string{}

	if newBmhsNeededCount > 0 {
		// We have new replicas requested, so search for baremetalhosts that don't have consumerRef or Online set

		for _, baremetalHost := range allBmhs.Items {
			if baremetalHost.Spec.Online || baremetalHost.Spec.ConsumerRef != nil {
				continue
			}

			if !verifyBaremetalSetHardwareMatch(log, instance, &baremetalHost) {
				log.Info(fmt.Sprintf("BaremetalHost %s does not match hardware requirements for %s %s",
					baremetalHost.ObjectMeta.Name,
					instance.Kind,
					instance.Name))

				continue
			}

			log.Info(fmt.Sprintf("Available BaremetalHost: %s", baremetalHost.ObjectMeta.Name))

			availableBaremetalHosts = append(availableBaremetalHosts, baremetalHost.ObjectMeta.Name)
		}

		// If we can't satisfy the new requested replica count, explicitly state so
		if newBmhsNeededCount > len(availableBaremetalHosts) {
			return nil, fmt.Errorf("Unable to find %d requested BaremetalHost count (%d in use, %d available)",
				instance.Spec.Count,
				len(existingBmhs.Items),
				len(availableBaremetalHosts))
		}

	}

	return availableBaremetalHosts, nil
}

// VerifyBaremetalSetScaleDown -
func VerifyBaremetalSetScaleDown(log logr.Logger, instance *OpenStackBaremetalSet, existingBmhs *metal3v1alpha1.BareMetalHostList, removalAnnotatedBmhCount int) error {
	// How many new BaremetalHost de-allocations do we need (if any)?
	bmhsToRemoveCount := len(existingBmhs.Items) - instance.Spec.Count

	if bmhsToRemoveCount > removalAnnotatedBmhCount {
		return fmt.Errorf("Unable to find sufficient amount of BaremetalHost replicas annotated for scale-down (%d found, %d requested)", removalAnnotatedBmhCount, bmhsToRemoveCount)
	}

	return nil
}

func verifyBaremetalSetHardwareMatch(
	log logr.Logger,
	instance *OpenStackBaremetalSet,
	bmh *metal3v1alpha1.BareMetalHost,
) bool {
	// If no requested hardware requirements, we're all set
	if instance.Spec.HardwareReqs == (HardwareReqs{}) {
		return true
	}

	// Can't make comparisons if the BMH lacks hardware details
	if bmh.Status.HardwareDetails == nil {
		log.Info(fmt.Sprintf("WARNING: BaremetalHost %s lacks hardware details in status; cannot verify against %s %s hardware requests!",
			bmh.Name,
			instance.Kind,
			instance.Name,
		))

		return false
	}

	cpuReqs := instance.Spec.HardwareReqs.CPUReqs

	// CPU architecture is always exact-match only
	if cpuReqs.Arch != "" && bmh.Status.HardwareDetails.CPU.Arch != cpuReqs.Arch {
		log.Info(fmt.Sprintf("BaremetalHost %s CPU arch %s does not match %s %s request for '%s'",
			bmh.Name,
			bmh.Status.HardwareDetails.CPU.Arch,
			instance.Kind,
			instance.Name,
			cpuReqs.Arch,
		))

		return false
	}

	// CPU count can be exact-match or (default) greater
	if cpuReqs.CountReq.Count != 0 && bmh.Status.HardwareDetails.CPU.Count != cpuReqs.CountReq.Count {
		if cpuReqs.CountReq.ExactMatch || cpuReqs.CountReq.Count > bmh.Status.HardwareDetails.CPU.Count {
			log.Info(fmt.Sprintf("BaremetalHost %s CPU count %d does not match %s %s request for '%d'",
				bmh.Name,
				bmh.Status.HardwareDetails.CPU.Count,
				instance.Kind,
				instance.Name,
				cpuReqs.CountReq.Count,
			))

			return false
		}
	}

	// CPU clock speed can be exact-match or (default) greater
	if cpuReqs.MhzReq.Mhz != 0 {
		clockSpeed := int(bmh.Status.HardwareDetails.CPU.ClockMegahertz)
		if cpuReqs.MhzReq.Mhz != clockSpeed && (cpuReqs.MhzReq.ExactMatch || cpuReqs.MhzReq.Mhz > clockSpeed) {
			log.Info(fmt.Sprintf("BaremetalHost %s CPU mhz %d does not match %s %s request for '%d'",
				bmh.Name,
				clockSpeed,
				instance.Kind,
				instance.Name,
				cpuReqs.MhzReq.Mhz,
			))

			return false
		}
	}

	memReqs := instance.Spec.HardwareReqs.MemReqs

	// Memory GBs can be exact-match or (default) greater
	if memReqs.GbReq.Gb != 0 {

		memGbBms := float64(memReqs.GbReq.Gb)
		memGbBmh := float64(bmh.Status.HardwareDetails.RAMMebibytes) / float64(1024)

		if memGbBmh != memGbBms && (memReqs.GbReq.ExactMatch || memGbBms > memGbBmh) {
			log.Info(fmt.Sprintf("BaremetalHost %s memory size %v does not match %s %s request for '%v'",
				bmh.Name,
				memGbBmh,
				instance.Kind,
				instance.Name,
				memGbBms,
			))

			return false
		}
	}

	diskReqs := instance.Spec.HardwareReqs.DiskReqs

	var foundDisk *metal3v1alpha1.Storage

	if diskReqs.GbReq.Gb != 0 {
		diskGbBms := float64(diskReqs.GbReq.Gb)
		// TODO: Make sure there's at least one disk of this size?
		for _, disk := range bmh.Status.HardwareDetails.Storage {
			diskGbBmh := float64(disk.SizeBytes) / float64(1073741824)

			if diskGbBmh == diskGbBms || (!diskReqs.GbReq.ExactMatch && diskGbBmh > diskGbBms) {
				foundDisk = &disk
				break
			}
		}

		if foundDisk == nil {
			log.Info(fmt.Sprintf("BaremetalHost %s does not contain a disk of size %v that matches %s %s request",
				bmh.Name,
				diskGbBms,
				instance.Kind,
				instance.Name,
			))

			return false
		}
	}

	// We only care about the SSD flag if the user requested an exact match for it or if SSD is true
	if diskReqs.SSDReq.ExactMatch || diskReqs.SSDReq.SSD {
		found := false

		// If we matched on a disk for a GbReqs above, we need to match on the same disk
		if foundDisk != nil {
			if foundDisk.Rotational != diskReqs.SSDReq.SSD {
				found = true
			}
		} else {
			// TODO: Just need to match on any disk?
			for _, disk := range bmh.Status.HardwareDetails.Storage {
				if disk.Rotational != diskReqs.SSDReq.SSD {
					found = true
				}
			}
		}

		if !found {
			log.Info(fmt.Sprintf("BaremetalHost %s does not contain a disk with 'rotational' equal to %v that matches %s %s request",
				bmh.Name,
				diskReqs.SSDReq.SSD,
				instance.Kind,
				instance.Name,
			))

			return false
		}
	}

	log.Info(fmt.Sprintf("BaremetalHost %s satisfies %s %s hardware requirements",
		bmh.Name,
		instance.Kind,
		instance.Name,
	))

	return true
}
