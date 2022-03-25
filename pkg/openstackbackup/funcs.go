package openstackbackup

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackconfiggenerator"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetCRLists - Get lists of all OSP-D operator resources in the namespace that we care to save/restore
func GetCRLists(
	ctx context.Context,
	r common.ReconcilerCommon,
	namespace string,
) (ospdirectorv1beta1.CrsForBackup, error) {
	crLists := ospdirectorv1beta1.CrsForBackup{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	// OpenStackBaremetalSets

	osBms := ospdirectorv1beta1.OpenStackBaremetalSetList{}

	if err := r.GetClient().List(ctx, &osBms, listOpts...); err != nil {
		return crLists, err
	}

	sort.Slice(osBms.Items, func(i, j int) bool {
		return osBms.Items[i].Name < osBms.Items[j].Name
	})
	crLists.OpenStackBaremetalSets = osBms

	// OpenStackClients

	osClients := ospdirectorv1beta1.OpenStackClientList{}

	if err := r.GetClient().List(ctx, &osClients, listOpts...); err != nil {
		return crLists, err
	}

	sort.Slice(osClients.Items, func(i, j int) bool {
		return osClients.Items[i].Name < osClients.Items[j].Name
	})
	crLists.OpenStackClients = osClients

	// OpenStackControlPlanes

	osCtlPlanes := ospdirectorv1beta1.OpenStackControlPlaneList{}

	if err := r.GetClient().List(ctx, &osCtlPlanes, listOpts...); err != nil {
		return crLists, err
	}

	sort.Slice(osCtlPlanes.Items, func(i, j int) bool {
		return osCtlPlanes.Items[i].Name < osCtlPlanes.Items[j].Name
	})
	crLists.OpenStackControlPlanes = osCtlPlanes

	// OpenStackMACAddresses

	osMacAddresses := ospdirectorv1beta1.OpenStackMACAddressList{}

	if err := r.GetClient().List(ctx, &osMacAddresses, listOpts...); err != nil {
		return crLists, err
	}

	sort.Slice(osMacAddresses.Items, func(i, j int) bool {
		return osMacAddresses.Items[i].Name < osMacAddresses.Items[j].Name
	})
	crLists.OpenStackMACAddresses = osMacAddresses

	// OpenStackNetworks

	osNets := ospdirectorv1beta1.OpenStackNetList{}

	if err := r.GetClient().List(ctx, &osNets, listOpts...); err != nil {
		return crLists, err
	}

	sort.Slice(osNets.Items, func(i, j int) bool {
		return osNets.Items[i].Name < osNets.Items[j].Name
	})
	crLists.OpenStackNets = osNets

	// OpenStackNetAttachments

	osNetAttachments := ospdirectorv1beta1.OpenStackNetAttachmentList{}

	if err := r.GetClient().List(ctx, &osNetAttachments, listOpts...); err != nil {
		return crLists, err
	}

	sort.Slice(osNetAttachments.Items, func(i, j int) bool {
		return osNetAttachments.Items[i].Name < osNetAttachments.Items[j].Name
	})
	crLists.OpenStackNetAttachments = osNetAttachments

	// OpenStackNetConfigs

	osNetConfigs := ospdirectorv1beta1.OpenStackNetConfigList{}

	if err := r.GetClient().List(ctx, &osNetConfigs, listOpts...); err != nil {
		return crLists, err
	}

	sort.Slice(osNetConfigs.Items, func(i, j int) bool {
		return osNetConfigs.Items[i].Name < osNetConfigs.Items[j].Name
	})
	crLists.OpenStackNetConfigs = osNetConfigs

	// OpenStackProvisionServers

	osProvServers := ospdirectorv1beta1.OpenStackProvisionServerList{}

	if err := r.GetClient().List(ctx, &osProvServers, listOpts...); err != nil {
		return crLists, err
	}

	sort.Slice(osProvServers.Items, func(i, j int) bool {
		return osProvServers.Items[i].Name < osProvServers.Items[j].Name
	})
	crLists.OpenStackProvisionServers = osProvServers

	// OpenStackVMSets

	osVms := ospdirectorv1beta1.OpenStackVMSetList{}

	if err := r.GetClient().List(ctx, &osVms, listOpts...); err != nil {
		return crLists, err
	}

	sort.Slice(osVms.Items, func(i, j int) bool {
		return osVms.Items[i].Name < osVms.Items[j].Name
	})
	crLists.OpenStackVMSets = osVms

	return crLists, nil
}

// GetConfigMapList - Get list of all OSP-D operator config maps in the namespace that we care to save/restore
func GetConfigMapList(
	ctx context.Context,
	r common.ReconcilerCommon,
	request *ospdirectorv1beta1.OpenStackBackupRequest,
	desiredCrs *ospdirectorv1beta1.CrsForBackup,
) (corev1.ConfigMapList, error) {
	configMapList := &corev1.ConfigMapList{}

	labels := client.HasLabels{
		common.OwnerControllerNameLabelSelector,
	}

	listOpts := []client.ListOption{
		client.InNamespace(request.Namespace),
		labels,
	}

	err := r.GetClient().List(ctx, configMapList, listOpts...)

	if err != nil {
		return *configMapList, err
	}

	// Also need to get config maps used by OpenStackConfigGenerator, if any
	osCGConfigMapList := &corev1.ConfigMapList{}

	labels = client.HasLabels{
		openstackconfiggenerator.ConfigGeneratorInputLabel,
	}

	listOpts = []client.ListOption{
		client.InNamespace(request.Namespace),
		labels,
	}

	err = r.GetClient().List(ctx, osCGConfigMapList, listOpts...)

	if err != nil {
		return *configMapList, err
	}

	configMapList.Items = append(configMapList.Items, osCGConfigMapList.Items...)

	// Also get certain config maps in other particular namespaces?
	// TODO: which ones?

	// Also get certain config maps used by our CRs
	// TODO: which ones?

	// Also get additional config maps potentially enumerated in the request CR
	for _, item := range request.Spec.AdditionalConfigMaps {
		if err := addConfigMapToList(ctx, r, request.Namespace, item, configMapList); err != nil {
			return *configMapList, err
		}
	}

	sort.Slice(configMapList.Items, func(i, j int) bool {
		return configMapList.Items[i].Name < configMapList.Items[j].Name
	})

	return *configMapList, nil
}

func addConfigMapToList(
	ctx context.Context,
	r common.ReconcilerCommon,
	namespace string,
	name string,
	configMapList *corev1.ConfigMapList,
) error {
	found := false

	for _, cm := range configMapList.Items {
		if name == cm.Name {
			found = true
			break
		}
	}

	if !found {
		cm := &corev1.ConfigMap{}

		if err := r.GetClient().Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, cm); err != nil {
			return err
		}

		configMapList.Items = append(configMapList.Items, *cm)
	}

	return nil
}

// GetSecretList - Get list of all OSP-D operator secrets in the namespace that we care to save/restore
func GetSecretList(
	ctx context.Context,
	r common.ReconcilerCommon,
	request *ospdirectorv1beta1.OpenStackBackupRequest,
	desiredCrs *ospdirectorv1beta1.CrsForBackup,
) (corev1.SecretList, error) {
	secretList := &corev1.SecretList{}

	labels := client.HasLabels{
		common.OwnerControllerNameLabelSelector,
	}

	listOpts := []client.ListOption{
		client.InNamespace(request.Namespace),
		labels,
	}

	err := r.GetClient().List(ctx, secretList, listOpts...)

	if err != nil {
		return *secretList, err
	}

	// Also get certain secrets used by our CRs
	for _, item := range desiredCrs.OpenStackBaremetalSets.Items {
		if item.Spec.DeploymentSSHSecret != "" {
			if err := addSecretToList(ctx, r, request.Namespace, item.Spec.DeploymentSSHSecret, secretList); err != nil {
				return *secretList, err
			}
		}
		if item.Spec.PasswordSecret != "" {
			if err := addSecretToList(ctx, r, request.Namespace, item.Spec.PasswordSecret, secretList); err != nil {
				return *secretList, err
			}
		}
	}

	for _, item := range desiredCrs.OpenStackVMSets.Items {
		if item.Spec.DeploymentSSHSecret != "" {
			if err := addSecretToList(ctx, r, request.Namespace, item.Spec.DeploymentSSHSecret, secretList); err != nil {
				return *secretList, err
			}
		}
		if item.Spec.PasswordSecret != "" {
			if err := addSecretToList(ctx, r, request.Namespace, item.Spec.PasswordSecret, secretList); err != nil {
				return *secretList, err
			}
		}
	}

	for _, item := range desiredCrs.OpenStackControlPlanes.Items {
		if item.Spec.PasswordSecret != "" {
			if err := addSecretToList(ctx, r, request.Namespace, item.Spec.PasswordSecret, secretList); err != nil {
				return *secretList, err
			}
		}
	}

	for _, item := range desiredCrs.OpenStackClients.Items {
		if item.Spec.DeploymentSSHSecret != "" {
			if err := addSecretToList(ctx, r, request.Namespace, item.Spec.DeploymentSSHSecret, secretList); err != nil {
				return *secretList, err
			}
		}
	}

	// Also get additional secrets potentially enumerated in the request CR
	for _, item := range request.Spec.AdditionalSecrets {
		if err := addSecretToList(ctx, r, request.Namespace, item, secretList); err != nil {
			return *secretList, err
		}
	}

	sort.Slice(secretList.Items, func(i, j int) bool {
		return secretList.Items[i].Name < secretList.Items[j].Name
	})

	return *secretList, nil
}

func addSecretToList(
	ctx context.Context,
	r common.ReconcilerCommon,
	namespace string,
	name string,
	secretList *corev1.SecretList,
) error {
	found := false

	for _, secret := range secretList.Items {
		if name == secret.Name {
			found = true
			break
		}
	}

	if !found {
		secret := &corev1.Secret{}

		if err := r.GetClient().Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, secret); err != nil {
			return err
		}

		secretList.Items = append(secretList.Items, *secret)
	}

	return nil
}

// GetAreControllersQuiesced - returns true if all desired CRs for backup are in their respective "finished" state, or false with a list of "bad" CRs if otherwise
func GetAreControllersQuiesced(instance *ospdirectorv1beta1.OpenStackBackupRequest, crLists ospdirectorv1beta1.CrsForBackup) (bool, []client.Object) {
	badCrs := []client.Object{}

	// Check provisioning status of all OpenStackBaremetalSets
	for _, cr := range crLists.OpenStackBaremetalSets.Items {
		if !cr.IsReady() {
			copy := cr.DeepCopy()
			badCrs = append(badCrs, copy)
		}
	}

	// Check provisioning status of all OpenStackClients
	for _, cr := range crLists.OpenStackClients.Items {
		if !cr.IsReady() {
			copy := cr.DeepCopy()
			badCrs = append(badCrs, copy)
		}
	}

	// Check provisioning status of all OpenStackControlPlanes
	for _, cr := range crLists.OpenStackControlPlanes.Items {
		if !cr.IsReady() {
			copy := cr.DeepCopy()
			badCrs = append(badCrs, copy)
		}
	}

	// Check the creation status of all OpenStackMACAddresses
	for _, cr := range crLists.OpenStackMACAddresses.Items {
		if !cr.IsReady() {
			copy := cr.DeepCopy()
			badCrs = append(badCrs, copy)
		}
	}

	// Check the creation status of all OpenStackNets
	for _, cr := range crLists.OpenStackNets.Items {
		if !cr.IsReady() {
			copy := cr.DeepCopy()
			badCrs = append(badCrs, copy)
		}
	}

	// Check the creation status of all OpenStackNetAttachments
	for _, cr := range crLists.OpenStackNetAttachments.Items {
		if !cr.IsReady() {
			copy := cr.DeepCopy()
			badCrs = append(badCrs, copy)
		}
	}

	// Check the creation status of all OpenStackNetConfigs
	for _, cr := range crLists.OpenStackNetConfigs.Items {
		if !cr.IsReady() {
			copy := cr.DeepCopy()
			badCrs = append(badCrs, copy)
		}
	}

	// Check the provisioning status of all OpenStackProvisionServers
	for _, cr := range crLists.OpenStackProvisionServers.Items {
		if !cr.IsReady() {
			copy := cr.DeepCopy()
			badCrs = append(badCrs, copy)
		}
	}

	// Check the provisioning status of all OpenStackVMSets
	for _, cr := range crLists.OpenStackVMSets.Items {
		if !cr.IsReady() {
			copy := cr.DeepCopy()
			badCrs = append(badCrs, copy)
		}
	}

	return (len(badCrs) == 0), badCrs
}

// GetAreResourcesRestored - returns true if all desired CRs for backup restore are in their respective "finished" state, or false with a list of "bad" CRs if otherwise
func GetAreResourcesRestored(backup *ospdirectorv1beta1.OpenStackBackup, crLists ospdirectorv1beta1.CrsForBackup) (bool, []client.Object) {
	badCrs := []client.Object{}

	// OpenStackNets
	for _, desired := range backup.Spec.Crs.OpenStackNets.Items {
		found := &ospdirectorv1beta1.OpenStackNet{}

		for _, actual := range crLists.OpenStackNets.Items {
			if actual.Name == desired.Name {
				found = actual.DeepCopy()
				break
			}
		}

		if found == nil || !found.IsReady() {
			badCrs = append(badCrs, found)
		}
	}

	// OpenStackNetAttachments
	for _, desired := range backup.Spec.Crs.OpenStackNetAttachments.Items {
		found := &ospdirectorv1beta1.OpenStackNetAttachment{}

		for _, actual := range crLists.OpenStackNetAttachments.Items {
			if actual.Name == desired.Name {
				found = actual.DeepCopy()
				break
			}
		}

		if found == nil || !found.IsReady() {
			badCrs = append(badCrs, found)
		}
	}

	// OpenStackNetConfigs
	for _, desired := range backup.Spec.Crs.OpenStackNetConfigs.Items {
		found := &ospdirectorv1beta1.OpenStackNetConfig{}

		for _, actual := range crLists.OpenStackNetConfigs.Items {
			if actual.Name == desired.Name {
				found = actual.DeepCopy()
				break
			}
		}

		if found == nil || !found.IsReady() {
			badCrs = append(badCrs, found)
		}
	}

	// OpenStackMACAddresses
	for _, desired := range backup.Spec.Crs.OpenStackMACAddresses.Items {
		found := &ospdirectorv1beta1.OpenStackMACAddress{}

		for _, actual := range crLists.OpenStackMACAddresses.Items {
			if actual.Name == desired.Name {
				found = actual.DeepCopy()
				break
			}
		}

		if found == nil || !found.IsReady() {
			badCrs = append(badCrs, found)
		}
	}

	// OpenStackProvisionServers
	for _, desired := range backup.Spec.Crs.OpenStackProvisionServers.Items {
		found := &ospdirectorv1beta1.OpenStackProvisionServer{}

		for _, actual := range crLists.OpenStackProvisionServers.Items {
			if actual.Name == desired.Name {
				found = actual.DeepCopy()
				break
			}
		}

		if found == nil || !found.IsReady() {
			badCrs = append(badCrs, found)
		}
	}

	// OpenStackBaremetalSets
	for _, desired := range backup.Spec.Crs.OpenStackBaremetalSets.Items {
		found := &ospdirectorv1beta1.OpenStackBaremetalSet{}

		for _, actual := range crLists.OpenStackBaremetalSets.Items {
			if actual.Name == desired.Name {
				found = actual.DeepCopy()
				break
			}
		}

		if found == nil || !found.IsReady() {
			badCrs = append(badCrs, found)
		}
	}

	// OpenStackClients
	for _, desired := range backup.Spec.Crs.OpenStackClients.Items {
		found := &ospdirectorv1beta1.OpenStackClient{}

		for _, actual := range crLists.OpenStackClients.Items {
			if actual.Name == desired.Name {
				found = actual.DeepCopy()
				break
			}
		}

		if found == nil || !found.IsReady() {
			badCrs = append(badCrs, found)
		}
	}

	// OpenStackVMSets
	for _, desired := range backup.Spec.Crs.OpenStackVMSets.Items {
		found := &ospdirectorv1beta1.OpenStackVMSet{}

		for _, actual := range crLists.OpenStackVMSets.Items {
			if actual.Name == desired.Name {
				found = actual.DeepCopy()
				break
			}
		}

		if found == nil || !found.IsReady() {
			badCrs = append(badCrs, found)
		}
	}

	// OpenStackControlPlanes
	for _, desired := range backup.Spec.Crs.OpenStackControlPlanes.Items {
		found := &ospdirectorv1beta1.OpenStackControlPlane{}

		for _, actual := range crLists.OpenStackControlPlanes.Items {
			if actual.Name == desired.Name {
				found = actual.DeepCopy()
				break
			}
		}

		if found == nil || !found.IsReady() {
			badCrs = append(badCrs, found)
		}
	}

	return (len(badCrs) == 0), badCrs
}

// CleanNamespace - deleted CRs, ConfigMaps and Secrets in this namespace
func CleanNamespace(
	ctx context.Context,
	r common.ReconcilerCommon,
	namespace string,
	crLists ospdirectorv1beta1.CrsForBackup,
	cmList corev1.ConfigMapList,
	secretList corev1.SecretList,
) (bool, error) {
	foundRemaining := false

	// Delete OpenStackConfigGenerators as these should not be needed at this point
	if err := r.GetClient().DeleteAllOf(ctx, &ospdirectorv1beta1.OpenStackConfigGenerator{}, client.InNamespace(namespace)); err != nil {
		return false, err
	}

	// OSP-D CRs can be mass-deleted
	if len(crLists.OpenStackBaremetalSets.Items) > 0 {
		foundRemaining = true
		if err := r.GetClient().DeleteAllOf(ctx, &ospdirectorv1beta1.OpenStackBaremetalSet{}, client.InNamespace(namespace)); err != nil {
			return false, err
		}
	}

	if len(crLists.OpenStackProvisionServers.Items) > 0 {
		foundRemaining = true
		if err := r.GetClient().DeleteAllOf(ctx, &ospdirectorv1beta1.OpenStackProvisionServer{}, client.InNamespace(namespace)); err != nil {
			return false, err
		}
	}

	if len(crLists.OpenStackControlPlanes.Items) > 0 {
		foundRemaining = true
		if err := r.GetClient().DeleteAllOf(ctx, &ospdirectorv1beta1.OpenStackControlPlane{}, client.InNamespace(namespace)); err != nil {
			return false, err
		}
	}

	if len(crLists.OpenStackVMSets.Items) > 0 {
		foundRemaining = true
		if err := r.GetClient().DeleteAllOf(ctx, &ospdirectorv1beta1.OpenStackVMSet{}, client.InNamespace(namespace)); err != nil {
			return false, err
		}
	}

	if len(crLists.OpenStackClients.Items) > 0 {
		foundRemaining = true
		if err := r.GetClient().DeleteAllOf(ctx, &ospdirectorv1beta1.OpenStackClient{}, client.InNamespace(namespace)); err != nil {
			return false, err
		}
	}

	if len(crLists.OpenStackMACAddresses.Items) > 0 {
		foundRemaining = true
		if err := r.GetClient().DeleteAllOf(ctx, &ospdirectorv1beta1.OpenStackMACAddress{}, client.InNamespace(namespace)); err != nil {
			return false, err
		}
	}

	if len(crLists.OpenStackNets.Items) > 0 {
		foundRemaining = true
		if err := r.GetClient().DeleteAllOf(ctx, &ospdirectorv1beta1.OpenStackNet{}, client.InNamespace(namespace)); err != nil {
			return false, err
		}
	}

	if len(crLists.OpenStackNetAttachments.Items) > 0 {
		foundRemaining = true
		if err := r.GetClient().DeleteAllOf(ctx, &ospdirectorv1beta1.OpenStackNetAttachment{}, client.InNamespace(namespace)); err != nil {
			return false, err
		}
	}

	if len(crLists.OpenStackNetConfigs.Items) > 0 {
		foundRemaining = true
		if err := r.GetClient().DeleteAllOf(ctx, &ospdirectorv1beta1.OpenStackNetConfig{}, client.InNamespace(namespace)); err != nil {
			return false, err
		}
	}

	// Can't mass-delete ConfigMaps and Secrets because k8s infrastructure uses namespace for these as well
	for _, cm := range cmList.Items {
		foundRemaining = true
		if err := r.GetClient().Delete(ctx, &cm, &client.DeleteOptions{}); err != nil && !k8s_errors.IsNotFound(err) {
			return false, err
		}
	}

	for _, secret := range secretList.Items {
		foundRemaining = true
		if err := r.GetClient().Delete(ctx, &secret, &client.DeleteOptions{}); err != nil && !k8s_errors.IsNotFound(err) {
			return false, err
		}
	}

	return !foundRemaining, nil
}
