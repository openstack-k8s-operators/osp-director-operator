/*


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

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/diff"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	controlplane "github.com/openstack-k8s-operators/osp-director-operator/pkg/controlplane"
	openstackclient "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackclient"
	openstackipset "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackipset"
	openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
	vmset "github.com/openstack-k8s-operators/osp-director-operator/pkg/vmset"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OSPVersion - global var to specify tge OSP version, defaults to OSP16.2 (train)
var OSPVersion = ospdirectorv1beta1.TemplateVersionTrain

// OpenStackControlPlaneReconciler reconciles an OpenStackControlPlane object
type OpenStackControlPlaneReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackControlPlaneReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackControlPlaneReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackControlPlaneReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackControlPlaneReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackcontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackcontrolplanes/finalizers,verbs=update
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackvmsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackvmsets/finalizers,verbs=update
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackmacaddresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hco.kubevirt.io,namespace=openstack,resources="*",verbs="*"
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=create;delete;get;list;patch;update;watch

// Reconcile - control plane
func (r *OpenStackControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("controlplane", req.NamespacedName)

	// Fetch the controlplane instance
	instance := &ospdirectorv1beta1.OpenStackControlPlane{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, instance.Status.ProvisioningStatus.State == ospdirectorv1beta1.ControlPlaneProvisioned)

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("OpenStackControlPlane %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	// Used in comparisons below to determine whether a status update is actually needed
	newProvStatus := ospdirectorv1beta1.OpenStackControlPlaneProvisioningStatus{}

	//
	// Set the OSP version, the version is usually set in the ctlplane webhook,
	// so this is mostly for when running local with no webhooks and no OpenStackRelease is provided
	//
	var OSPVersion ospdirectorv1beta1.OSPVersion
	if instance.Spec.OpenStackRelease != "" {
		OSPVersion, err = ospdirectorv1beta1.GetOSPVersion(instance.Spec.OpenStackRelease)
	} else {
		OSPVersion = ospdirectorv1beta1.OSPVersion(ospdirectorv1beta1.TemplateVersion16_2)
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	instance.Status.OSPVersion = OSPVersion

	// Secret - containing Tripleo Passwords
	envVars := make(map[string]common.EnvSetter)

	// check if "tripleo-passwords" controlplane.TripleoPasswordSecret secret already exist
	_, secretHash, err := common.GetSecret(r, controlplane.TripleoPasswordSecret, instance.Namespace)
	if err != nil && k8s_errors.IsNotFound(err) {

		r.Log.Info(fmt.Sprintf("Creating password secret: %s", controlplane.TripleoPasswordSecret))

		pwSecretLabel := common.GetLabels(instance, controlplane.AppLabel, map[string]string{})

		templateParameters := make(map[string]interface{})
		templateParameters["TripleoPasswords"] = common.GeneratePasswords()
		pwSecret := []common.Template{
			{
				Name:               controlplane.TripleoPasswordSecret,
				Namespace:          instance.Namespace,
				Type:               common.TemplateTypeConfig,
				InstanceType:       instance.Kind,
				AdditionalTemplate: map[string]string{},
				Labels:             pwSecretLabel,
				ConfigOptions:      templateParameters,
			},
		}

		err = common.EnsureSecrets(r, instance, pwSecret, &envVars)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil {
		newProvStatus.State = ospdirectorv1beta1.ControlPlaneError
		newProvStatus.Reason = err.Error()
		_ = r.setProvisioningStatus(instance, newProvStatus)
		return ctrl.Result{}, fmt.Errorf("error get secret %s: %v", controlplane.TripleoPasswordSecret, err)
	}
	envVars[controlplane.TripleoPasswordSecret] = common.EnvValue(secretHash)

	// Secret - used for deployment to ssh into the overcloud nodes,
	//          gets added to the controller VMs cloud-admin user using cloud-init
	deploymentSecretName := strings.ToLower(controlplane.AppLabel) + "-ssh-keys"

	deploymentSecret, secretHash, err := common.GetSecret(r, deploymentSecretName, instance.Namespace)
	if err != nil && k8s_errors.IsNotFound(err) {
		var op controllerutil.OperationResult

		r.Log.Info(fmt.Sprintf("Creating deployment ssh secret: %s", deploymentSecretName))
		deploymentSecret, err = common.SSHKeySecret(deploymentSecretName, instance.Namespace, map[string]string{deploymentSecretName: ""})
		if err != nil {
			newProvStatus.State = ospdirectorv1beta1.ControlPlaneError
			newProvStatus.Reason = err.Error()
			_ = r.setProvisioningStatus(instance, newProvStatus)
			return ctrl.Result{}, err
		}
		secretHash, op, err = common.CreateOrUpdateSecret(r, instance, deploymentSecret)
		if err != nil {
			newProvStatus.State = ospdirectorv1beta1.ControlPlaneError
			newProvStatus.Reason = err.Error()
			_ = r.setProvisioningStatus(instance, newProvStatus)
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			r.Log.Info(fmt.Sprintf("Secret %s successfully reconciled - operation: %s", deploymentSecret.Name, string(op)))
		}
	} else if err != nil {
		newProvStatus.State = ospdirectorv1beta1.ControlPlaneError
		newProvStatus.Reason = err.Error()
		_ = r.setProvisioningStatus(instance, newProvStatus)
		return ctrl.Result{}, fmt.Errorf("error get secret %s: %v", deploymentSecretName, err)
	}
	envVars[deploymentSecret.Name] = common.EnvValue(secretHash)

	if instance.Spec.PasswordSecret != "" {
		// check if specified password secret exists before creating the controlplane
		_, _, err = common.GetSecret(r, instance.Spec.PasswordSecret, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				err2 := fmt.Errorf("PasswordSecret %s not found but specified in CR, next reconcile in 30s", instance.Spec.PasswordSecret)
				newProvStatus.State = ospdirectorv1beta1.ControlPlaneWaiting
				newProvStatus.Reason = err2.Error()
				_ = r.setProvisioningStatus(instance, newProvStatus)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, err2
			}
			newProvStatus.State = ospdirectorv1beta1.ControlPlaneError
			newProvStatus.Reason = err.Error()
			_ = r.setProvisioningStatus(instance, newProvStatus)
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("PasswordSecret %s exists", instance.Spec.PasswordSecret))
	}

	if instance.Spec.IdmSecret != "" {
		_, _, err = common.GetSecret(r, instance.Spec.IdmSecret, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				err2 := fmt.Errorf("IdmSecret %s not found but specified in CR, next reconcile in 30s", instance.Spec.IdmSecret)
				newProvStatus.State = ospdirectorv1beta1.ControlPlaneWaiting
				newProvStatus.Reason = err2.Error()
				_ = r.setProvisioningStatus(instance, newProvStatus)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, err2
			}
			newProvStatus.State = ospdirectorv1beta1.ControlPlaneError
			newProvStatus.Reason = err.Error()
			_ = r.setProvisioningStatus(instance, newProvStatus)
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("IdmSecret %s exists", instance.Spec.IdmSecret))
	}

	if instance.Spec.CAConfigMap != "" {
		_, _, err = common.GetConfigMapAndHashWithName(r, instance.Spec.CAConfigMap, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				err2 := fmt.Errorf("CAConfigMap %s not found but specified in CR, next reconcile in 30s", instance.Spec.CAConfigMap)
				newProvStatus.State = ospdirectorv1beta1.ControlPlaneWaiting
				newProvStatus.Reason = err2.Error()
				_ = r.setProvisioningStatus(instance, newProvStatus)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, err2
			}
			newProvStatus.State = ospdirectorv1beta1.ControlPlaneError
			newProvStatus.Reason = err.Error()
			_ = r.setProvisioningStatus(instance, newProvStatus)
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("CAConfigMap %s exists", instance.Spec.CAConfigMap))
	}

	if instance.Status.VIPStatus == nil {
		instance.Status.VIPStatus = map[string]ospdirectorv1beta1.HostStatus{}
	}

	//
	// create hostnames for the overcloud VIP
	//
	hostnameDetails := common.Hostname{
		Basename: controlplane.Role,
		VIP:      true,
	}
	err = common.CreateOrGetHostname(instance, &hostnameDetails)
	if err != nil {
		newProvStatus.State = ospdirectorv1beta1.ControlPlaneError
		newProvStatus.Reason = err.Error()
		_ = r.setProvisioningStatus(instance, newProvStatus)
		return ctrl.Result{}, err
	}

	if _, ok := instance.Status.VIPStatus[hostnameDetails.Hostname]; !ok {
		instance.Status.VIPStatus[hostnameDetails.Hostname] = ospdirectorv1beta1.HostStatus{
			Hostname: hostnameDetails.Hostname,
			HostRef:  strings.ToLower(instance.Name),
		}

		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			r.Log.Error(err, "Failed to update CR status %v")
			return ctrl.Result{}, err
		}
	}

	hostnameRefs := instance.GetHostnames()
	// get a copy of the current CR status
	currentStatus := instance.Status.DeepCopy()
	vmSets := []*ospdirectorv1beta1.OpenStackVMSet{}

	//
	// Create VIPs for networks where VIP parameter is true
	//

	// create list of networks where Spec.VIP == True
	vipNetworksList, err := r.createVIPNetworkList(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	ipsetDetails := common.IPSet{
		Networks:            vipNetworksList,
		Role:                controlplane.Role,
		HostCount:           controlplane.Count,
		VIP:                 true,
		AddToPredictableIPs: true,
		HostNameRefs:        hostnameRefs,
	}
	ipset, op, err := openstackipset.OvercloudipsetCreateOrUpdate(r, instance, ipsetDetails)
	if err != nil {
		newProvStatus.State = ospdirectorv1beta1.ControlPlaneError
		newProvStatus.Reason = err.Error()
		_ = r.setProvisioningStatus(instance, newProvStatus)
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("IPSet for %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	if len(ipset.Status.HostIPs) < controlplane.Count {
		newProvStatus.State = ospdirectorv1beta1.ControlPlaneWaiting
		newProvStatus.Reason = fmt.Sprintf("IPSet has not yet reached the required replicas %d", controlplane.Count)
		_ = r.setProvisioningStatus(instance, newProvStatus)
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	// update VIP network status
	for _, netName := range vipNetworksList {
		// TODO: mschuppert, host status format now used for controlplane, openstackclient and vmset, TBD baremetalset
		r.setNetStatus(instance, &hostnameDetails, netName, ipset.Status.HostIPs[hostnameDetails.Hostname].IPAddresses[netName])
	}

	for _, vmRole := range instance.Spec.VirtualMachineRoles {
		actualStatus := instance.Status
		if !reflect.DeepEqual(currentStatus, &actualStatus) {
			err = r.Client.Status().Update(context.TODO(), instance)
			if err != nil {
				r.Log.Error(err, "Failed to update CR status %v")
				return ctrl.Result{}, err
			}
			r.Log.Info(fmt.Sprintf("VIP network status for Hostname: %s - %s", instance.Status.VIPStatus[hostnameDetails.Hostname].Hostname, instance.Status.VIPStatus[hostnameDetails.Hostname].IPAddresses))
		}

		// Create or update the vmSet CR object
		// TODO: mschuppert move to method like createOrUpdateOpenStackMACAddress to make reconcile easier to read
		vmSet := &ospdirectorv1beta1.OpenStackVMSet{
			ObjectMeta: metav1.ObjectMeta{
				// use the role name as the VM CR name
				Name:      strings.ToLower(vmRole.RoleName),
				Namespace: instance.Namespace,
			},
		}

		op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, vmSet, func() error {
			vmSet.Spec.VMCount = vmRole.RoleCount
			vmSet.Spec.Cores = vmRole.Cores
			vmSet.Spec.Memory = vmRole.Memory
			vmSet.Spec.DiskSize = vmRole.DiskSize
			if instance.Spec.DomainName != "" {
				vmSet.Spec.DomainName = instance.Spec.DomainName
			}
			vmSet.Spec.BootstrapDNS = instance.Spec.DNSServers
			vmSet.Spec.DNSSearchDomains = instance.Spec.DNSSearchDomains
			if vmRole.StorageClass != "" {
				vmSet.Spec.StorageClass = vmRole.StorageClass
			}
			vmSet.Spec.BaseImageVolumeName = vmRole.DeepCopy().BaseImageVolumeName
			vmSet.Spec.DeploymentSSHSecret = deploymentSecretName
			vmSet.Spec.CtlplaneInterface = vmRole.CtlplaneInterface
			vmSet.Spec.Networks = vmRole.Networks
			vmSet.Spec.RoleName = vmRole.RoleName
			vmSet.Spec.IsTripleoRole = vmRole.IsTripleoRole
			if instance.Spec.PasswordSecret != "" {
				vmSet.Spec.PasswordSecret = instance.Spec.PasswordSecret
			}

			err := controllerutil.SetControllerReference(instance, vmSet, r.Scheme)
			if err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			newProvStatus.State = ospdirectorv1beta1.ControlPlaneError
			newProvStatus.Reason = err.Error()
			_ = r.setProvisioningStatus(instance, newProvStatus)
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			r.Log.Info(fmt.Sprintf("VMSet CR %s successfully reconciled - operation: %s", instance.Name, string(op)))
		}
		vmSets = append(vmSets, vmSet)
	}

	// TODO:
	// - check vm container status and update CR.Status.VMsReady
	// - change CR.Status.VMs to be struct with name + Pod IP of the controllers

	// Create or update the MACAddress CR object
	err = r.createOrUpdateOpenStackMACAddress(instance, newProvStatus)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create openstack client pod
	// TODO: mschuppert move to method like createOrUpdateOpenStackMACAddress to make reconcile easier to read
	osc := &ospdirectorv1beta1.OpenStackClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openstackclient",
			Namespace: instance.Namespace,
		},
	}
	op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, osc, func() error {
		osc.Spec.ImageURL = instance.Spec.OpenStackClientImageURL
		osc.Spec.DeploymentSSHSecret = deploymentSecretName
		osc.Spec.CloudName = instance.Name
		osc.Spec.StorageClass = instance.Spec.OpenStackClientStorageClass
		osc.Spec.GitSecret = instance.Spec.GitSecret
		osc.Spec.RunUID = openstackclient.CloudAdminUID
		osc.Spec.RunGID = openstackclient.CloudAdminGID
		if instance.Spec.DomainName != "" {
			osc.Spec.DomainName = instance.Spec.DomainName
		}
		osc.Spec.DNSServers = instance.Spec.DNSServers
		osc.Spec.DNSSearchDomains = instance.Spec.DNSSearchDomains
		if instance.Spec.IdmSecret != "" {
			osc.Spec.IdmSecret = instance.Spec.IdmSecret
		}
		if instance.Spec.CAConfigMap != "" {
			osc.Spec.CAConfigMap = instance.Spec.CAConfigMap
		}

		if len(instance.Spec.OpenStackClientNetworks) > 0 {
			osc.Spec.Networks = instance.Spec.OpenStackClientNetworks
		}

		err := controllerutil.SetControllerReference(instance, osc, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		newProvStatus.State = ospdirectorv1beta1.ControlPlaneError
		newProvStatus.Reason = err.Error()
		_ = r.setProvisioningStatus(instance, newProvStatus)
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("OpenStackClient CR successfully reconciled - operation: %s", string(op)))
	}

	// Calculate overall status

	clientPod, err := r.Kclient.CoreV1().Pods(instance.Namespace).Get(context.TODO(), "openstackclient", metav1.GetOptions{})

	if err != nil && !k8s_errors.IsNotFound(err) {
		newProvStatus.State = ospdirectorv1beta1.ControlPlaneError
		newProvStatus.Reason = err.Error()
		_ = r.setProvisioningStatus(instance, newProvStatus)
		return ctrl.Result{}, err
	}

	var ctlPlaneState ospdirectorv1beta1.ControlPlaneProvisioningState
	var reasonMsg string
	vmSetStateCounts := map[ospdirectorv1beta1.VMSetProvisioningState]int{}

	for _, vmSet := range vmSets {
		if vmSet.Status.ProvisioningStatus.State == ospdirectorv1beta1.VMSetCondTypeError {
			// An error overrides all aggregrate state considerations
			ctlPlaneState = ospdirectorv1beta1.ControlPlaneError
			reasonMsg = fmt.Sprintf("Underlying OSVMSet %s hit an error: %s", vmSet.Name, vmSet.Status.ProvisioningStatus.Reason)
		} else {
			vmSetStateCounts[vmSet.Status.ProvisioningStatus.State]++
		}
	}

	if ctlPlaneState == "" {
		// No overrides were set, so calculate an appropriate aggregate status.
		// TODO?: Currently considering states in an arbitrary order of priority here...
		if vmSetStateCounts[ospdirectorv1beta1.VMSetCondTypeProvisioning] > 0 {
			ctlPlaneState = ospdirectorv1beta1.ControlPlaneProvisioning
			reasonMsg = "One or more OSVMSets are provisioning"
		} else if vmSetStateCounts[ospdirectorv1beta1.VMSetCondTypeDeprovisioning] > 0 {
			ctlPlaneState = ospdirectorv1beta1.ControlPlaneDeprovisioning
			reasonMsg = "One or more OSVMSets are deprovisioning"
		} else if vmSetStateCounts[ospdirectorv1beta1.VMSetCondTypeWaiting] > 0 || vmSetStateCounts[""] > 0 {
			ctlPlaneState = ospdirectorv1beta1.ControlPlaneWaiting
			reasonMsg = "Waiting on one or more OSVMSets to initialize or continue"
		} else {
			// If we get here, the only states possible for the VMSets are provisioned or empty,
			// which both count as provisioned
			ctlPlaneState = ospdirectorv1beta1.ControlPlaneProvisioned
			reasonMsg = "All requested OSVMSets have been provisioned"
		}
	}

	newProvStatus.ClientReady = (clientPod != nil && clientPod.Status.Phase == corev1.PodRunning)
	newProvStatus.DesiredCount = len(instance.Spec.VirtualMachineRoles)
	newProvStatus.ReadyCount = vmSetStateCounts[ospdirectorv1beta1.VMSetCondTypeProvisioned] + vmSetStateCounts[ospdirectorv1beta1.VMSetCondTypeEmpty]
	newProvStatus.State = ctlPlaneState
	newProvStatus.Reason = reasonMsg

	return ctrl.Result{}, r.setProvisioningStatus(instance, newProvStatus)
}

// SetupWithManager -
func (r *OpenStackControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// watch for objects in the same namespace as the controller CR
	podWatcher := handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// verify if pods label match any of:
		// osp-director.openstack.org/controller: osp-vmset
		// osp-director.openstack.org/controller: osp-openstackclient
		controllers := map[string]bool{
			vmset.AppLabel:           true,
			openstackclient.AppLabel: true,
		}
		labels := obj.GetLabels()
		controller, ok := labels[common.OwnerControllerNameLabelSelector]
		if ok || controllers[controller] {
			// get all CRs from the same namespace
			crs := &ospdirectorv1beta1.OpenStackControlPlaneList{}
			listOpts := []client.ListOption{
				client.InNamespace(obj.GetNamespace()),
			}
			if err := r.Client.List(context.Background(), crs, listOpts...); err != nil {
				r.Log.Error(err, "Unable to retrieve CRs %v")
				return nil
			}

			for _, cr := range crs.Items {
				if obj.GetNamespace() == cr.Namespace {
					// return namespace and Name of CR
					name := client.ObjectKey{
						Namespace: cr.Namespace,
						Name:      cr.Name,
					}
					result = append(result, reconcile.Request{NamespacedName: name})
				}
			}
		}
		return result
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackControlPlane{}).
		Owns(&corev1.Secret{}).
		Owns(&ospdirectorv1beta1.OpenStackVMSet{}).
		Owns(&ospdirectorv1beta1.OpenStackClient{}).
		Owns(&ospdirectorv1beta1.OpenStackMACAddress{}).
		// watch vmset and openstackclient pods in the same namespace
		// as we want to reconcile if VMs or openstack client pods change
		Watches(&source.Kind{Type: &corev1.Pod{}}, podWatcher).
		Complete(r)
}

func (r *OpenStackControlPlaneReconciler) setNetStatus(instance *ospdirectorv1beta1.OpenStackControlPlane, hostnameDetails *common.Hostname, netName string, ipaddress string) {

	// If OpenStackControlPlane status map is nil, create it
	if instance.Status.VIPStatus == nil {
		instance.Status.VIPStatus = map[string]ospdirectorv1beta1.HostStatus{}
	}

	// Set network information status
	if instance.Status.VIPStatus[hostnameDetails.Hostname].IPAddresses == nil {
		instance.Status.VIPStatus[hostnameDetails.Hostname] = ospdirectorv1beta1.HostStatus{
			Hostname: hostnameDetails.Hostname,
			HostRef:  strings.ToLower(instance.Name),
			IPAddresses: map[string]string{
				netName: ipaddress,
			},
		}
	} else {
		status := instance.Status.VIPStatus[hostnameDetails.Hostname]
		status.HostRef = strings.ToLower(instance.Name)
		status.IPAddresses[netName] = ipaddress
		instance.Status.VIPStatus[hostnameDetails.Hostname] = status
	}

}

func (r *OpenStackControlPlaneReconciler) setProvisioningStatus(instance *ospdirectorv1beta1.OpenStackControlPlane, newProvStatus ospdirectorv1beta1.OpenStackControlPlaneProvisioningStatus) error {

	// if the current ProvisioningStatus is different from the actual, store the update
	// otherwise, just log the status again
	if !reflect.DeepEqual(instance.Status.ProvisioningStatus, newProvStatus) {
		r.Log.Info(fmt.Sprintf("%s - diff %s", instance.Status.ProvisioningStatus.Reason, diff.ObjectReflectDiff(instance.Status.ProvisioningStatus, newProvStatus)))
		instance.Status.ProvisioningStatus = newProvStatus

		instance.Status.Conditions = ospdirectorv1beta1.ConditionList{}
		instance.Status.Conditions.Set(ospdirectorv1beta1.ConditionType(newProvStatus.State), corev1.ConditionTrue, ospdirectorv1beta1.ConditionReason(newProvStatus.Reason), newProvStatus.Reason)

		if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
			r.Log.Error(err, "Failed to update CR status %v")
			return err
		}
	} else if newProvStatus.Reason != "" {
		r.Log.Info(newProvStatus.Reason)
	}

	return nil
}

func (r *OpenStackControlPlaneReconciler) createVIPNetworkList(instance *ospdirectorv1beta1.OpenStackControlPlane) ([]string, error) {

	// create uniq list networls of all VirtualMachineRoles
	networkList := make(map[string]bool)
	uniqNetworksList := []string{}

	for _, vmRole := range instance.Spec.VirtualMachineRoles {
		for _, networkNameLower := range vmRole.Networks {

			// get network with name_lower label to verify if VIP needs to be requested from Spec
			network, err := openstacknet.GetOpenStackNetWithLabel(
				r,
				instance.Namespace,
				map[string]string{
					openstacknet.SubNetNameLabelSelector: networkNameLower,
				},
			)
			if err != nil {
				if k8s_errors.IsNotFound(err) {
					r.Log.Info(fmt.Sprintf("OpenStackNet with NameLower %s not found!", networkNameLower))
					continue
				}
				// Error reading the object - requeue the request.
				return uniqNetworksList, err
			}

			if _, value := networkList[networkNameLower]; !value && network.Spec.VIP {
				networkList[networkNameLower] = true
				uniqNetworksList = append(uniqNetworksList, networkNameLower)
			}
		}
	}

	return uniqNetworksList, nil
}

func (r *OpenStackControlPlaneReconciler) createOrUpdateOpenStackMACAddress(instance *ospdirectorv1beta1.OpenStackControlPlane, newProvStatus ospdirectorv1beta1.OpenStackControlPlaneProvisioningStatus) error {

	mac := &ospdirectorv1beta1.OpenStackMACAddress{
		ObjectMeta: metav1.ObjectMeta{
			// use the role name as the VM CR name
			Name:      strings.ToLower(instance.Name),
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, mac, func() error {
		if len(instance.Spec.PhysNetworks) == 0 {
			mac.Spec.PhysNetworks = []ospdirectorv1beta1.Physnet{
				{
					Name:      controlplane.DefaultOVNChassisPhysNetName,
					MACPrefix: controlplane.DefaultOVNChassisPhysNetMACPrefix,
				},
			}
		} else {
			macPhysnets := []ospdirectorv1beta1.Physnet{}
			for _, physnet := range instance.Spec.PhysNetworks {
				macPrefix := physnet.MACPrefix
				// make sure if MACPrefix was not speficied to set the default prefix
				if macPrefix == "" {
					macPrefix = controlplane.DefaultOVNChassisPhysNetMACPrefix
				}
				macPhysnets = append(macPhysnets, ospdirectorv1beta1.Physnet{
					Name:      physnet.Name,
					MACPrefix: macPrefix,
				})
			}

			mac.Spec.PhysNetworks = macPhysnets
		}

		err := controllerutil.SetControllerReference(instance, mac, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		newProvStatus.State = ospdirectorv1beta1.ControlPlaneError
		newProvStatus.Reason = err.Error()
		_ = r.setProvisioningStatus(instance, newProvStatus)
		return err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("OpenStackMACAddress CR successfully reconciled - operation: %s", string(op)))
	}

	return nil
}
