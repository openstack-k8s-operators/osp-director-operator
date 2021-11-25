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
	"strings"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/client-go/kubernetes"
	virtv1 "kubevirt.io/client-go/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	controlplane "github.com/openstack-k8s-operators/osp-director-operator/pkg/controlplane"
	openstackconfigversion "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackconfigversion"
	openstackplaybookgenerator "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackplaybookgenerator"
)

// OpenStackPlaybookGeneratorReconciler reconciles a OpenStackPlaybookGenerator object
type OpenStackPlaybookGeneratorReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackPlaybookGeneratorReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackPlaybookGeneratorReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackPlaybookGeneratorReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackPlaybookGeneratorReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackplaybookgenerators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackplaybookgenerators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackplaybookgenerators/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;create;update;delete;watch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackvmsets,verbs=get;list
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbaremetalsets,verbs=get;list
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackcontrolplanes,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances,verbs=get;list;watch

// Reconcile - PlaybookGenerator
func (r *OpenStackPlaybookGeneratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstackplaybookgenerator", req.NamespacedName)

	// Fetch the instance
	instance := &ospdirectorv1beta1.OpenStackPlaybookGenerator{}
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

	// Initialize conditions list if not already set
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = ospdirectorv1beta1.ConditionList{}
	}

	envVars := make(map[string]common.EnvSetter)
	templateParameters := make(map[string]interface{})
	cmLabels := common.GetLabels(instance, openstackplaybookgenerator.AppLabel, map[string]string{})

	// get unified OSPVersion from ControlPlane CR
	// which means also get either 16.2 or 17.0 for upstream versions
	controlPlane, ctrlResult, err := common.GetControlPlane(r, instance)
	if err != nil {
		_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
		return ctrlResult, err
	}
	OSPVersion, err := common.GetOSPVersion(string(controlPlane.Status.OSPVersion))
	if err != nil {
		_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
		return ctrlResult, err
	}

	// check if heat-env-config (customizations provided by administrator) exist
	// if it does not exist, requeue
	tripleoCustomDeployCM, _, err := common.GetConfigMapAndHashWithName(r, instance.Spec.HeatEnvConfigMap, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			msg := fmt.Sprintf("The ConfigMap %s doesn't yet exist. Requeing...", instance.Spec.HeatEnvConfigMap)
			r.Log.Info(msg)
			err = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorWaiting, msg)
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}
		// Ignore any potential error from "setCurrentState"; focus on previous error
		// NOTE: This pattern will be repeated a lot below, but the comment will not be copied
		_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
		return ctrl.Result{}, err
	}

	tripleoCustomDeployFiles := tripleoCustomDeployCM.Data
	templateParameters["TripleoCustomDeployFiles"] = tripleoCustomDeployFiles

	// Now read the tripleo-deploy-config and the fencing-config CMs for use in the PlaybookGenerator
	tripleoDeployCM, _, err := common.GetConfigMapAndHashWithName(r, "tripleo-deploy-config", instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			msg := "The tripleo-deploy-config map doesn't yet exist. Requeing..."
			r.Log.Info(msg)
			err = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorWaiting, msg)
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}
		_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
		return ctrl.Result{}, err
	}

	tripleoDeployFiles := tripleoDeployCM.Data
	// Delete network_data.yaml from tripleoDeployFiles as it is not an ooo parameter env file
	delete(tripleoDeployFiles, "network_data.yaml")
	// Delete all role nic templates from tripleoDeployFiles as it is not an ooo parameter env file
	for k := range tripleoDeployFiles {
		if strings.HasSuffix(k, "nic-template.yaml") || strings.HasSuffix(k, "nic-template.j2") {
			delete(tripleoDeployFiles, k)
		}
	}

	// Also add fencing.yaml to the tripleoDeployFiles (just need the file name)
	templateParameters["TripleoDeployFiles"] = tripleoDeployFiles
	templateParameters["HeatServiceName"] = "heat-" + instance.Name
	templateParameters["OSPVersion"] = OSPVersion

	var tripleoTarballCM *corev1.ConfigMap

	// extra stuff if custom tarballs are provided
	if instance.Spec.TarballConfigMap != "" {
		tripleoTarballCM, _, err = common.GetConfigMapAndHashWithName(r, instance.Spec.TarballConfigMap, instance.Namespace)
		if err != nil {
			if errors.IsNotFound(err) {
				err = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorWaiting, "Tarball config map not found, requeuing and waiting")
				return ctrl.Result{RequeueAfter: time.Second * 10}, err
			}
			_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
			return ctrl.Result{}, err
		}
		tripleoTarballFiles := tripleoTarballCM.BinaryData
		templateParameters["TripleoTarballFiles"] = tripleoTarballFiles
	}

	cms := []common.Template{
		{
			Name:               "openstackplaybook-script-" + instance.Name,
			Namespace:          instance.Namespace,
			Type:               common.TemplateTypeScripts,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{},
			ConfigOptions:      templateParameters,
			Labels:             cmLabels,
		},
	}
	err = common.EnsureConfigMaps(r, instance, cms, &envVars)
	if err != nil {
		_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
		return ctrl.Result{}, err
	}

	//
	// BEGIN: Fencing considerations
	//

	// Reuse templateParameters, but reinitialize it (envVars and cmLabels are also reused, but do not require
	// reinitialization)
	templateParameters = make(map[string]interface{})

	if controlPlane.Status.ProvisioningStatus.State != ospdirectorv1beta1.ControlPlaneProvisioned {
		msg := fmt.Sprintf("Control plane %s VMs are not yet provisioned. Requeing...", controlPlane.Name)
		r.Log.Info(msg)
		err = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorWaiting, msg)
		return ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	templateParameters["EnableFencing"] = false

	if controlPlane.Spec.EnableFencing {
		fencingRoles := common.GetFencingRoles()

		// First check if custom roles were included that require fencing
		if tripleoTarballCM != nil {
			customFencingRoles, err := common.GetCustomFencingRoles(tripleoTarballCM.BinaryData)

			if err != nil {
				_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
				return ctrl.Result{}, err
			}

			fencingRoles = append(fencingRoles, customFencingRoles...)
		}

		// TODO: This will likely need refactoring sooner or later
		var virtualMachineInstanceLists []*virtv1.VirtualMachineInstanceList

		for roleName, roleParams := range controlPlane.Spec.VirtualMachineRoles {
			if common.StringInSlice(roleParams.RoleName, fencingRoles) && roleParams.RoleCount == 3 {
				// Get the associated VM instances
				virtualMachineInstanceList, err := common.GetVirtualMachineInstances(r, instance.Namespace, map[string]string{
					common.OwnerNameLabelSelector: strings.ToLower(roleName),
				})

				if err != nil {
					_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
					return ctrl.Result{}, err
				}

				virtualMachineInstanceLists = append(virtualMachineInstanceLists, virtualMachineInstanceList)
			}
		}

		if len(virtualMachineInstanceLists) > 0 {
			templateParameters["EnableFencing"] = true

			fencingTemplateParameters, err := controlplane.CreateFencingConfigMapParams(virtualMachineInstanceLists)

			if err != nil {
				_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
				return ctrl.Result{}, err
			}

			templateParameters = common.MergeMaps(templateParameters, fencingTemplateParameters)
		}
	}

	cm := []common.Template{
		{
			Name:               "fencing-config",
			Namespace:          instance.Namespace,
			Type:               common.TemplateTypeConfig,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{},
			Labels:             cmLabels,
			ConfigOptions:      templateParameters,
		},
	}

	err = common.EnsureConfigMaps(r, instance, cm, &envVars)
	if err != nil {
		_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
		return ctrl.Result{}, err
	}

	fencingCM, _, err := common.GetConfigMapAndHashWithName(r, "fencing-config", instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// This should really never happen, but putting the check here anyhow
			msg := "The fencing-config map doesn't yet exist. Requeing..."
			r.Log.Info(msg)
			err = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorWaiting, msg)
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}
		_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
		return ctrl.Result{}, err
	}

	//
	// END: Fencing considerations
	//

	// Calc config map hash
	hashList := []interface{}{
		tripleoCustomDeployCM.Data,
		tripleoDeployCM.Data,
		fencingCM.Data,
	}

	if tripleoTarballCM != nil {
		hashList = append(hashList, tripleoTarballCM.BinaryData)
	}

	configMapHash, err := common.ObjectHash(hashList)
	if err != nil {
		_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
		return ctrl.Result{}, fmt.Errorf("error calculating configmap hash: %v", err)
	}

	// Create ephemeral heat
	heat := &ospdirectorv1beta1.OpenStackEphemeralHeat{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: ospdirectorv1beta1.OpenStackEphemeralHeatSpec{
			ConfigHash:         configMapHash,
			HeatAPIImageURL:    instance.Spec.EphemeralHeatSettings.HeatAPIImageURL,
			HeatEngineImageURL: instance.Spec.EphemeralHeatSettings.HeatEngineImageURL,
			MariadbImageURL:    instance.Spec.EphemeralHeatSettings.MariadbImageURL,
			RabbitImageURL:     instance.Spec.EphemeralHeatSettings.RabbitImageURL,
			HeatEngineReplicas: instance.Spec.EphemeralHeatSettings.HeatEngineReplicas,
		},
	}

	// Define a new Job object
	job := openstackplaybookgenerator.PlaybookJob(instance, configMapHash, OSPVersion)

	if instance.Status.PlaybookHash != configMapHash {
		op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, heat, func() error {
			err := controllerutil.SetControllerReference(instance, heat, r.Scheme)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return ctrl.Result{}, err
		}

		if op != controllerutil.OperationResultNone {
			msg := fmt.Sprintf("OpenStackEphemeralHeat successfully created/updated - operation: %s", string(op))
			r.Log.Info(msg)
			if err := r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorInitializing, msg); err != nil {
				return ctrl.Result{}, err
			}
			r.Log.Info("Requeuing...")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		if !heat.Status.Active {
			msg := "Waiting on Ephemeral Heat instance to launch..."
			r.Log.Info(msg)
			err = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorInitializing, msg)
			return ctrl.Result{RequeueAfter: time.Second * 5}, err
		}

		// configMap Hash changed after Ephemeral Heat was created
		if heat.Spec.ConfigHash != configMapHash {
			err = r.Client.Delete(context.TODO(), heat)
			if err != nil && !k8s_errors.IsNotFound(err) {
				_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
				return ctrl.Result{}, err
			}

			msg := "ConfigMap has changed. Requeing to start again..."
			r.Log.Info(msg)
			err = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorInitializing, msg)
			return ctrl.Result{RequeueAfter: time.Second * 5}, err
		}

		// check if osvmset and osbms is in status provisioned
		msg, deployed, err := r.verifyNodeResourceStatus(instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !deployed {
			timeout := 20
			r.Log.Info(msg)
			err = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorWaiting, msg)
			return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
		}

		op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, job, func() error {

			err := controllerutil.SetControllerReference(instance, job, r.Scheme)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil && !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			r.Log.Info(fmt.Sprintf("Job successfully created/updated - operation: %s", string(op)))
			r.Log.Info("Requeuing...")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		// configMap Hash changed while job was running (NOTE: configHash is only ENV in the job)
		r.Log.Info(fmt.Sprintf("Job Hash : %s", job.Spec.Template.Spec.Containers[0].Env[0].Value))
		r.Log.Info(fmt.Sprintf("ConfigMap Hash : %s", configMapHash))
		if configMapHash != job.Spec.Template.Spec.Containers[0].Env[0].Value {
			_, err = common.DeleteJob(job, r.Kclient, r.Log)
			if err != nil {
				_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
				return ctrl.Result{}, err
			}

			// in this case delete heat too as the database may have been used
			r.Log.Info("Deleting Ephemeral Heat...")
			err = r.Client.Delete(context.TODO(), heat)
			if err != nil && !k8s_errors.IsNotFound(err) {
				_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
				return ctrl.Result{}, err
			}
			msg := "ConfigMap has changed. Requeing to start again..."
			r.Log.Info(msg)
			err = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorInitializing, msg)
			return ctrl.Result{RequeueAfter: time.Second * 5}, err

		}

		requeue, err := common.WaitOnJob(job, r.Client, r.Log)
		r.Log.Info("Generating Playbooks...")
		if err != nil {
			// the job failed in error
			_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
			r.Log.Info("Job failed... Please check job/pod logs.")
			return ctrl.Result{}, err
		} else if requeue {
			msg := "Waiting on Playbook Generation..."
			r.Log.Info(msg)
			if err := r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorGenerating, msg); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
	}

	if err := r.setPlaybookHash(instance, configMapHash); err != nil {
		if !strings.Contains(err.Error(), registry.OptimisticLockErrorMsg) {
			_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
		}
		return ctrl.Result{}, err
	}
	_, err = common.DeleteJob(job, r.Kclient, r.Log)
	if err != nil {
		_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
		return ctrl.Result{}, err
	}

	// cleanup the ephemeral Heat
	err = r.Client.Delete(context.TODO(), heat)
	if err != nil && !k8s_errors.IsNotFound(err) {
		_ = r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorError, err.Error())
		return ctrl.Result{}, err
	}

	// update ConfigVersions with Git Commits
	configVersions, gerr := openstackconfigversion.SyncGit(instance, r.Client, r.Log)
	if gerr != nil {
		r.Log.Error(gerr, "ConfigVersions")
		return ctrl.Result{}, gerr
	}

	if err := r.syncConfigVersions(instance, configVersions); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.setCurrentState(instance, ospdirectorv1beta1.PlaybookGeneratorFinished, "The OpenStackPlaybookGenerator job has completed"); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *OpenStackPlaybookGeneratorReconciler) setPlaybookHash(instance *ospdirectorv1beta1.OpenStackPlaybookGenerator, hashStr string) error {

	if hashStr != instance.Status.PlaybookHash {
		instance.Status.PlaybookHash = hashStr
		if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
			return err
		}
	}
	return nil

}

func (r *OpenStackPlaybookGeneratorReconciler) setCurrentState(instance *ospdirectorv1beta1.OpenStackPlaybookGenerator, currentState ospdirectorv1beta1.PlaybookGeneratorState, msg string) error {

	if currentState != instance.Status.CurrentState {
		instance.Status.CurrentState = currentState
		instance.Status.Conditions = ospdirectorv1beta1.ConditionList{}
		// TODO: Using msg as reason and message for now
		instance.Status.Conditions.Set(ospdirectorv1beta1.ConditionType(currentState), corev1.ConditionTrue, ospdirectorv1beta1.ConditionReason(msg), msg)
		if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
			r.Log.Error(err, "OpenStackPlaybookGenerator update status error: %v")
			return err
		}
	}
	return nil

}

func (r *OpenStackPlaybookGeneratorReconciler) syncConfigVersions(instance *ospdirectorv1beta1.OpenStackPlaybookGenerator, configVersions map[string]ospdirectorv1beta1.OpenStackConfigVersion) error {

	for _, version := range configVersions {

		// Check if this ConfigVersion already exists
		foundVersion := &ospdirectorv1beta1.OpenStackConfigVersion{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: version.Name, Namespace: instance.Namespace}, foundVersion)
		if err == nil {
			//FIXME(dprince): update existing?
		} else if err != nil && k8s_errors.IsNotFound(err) {
			r.Log.Info("Creating a ConfigVersion", "ConfigVersion.Namespace", instance.Namespace, "ConfigVersion.Name", version.Name)
			err = r.Client.Create(context.TODO(), &version)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	//FIXME(dprince): remove deleted?
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackPlaybookGeneratorReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// watch for objects in the same namespace as the controller CR
	namespacedFn := handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// get all CRs from the same namespace (there should only be one)
		crs := &ospdirectorv1beta1.OpenStackPlaybookGeneratorList{}
		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.Client.List(context.Background(), crs, listOpts...); err != nil {
			r.Log.Error(err, "Unable to retrieve CRs %v")
			return nil
		}

		for _, cr := range crs.Items {
			if o.GetNamespace() == cr.Namespace {
				// return namespace and Name of CR
				name := client.ObjectKey{
					Namespace: cr.Namespace,
					Name:      cr.Name,
				}
				result = append(result, reconcile.Request{NamespacedName: name})
			}
		}
		if len(result) > 0 {
			return result
		}
		return nil
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackPlaybookGenerator{}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, namespacedFn).
		Owns(&corev1.ConfigMap{}).
		Owns(&ospdirectorv1beta1.OpenStackEphemeralHeat{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *OpenStackPlaybookGeneratorReconciler) verifyNodeResourceStatus(instance *ospdirectorv1beta1.OpenStackPlaybookGenerator) (string, bool, error) {

	msg := ""

	// check if all osvmset are in status provisioned
	vmsetList := &ospdirectorv1beta1.OpenStackVMSetList{}

	listOpts := []client.ListOption{}
	err := r.Client.List(context.TODO(), vmsetList, listOpts...)
	if err != nil {
		return msg, false, err
	}

	for _, vmset := range vmsetList.Items {
		if vmset.Status.ProvisioningStatus.State != ospdirectorv1beta1.VMSetProvisioned {
			msg := fmt.Sprintf("Waiting on OpenStackVMset %s to be provisioned...", vmset.Name)
			return msg, false, nil
		}
	}

	// check if all osbms are in status provisioned
	bmsetList := &ospdirectorv1beta1.OpenStackBaremetalSetList{}

	listOpts = []client.ListOption{}
	err = r.Client.List(context.TODO(), bmsetList, listOpts...)
	if err != nil {
		return msg, false, err
	}

	for _, bmset := range bmsetList.Items {
		//
		// wait for all BMS be provisioned if baremetalhosts for the bms are requested
		//
		if bmset.Status.ProvisioningStatus.State != ospdirectorv1beta1.BaremetalSetProvisioningState(ospdirectorv1beta1.BaremetalSetProvisioned) &&
			bmset.Status.ProvisioningStatus.State != ospdirectorv1beta1.BaremetalSetProvisioningState(ospdirectorv1beta1.BaremetalSetEmpty) {
			msg := fmt.Sprintf("Waiting on OpenStackBaremetalSet %s to be provisioned...", bmset.Name)
			return msg, false, nil
		}
	}

	return msg, true, nil
}
