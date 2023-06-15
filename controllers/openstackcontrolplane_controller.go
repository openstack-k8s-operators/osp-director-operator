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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	ospdirectorv1beta2 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta2"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	controlplane "github.com/openstack-k8s-operators/osp-director-operator/pkg/controlplane"
	openstackclient "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackclient"
	openstackipset "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackipset"
	vmset "github.com/openstack-k8s-operators/osp-director-operator/pkg/vmset"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	storageversionmigrations "sigs.k8s.io/kube-storage-version-migrator/pkg/apis/migration/v1alpha1"
)

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
// +kubebuilder:rbac:groups=migration.k8s.io,resources=storageversionmigrations,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=create;delete;get;list;patch;update;watch

// Reconcile - control plane
func (r *OpenStackControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("controlplane", req.NamespacedName)

	// Fetch the controlplane instance
	instance := &ospdirectorv1beta2.OpenStackControlPlane{}
	err := r.Get(ctx, req.NamespacedName, instance)
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

	//
	// initialize condition
	//
	cond := &shared.Condition{}

	if instance.Status.VIPStatus == nil {
		instance.Status.VIPStatus = map[string]ospdirectorv1beta2.HostStatus{}
	}

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, instance.IsReady())

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		err = common.WrapErrorForObject(
			fmt.Sprintf("OpenStackControlPlane %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name),
			instance,
			err,
		)

		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	//
	// Used in comparisons below to determine whether a status update is actually needed
	//
	currentStatus := instance.Status.DeepCopy()
	statusChanged := func() bool {
		return !equality.Semantic.DeepEqual(
			r.getNormalizedStatus(&instance.Status),
			r.getNormalizedStatus(currentStatus),
		)
	}

	defer func(cond *shared.Condition) {
		//
		// Update object conditions
		//
		instance.Status.Conditions.UpdateCurrentCondition(
			cond.Type,
			cond.Reason,
			cond.Message,
		)

		instance.Status.ProvisioningStatus.Reason = cond.Message
		instance.Status.ProvisioningStatus.State = shared.ProvisioningState(cond.Type)

		if statusChanged() {
			if updateErr := r.Status().Update(context.Background(), instance); updateErr != nil {
				common.LogErrorForObject(r, updateErr, "Update status", instance)
			}
		}

		// log current status message to operator log
		common.LogForObject(r, cond.Message, instance)
	}(cond)

	envVars := make(map[string]common.EnvSetter)

	//
	// verify if API storageversionmigration is required
	//
	ctrlResult, err := r.ensureStorageVersionMigration(ctx, instance, cond)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// Set the OSP version, the version is usually set in the ctlplane webhook,
	// so this is mostly for when running local with no webhooks and no OpenStackRelease is provided
	//
	var OSPVersion shared.OSPVersion
	if instance.Spec.OpenStackRelease != "" {
		OSPVersion, err = shared.GetOSPVersion(instance.Spec.OpenStackRelease)
	} else {
		OSPVersion = shared.OSPVersion(shared.TemplateVersion16_2)
	}
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = shared.ControlPlaneReasonNotSupportedVersion
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrl.Result{}, err
	}
	instance.Status.OSPVersion = OSPVersion

	//
	// create or get hash of "tripleo-passwords" controlplane.TripleoPasswordSecret secret
	//
	err = r.createOrGetTripleoPasswords(ctx, instance, cond, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// Secret - used for deployment to ssh into the overcloud nodes,
	//          gets added to the controller VMs cloud-admin user using cloud-init
	//
	deploymentSecret, err := r.createOrGetDeploymentSecret(ctx, instance, cond, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// check if specified PasswordSecret secret exists
	//
	ctrlResult, err = r.verifySecretExist(ctx, instance, cond, instance.Spec.PasswordSecret)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// check if specified IdmSecret secret exists
	//
	ctrlResult, err = r.verifySecretExist(ctx, instance, cond, instance.Spec.IdmSecret)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// check if specified CAConfigMap config map exists
	//
	ctrlResult, err = r.verifyConfigMapExist(ctx, instance, cond, instance.Spec.CAConfigMap)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// create VIPs for networks where VIP parameter is true
	// AND the service VIPs for Spec.AdditionalServiceVIPs
	//
	ctrlResult, err = r.ensureVIPs(
		ctx,
		instance,
		cond,
	)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// Create VMSets
	//
	vmSets, err := r.createOrUpdateVMSets(
		ctx,
		instance,
		cond,
		deploymentSecret,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO:
	// - check vm container status and update CR.Status.VMsReady
	// - change CR.Status.VMs to be struct with name + Pod IP of the controllers

	//
	// Create openstack client pod
	//
	osc, err := r.createOrUpdateOpenStackClient(
		ctx,
		instance,
		cond,
		deploymentSecret,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// Calculate overall status
	//

	// 1) OpenStackClient pod status
	clientPod, err := r.Kclient.CoreV1().Pods(instance.Namespace).Get(ctx, osc.Name, metav1.GetOptions{})
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			timeout := 30
			cond.Message = fmt.Sprintf("%s pod %s not found, next reconcile in %d s", osc.Kind, osc.Name, timeout)
			cond.Reason = shared.OsClientCondReasonPodMissing
			cond.Type = shared.CommonCondTypeWaiting

			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, nil
		}

		cond.Message = fmt.Sprintf("%s pod %s error", osc.Kind, osc.Name)
		cond.Reason = shared.OsClientCondReasonPodError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrl.Result{}, err
	}

	instance.Status.ProvisioningStatus.ClientReady = (clientPod != nil && clientPod.Status.Phase == corev1.PodRunning)

	// 2) OpenStackVMSet status
	vmSetStateCounts := map[shared.ProvisioningState]int{}
	for _, vmSet := range vmSets {
		if vmSet.Status.ProvisioningStatus.State == shared.ProvisioningState(shared.VMSetCondTypeError) {
			// An error overrides all aggregrate state considerations
			vmSetCondition := vmSet.Status.Conditions.GetCurrentCondition()
			cond.Message = fmt.Sprintf("Underlying OSVMSet %s hit an error: %s", vmSet.Name, vmSet.Status.ProvisioningStatus.Reason)
			cond.Reason = vmSetCondition.Reason
			cond.Type = vmSetCondition.Type

			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}
		vmSetStateCounts[shared.ProvisioningState(vmSet.Status.ProvisioningStatus.State)]++
	}

	instance.Status.ProvisioningStatus.DesiredCount = len(instance.Spec.VirtualMachineRoles)
	instance.Status.ProvisioningStatus.ReadyCount =
		vmSetStateCounts[shared.ProvisioningState(shared.VMSetCondTypeProvisioned)] +
			vmSetStateCounts[shared.ProvisioningState(shared.VMSetCondTypeEmpty)]

	// TODO?: Currently considering states in an arbitrary order of priority here...
	if vmSetStateCounts[shared.ProvisioningState(shared.VMSetCondTypeProvisioning)] > 0 {
		cond.Type = shared.ControlPlaneProvisioning
		cond.Reason = shared.VMSetCondReasonProvisioning
		cond.Message = "One or more OSVMSets are provisioning"
	} else if vmSetStateCounts[shared.ProvisioningState(shared.VMSetCondTypeDeprovisioning)] > 0 {
		cond.Type = shared.ControlPlaneDeprovisioning
		cond.Reason = shared.VMSetCondReasonDeprovisioning
		cond.Message = "One or more OSVMSets are deprovisioning"
	} else if vmSetStateCounts[shared.ProvisioningState(shared.VMSetCondTypeWaiting)] > 0 || vmSetStateCounts[""] > 0 {
		cond.Type = shared.ControlPlaneWaiting
		cond.Reason = shared.VMSetCondReasonInitialize
		cond.Message = "Waiting on one or more OSVMSets to initialize or continue"
	} else {
		// If we get here, the only states possible for the VMSets are provisioned or empty,
		// which both count as provisioned
		cond.Type = shared.ControlPlaneProvisioned
		cond.Reason = shared.VMSetCondReasonProvisioned
		cond.Message = "All requested OSVMSets have been provisioned"
	}

	return ctrl.Result{}, nil
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
			crs := &ospdirectorv1beta2.OpenStackControlPlaneList{}
			listOpts := []client.ListOption{
				client.InNamespace(obj.GetNamespace()),
			}
			if err := r.List(context.Background(), crs, listOpts...); err != nil {
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
		For(&ospdirectorv1beta2.OpenStackControlPlane{}).
		Owns(&corev1.Secret{}).
		Owns(&ospdirectorv1beta1.OpenStackIPSet{}).
		Owns(&ospdirectorv1beta2.OpenStackVMSet{}).
		Owns(&ospdirectorv1beta1.OpenStackClient{}).
		Owns(&storageversionmigrations.StorageVersionMigration{}).

		// watch vmset and openstackclient pods in the same namespace
		// as we want to reconcile if VMs or openstack client pods change
		Watches(&source.Kind{Type: &corev1.Pod{}}, podWatcher).
		Complete(r)
}

func (r *OpenStackControlPlaneReconciler) getNormalizedStatus(status *ospdirectorv1beta2.OpenStackControlPlaneStatus) *ospdirectorv1beta2.OpenStackControlPlaneStatus {

	//
	// set LastHeartbeatTime and LastTransitionTime to a default value as those
	// need to be ignored to compare if conditions changed.
	//
	s := status.DeepCopy()
	for idx := range s.Conditions {
		s.Conditions[idx].LastHeartbeatTime = metav1.Time{}
		s.Conditions[idx].LastTransitionTime = metav1.Time{}
	}

	return s
}

// create or get hash of "tripleo-passwords" controlplane.TripleoPasswordSecret secret
func (r *OpenStackControlPlaneReconciler) createOrGetTripleoPasswords(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackControlPlane,
	cond *shared.Condition,
	envVars *map[string]common.EnvSetter,
) error {
	//
	// check if "tripleo-passwords" controlplane.TripleoPasswordSecret secret already exist
	//
	secret, secretHash, err := common.GetSecret(ctx, r, controlplane.TripleoPasswordSecret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {

			templateParameters := make(map[string]interface{})
			templateParameters["TripleoPasswords"] = common.GeneratePasswords()

			err := r.createOrUpdatePasswordSecret(ctx, instance, cond, envVars, templateParameters)
			if err != nil {
				return err
			}

			common.LogForObject(
				r,
				fmt.Sprintf("Created Tripleo Passwords Secret: %s", controlplane.TripleoPasswordSecret),
				instance,
			)
		} else {
			cond.Message = fmt.Sprintf("Error get TripleoPasswordsSecret %s", controlplane.TripleoPasswordSecret)
			cond.Reason = shared.ControlPlaneReasonTripleoPasswordsSecretError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}
	} else {
		tripleoPasswordsByte := secret.Data["tripleo-overcloud-passwords.yaml"]

		tripleoPasswordsRaw := make(map[string]interface{})
		if err := yaml.Unmarshal(tripleoPasswordsByte, &tripleoPasswordsRaw); err != nil {
			cond.Message = fmt.Sprintf("Error extract TripleoPasswords from Secret %s", controlplane.TripleoPasswordSecret)
			cond.Reason = shared.ControlPlaneReasonTripleoPasswordsSecretError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}

		// recursive convert nested map[interface{}]interface{} password data
		// into map[string]interface{} and merge into a new tripleoPasswords map
		tripleoPasswords, err := common.RecursiveMergeMaps(
			map[string]interface{}{},
			tripleoPasswordsRaw,
			common.PasswordMaxDepth,
		)
		if err != nil {
			cond.Message = fmt.Sprintf("Error recursive merge TripleoPasswords from Secret %s", controlplane.TripleoPasswordSecret)
			cond.Reason = shared.ControlPlaneReasonTripleoPasswordsSecretError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}

		// get actual password level of the password data, so everything bellow "parameter_defaults"
		currentPasswordsMap := tripleoPasswords["parameter_defaults"].(map[string]interface{})

		// If there are netsted maps bellow the highest level, Marshal them into a json string
		for key, val := range currentPasswordsMap {
			if val == nil {
				cond.Message = fmt.Sprintf("Error, password for %s is nil!", key)
				cond.Reason = shared.ControlPlaneReasonTripleoPasswordsSecretError
				cond.Type = shared.CommonCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err
			}

			if common.IsInterfaceMap(val) {
				b, err := json.Marshal(val)
				if err != nil {
					cond.Message = fmt.Sprintf("Error marshal nested password maps from TripleoPasswords from Secret %s", controlplane.TripleoPasswordSecret)
					cond.Reason = shared.ControlPlaneReasonTripleoPasswordsSecretError
					cond.Type = shared.CommonCondTypeError
					err = common.WrapErrorForObject(cond.Message, instance, err)

					return err
				}

				currentPasswordsMap[key] = string(b)
			}
		}

		// validate if new password entries need to be added
		newPasswordsMap := make(map[string]interface{})
		for _, pwName := range common.PasswordNames() {
			if _, ok := currentPasswordsMap[pwName]; !ok {
				newPasswordsMap[pwName] = common.GeneratePassword(pwName)

				common.LogForObject(
					r,
					fmt.Sprintf("New Password %s added to TripleoPasswords list", pwName),
					instance,
				)
			}
		}

		if len(newPasswordsMap) > 0 {
			templateParameters := make(map[string]interface{})
			templateParameters["TripleoPasswords"] = common.MergeMaps(currentPasswordsMap, newPasswordsMap)

			err := r.createOrUpdatePasswordSecret(ctx, instance, cond, envVars, templateParameters)
			if err != nil {
				return err
			}

			common.LogForObject(
				r,
				fmt.Sprintf("Updated Tripleo Passwords Secret with: %v", reflect.ValueOf(newPasswordsMap).MapKeys()),
				instance,
			)
		}
	}

	(*envVars)[controlplane.TripleoPasswordSecret] = common.EnvValue(secretHash)

	return nil
}

func (r *OpenStackControlPlaneReconciler) createOrUpdatePasswordSecret(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackControlPlane,
	cond *shared.Condition,
	envVars *map[string]common.EnvSetter,
	templateParameters map[string]interface{},
) error {

	pwSecretLabel := common.GetLabels(instance, controlplane.AppLabel, map[string]string{})

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

	err := common.EnsureSecrets(ctx, r, instance, pwSecret, envVars)
	if err != nil {
		cond.Message = fmt.Sprintf("Error creating TripleoPasswordsSecret %s", controlplane.TripleoPasswordSecret)
		cond.Reason = shared.ControlPlaneReasonTripleoPasswordsSecretCreateError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	return nil
}

// Secret - used for deployment to ssh into the overcloud nodes,
//
//	gets added to the controller VMs cloud-admin user using cloud-init
func (r *OpenStackControlPlaneReconciler) createOrGetDeploymentSecret(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackControlPlane,
	cond *shared.Condition,
	envVars *map[string]common.EnvSetter,
) (*corev1.Secret, error) {
	deploymentSecretName := strings.ToLower(controlplane.AppLabel) + "-ssh-keys"

	deploymentSecret, secretHash, err := common.GetSecret(ctx, r, deploymentSecretName, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {

			var op controllerutil.OperationResult

			common.LogForObject(
				r,
				fmt.Sprintf("Creating deployment ssh secret: %s", deploymentSecretName),
				instance,
			)

			deploymentSecret, err = common.SSHKeySecret(
				deploymentSecretName,
				instance.Namespace,
				map[string]string{deploymentSecretName: ""},
			)
			if err != nil {
				cond.Message = fmt.Sprintf("Error creating ssh keys %s", deploymentSecretName)
				cond.Reason = shared.ControlPlaneReasonDeploymentSSHKeysGenError
				cond.Type = shared.CommonCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return deploymentSecret, err
			}

			secretHash, op, err = common.CreateOrUpdateSecret(ctx, r, instance, deploymentSecret)
			if err != nil {
				cond.Message = fmt.Sprintf("Error create or update ssh keys secret %s", deploymentSecretName)
				cond.Reason = shared.ControlPlaneReasonDeploymentSSHKeysSecretCreateOrUpdateError
				cond.Type = shared.CommonCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return deploymentSecret, err
			}
			if op != controllerutil.OperationResultNone {
				common.LogForObject(
					r,
					fmt.Sprintf("Secret %s successfully reconciled - operation: %s", deploymentSecret.Name, string(op)),
					instance,
				)
			}
		} else {
			cond.Message = fmt.Sprintf("Error get secret %s", deploymentSecretName)
			cond.Reason = shared.CommonCondReasonDeploymentSecretError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return deploymentSecret, err
		}
	}
	(*envVars)[deploymentSecret.Name] = common.EnvValue(secretHash)

	return deploymentSecret, nil
}

func (r *OpenStackControlPlaneReconciler) verifySecretExist(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackControlPlane,
	cond *shared.Condition,
	secretName string,
) (ctrl.Result, error) {
	if secretName != "" {
		// check if specified secret exists before creating the controlplane
		_, _, err := common.GetSecret(ctx, r, secretName, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				timeout := 30
				cond.Message = fmt.Sprintf("Secret %s not found but specified in CR, next reconcile in %d s", secretName, timeout)
				cond.Reason = shared.CommonCondReasonSecretMissing
				cond.Type = shared.CommonCondTypeWaiting

				common.LogForObject(r, cond.Message, instance)

				return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, nil
			}
			// Error reading the object - requeue the request.
			cond.Message = fmt.Sprintf("Error reading secret object: %s", secretName)
			cond.Reason = shared.CommonCondReasonSecretError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}

		common.LogForObject(
			r,
			fmt.Sprintf("Secret %s exists", secretName),
			instance,
		)
	}
	return ctrl.Result{}, nil
}

func (r *OpenStackControlPlaneReconciler) verifyConfigMapExist(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackControlPlane,
	cond *shared.Condition,
	configMapName string,
) (ctrl.Result, error) {

	if configMapName != "" {
		_, _, err := common.GetConfigMapAndHashWithName(ctx, r, instance.Spec.CAConfigMap, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				timeout := 30
				cond.Message = fmt.Sprintf("ConfigMap %s not found but specified in CR, next reconcile in %d s", configMapName, timeout)
				cond.Reason = shared.CommonCondReasonConfigMapMissing
				cond.Type = shared.CommonCondTypeWaiting

				common.LogForObject(r, cond.Message, instance)

				return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, nil
			}
			// Error reading the object - requeue the request.
			cond.Message = fmt.Sprintf("Error reading config map object: %s", configMapName)
			cond.Reason = shared.CommonCondReasonConfigMapError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}
		common.LogForObject(
			r,
			fmt.Sprintf("Secret %s exists", configMapName),
			instance,
		)
	}
	return ctrl.Result{}, nil
}

// Create VMSets
func (r *OpenStackControlPlaneReconciler) createOrUpdateVMSets(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackControlPlane,
	cond *shared.Condition,
	deploymentSecret *corev1.Secret,
) ([]*ospdirectorv1beta2.OpenStackVMSet, error) {
	vmSets := []*ospdirectorv1beta2.OpenStackVMSet{}

	for _, vmRole := range instance.Spec.VirtualMachineRoles {

		//
		// Create or update the vmSet CR object
		//
		vmSet := &ospdirectorv1beta2.OpenStackVMSet{
			ObjectMeta: metav1.ObjectMeta{
				// use the role name as the VM CR name
				Name:      strings.ToLower(vmRole.RoleName),
				Namespace: instance.Namespace,
			},
		}

		op, err := controllerutil.CreateOrPatch(ctx, r.Client, vmSet, func() error {
			vmSet.Spec.VMCount = vmRole.RoleCount
			vmSet.Spec.Cores = vmRole.Cores
			vmSet.Spec.Memory = vmRole.Memory
			if len(vmSet.Spec.IOThreadsPolicy) > 0 {
				vmSet.Spec.IOThreadsPolicy = vmRole.IOThreadsPolicy
			}
			vmSet.Spec.BlockMultiQueue = vmRole.BlockMultiQueue
			vmSet.Spec.RootDisk = vmRole.RootDisk
			vmSet.Spec.AdditionalDisks = vmRole.AdditionalDisks
			vmSet.Spec.DeploymentSSHSecret = deploymentSecret.Name
			vmSet.Spec.CtlplaneInterface = vmRole.CtlplaneInterface
			vmSet.Spec.Networks = vmRole.Networks
			vmSet.Spec.RoleName = vmRole.RoleName
			vmSet.Spec.IsTripleoRole = vmRole.IsTripleoRole
			if instance.Spec.PasswordSecret != "" {
				vmSet.Spec.PasswordSecret = instance.Spec.PasswordSecret
			}
			vmSet.Spec.NodeSelector = vmRole.NodeSelector
			vmSet.Spec.GrowvolsArgs = vmRole.GrowvolsArgs

			err := controllerutil.SetControllerReference(instance, vmSet, r.Scheme)
			if err != nil {
				cond.Message = fmt.Sprintf("Error set controller reference for %s %s", vmSet.Kind, vmSet.Name)
				cond.Reason = shared.CommonCondReasonControllerReferenceError
				cond.Type = shared.CommonCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err
			}

			return nil
		})

		if err != nil {
			cond.Message = fmt.Sprintf("Failed to create or update %s %s ", vmSet.Kind, vmSet.Name)
			cond.Reason = shared.VMSetCondReasonError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return vmSets, err
		}

		cond.Message = fmt.Sprintf("%s %s successfully reconciled", vmSet.Kind, vmSet.Name)
		if op != controllerutil.OperationResultNone {
			cond.Message = fmt.Sprintf("%s - operation: %s", cond.Message, string(op))

			common.LogForObject(
				r,
				cond.Message,
				instance,
			)
		}
		cond.Reason = shared.VMSetCondReasonCreated
		cond.Type = shared.CommonCondTypeCreated

		vmSets = append(vmSets, vmSet)
	}

	return vmSets, nil
}

func (r *OpenStackControlPlaneReconciler) createOrUpdateOpenStackClient(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackControlPlane,
	cond *shared.Condition,
	deploymentSecret *corev1.Secret,
) (*ospdirectorv1beta1.OpenStackClient, error) {
	osc := &ospdirectorv1beta1.OpenStackClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openstackclient",
			Namespace: instance.Namespace,
		},
	}
	op, err := controllerutil.CreateOrPatch(ctx, r.Client, osc, func() error {
		if instance.Spec.OpenStackClientImageURL != "" {
			osc.Spec.ImageURL = instance.Spec.OpenStackClientImageURL
		}
		osc.Spec.DeploymentSSHSecret = deploymentSecret.Name
		osc.Spec.CloudName = instance.Name
		osc.Spec.StorageClass = instance.Spec.OpenStackClientStorageClass
		osc.Spec.RunUID = openstackclient.CloudAdminUID
		osc.Spec.RunGID = openstackclient.CloudAdminGID
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
			cond.Message = fmt.Sprintf("Error set controller reference for %s %s", osc.Kind, osc.Name)
			cond.Reason = shared.CommonCondReasonControllerReferenceError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}

		return nil
	})
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update %s %s ", osc.Kind, osc.Name)
		cond.Reason = shared.OsClientCondReasonError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return osc, err
	}

	cond.Message = fmt.Sprintf("%s %s successfully reconciled", osc.Kind, osc.Name)
	if op != controllerutil.OperationResultNone {
		cond.Message = fmt.Sprintf("%s - operation: %s", cond.Message, string(op))

		common.LogForObject(
			r,
			cond.Message,
			instance,
		)
	}
	cond.Reason = shared.OsClientCondReasonCreated
	cond.Type = shared.CommonCondTypeCreated

	return osc, nil
}

func (r *OpenStackControlPlaneReconciler) ensureVIPs(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackControlPlane,
	cond *shared.Condition,
) (ctrl.Result, error) {
	//
	// Create VIPs for networks where VIP parameter is true
	//

	// create list of networks where Spec.VIP == True
	vipNetworksList, err := ospdirectorv1beta2.CreateVIPNetworkList(r.Client, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	var ctrlResult ctrl.Result
	currentLabels := instance.DeepCopy().Labels

	//
	// Only kept for running local
	// add osnetcfg CR label reference which is used in the in the osnetcfg
	// controller to watch this resource and reconcile
	//
	if _, ok := currentLabels[shared.OpenStackNetConfigReconcileLabel]; !ok {
		common.LogForObject(r, "osnetcfg reference label not added by webhook, adding it!", instance)
		instance.Labels, err = ospdirectorv1beta1.AddOSNetConfigRefLabel(
			r.Client,
			instance.Namespace,
			vipNetworksList[0],
			currentLabels,
		)
		if err != nil {
			return ctrlResult, err
		}
	}

	//
	// add labels of all networks used by this CR
	//
	instance.Labels = ospdirectorv1beta1.AddOSNetNameLowerLabels(r.GetLogger(), instance.Labels, vipNetworksList)

	//
	// update instance to sync labels if changed
	//
	if !equality.Semantic.DeepEqual(
		currentLabels,
		instance.Labels,
	) {
		err = r.Update(ctx, instance)
		if err != nil {
			cond.Message = fmt.Sprintf("Failed to update %s %s", instance.Kind, instance.Name)
			cond.Reason = shared.CommonCondReasonAddOSNetLabelError
			cond.Type = shared.CommonCondTypeError

			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}
	}

	//
	// create controlplane VIPs for all networks which has VIP flag
	//
	ipsetStatus, ctrlResult, err := openstackipset.EnsureIPs(
		ctx,
		r,
		instance,
		cond,
		controlplane.Role,
		vipNetworksList,
		1,
		true,
		false,
		[]string{},
		true,
	)

	for _, status := range ipsetStatus {
		hostStatus := ospdirectorv1beta2.SyncIPsetStatus(cond, instance.Status.VIPStatus, status)
		instance.Status.VIPStatus[status.Hostname] = hostStatus
	}

	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// create Service VIPs starting OSP17/wallaby for RedisVirtualFixedIPs and OVNDBsVirtualFixedIPs
	//
	if instance.Status.OSPVersion == shared.OSPVersion(shared.TemplateVersion17_0) ||
		instance.Status.OSPVersion == shared.OSPVersion(shared.TemplateVersion17_1) {
		for service, network := range instance.Spec.AdditionalServiceVIPs {
			ipsetStatus, ctrlResult, err := openstackipset.EnsureIPs(
				ctx,
				r,
				instance,
				cond,
				service,
				[]string{network},
				1,
				false,
				true,
				[]string{},
				true,
			)

			for _, status := range ipsetStatus {
				hostStatus := ospdirectorv1beta2.SyncIPsetStatus(cond, instance.Status.VIPStatus, status)
				instance.Status.VIPStatus[status.Hostname] = hostStatus
			}

			if (err != nil) || (ctrlResult != ctrl.Result{}) {
				return ctrlResult, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// verify if API storageversionmigration is required
func (r *OpenStackControlPlaneReconciler) ensureStorageVersionMigration(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackControlPlane,
	cond *shared.Condition,
) (ctrl.Result, error) {
	// if old vmspec.DiskSize is present in the CR the storageversionmigration needs to be performed
	for _, vmspec := range instance.Spec.VirtualMachineRoles {
		if vmspec.DiskSize > 0 {
			//
			// get all storageversionmigrations for ths app label for this CRD
			//
			smList := &storageversionmigrations.StorageVersionMigrationList{}
			labelSelectorMap := map[string]string{
				common.OwnerControllerNameLabelSelector: controlplane.AppLabel,
			}

			listOpts := []client.ListOption{
				client.MatchingLabels(labelSelectorMap),
			}

			if err := r.List(ctx, smList, listOpts...); err != nil {
				err = fmt.Errorf("Error listing services for %s: %w", smList.GroupVersionKind().Kind, err)
				return ctrl.Result{}, err
			}

			//
			// if empty storageversionmigrations list, create the storageversionmigration
			//
			if len(smList.Items) == 0 {
				sm := &storageversionmigrations.StorageVersionMigration{}
				// plural, add s
				resource := fmt.Sprintf("%ss", strings.ToLower(instance.GroupVersionKind().Kind))
				sm.Name = fmt.Sprintf("%s-storage-version-migration", resource)
				sm.SetLabels(common.GetLabels(instance, controlplane.AppLabel, map[string]string{}))

				sm.Spec.Resource.Group = strings.ToLower(instance.GroupVersionKind().Group)
				sm.Spec.Resource.Resource = resource
				sm.Spec.Resource.Version = "v1beta2"

				err := r.Create(ctx, sm)
				if err != nil {
					return ctrl.Result{}, err
				}
			} else {
				//
				// get storageversionmigrations for this API version
				//
				for _, sm := range smList.Items {
					if sm.Spec.Resource.Version == "v1beta2" {
						currentCond := &storageversionmigrations.MigrationCondition{}
						for i, c := range sm.Status.Conditions {
							if c.Status == corev1.ConditionTrue {
								currentCond = &sm.Status.Conditions[i]
								break
							}
						}

						if currentCond != nil {
							switch currentCond.Type {
							case storageversionmigrations.MigrationSucceeded:
								cond.Message = fmt.Sprintf("All runtime objects %s migrated to new API version",
									strings.ToLower(instance.GroupVersionKind().Group),
								)
								cond.Reason = shared.ConditionReason(currentCond.Reason)
								cond.Type = shared.ConditionType(currentCond.Type)

								common.LogForObject(r, cond.Message, instance)

							case storageversionmigrations.MigrationRunning:
								cond.Message = fmt.Sprintf("waiting for runtime objects %s to be migrated to new API version",
									strings.ToLower(instance.GroupVersionKind().Group),
								)
								cond.Reason = shared.ConditionReason(currentCond.Reason)
								cond.Type = shared.ConditionType(currentCond.Type)
								common.LogForObject(r, cond.Message, instance)

								return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
							case storageversionmigrations.MigrationFailed:
								cond.Message = fmt.Sprintf("storageversionmigration failed for %s",
									strings.ToLower(instance.GroupVersionKind().Group),
								)
								cond.Reason = shared.ConditionReason(currentCond.Reason)
								cond.Type = shared.ConditionType(currentCond.Type)
								err := common.WrapErrorForObject(cond.Message, instance, fmt.Errorf(cond.Message))

								return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, err
							}
						}
					}
				}
			}
		}
	}
	return ctrl.Result{}, nil
}
