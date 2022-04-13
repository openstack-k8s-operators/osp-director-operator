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
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	virtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	controlplane "github.com/openstack-k8s-operators/osp-director-operator/pkg/controlplane"
	openstackconfiggenerator "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackconfiggenerator"
	openstackconfigversion "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackconfigversion"
)

// OpenStackConfigGeneratorReconciler reconciles a OpenStackConfigGenerator object
type OpenStackConfigGeneratorReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackConfigGeneratorReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackConfigGeneratorReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackConfigGeneratorReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackConfigGeneratorReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackconfiggenerators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackconfiggenerators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackconfiggenerators/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;create;update;delete;watch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackvmsets,verbs=get;list
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbaremetalsets,verbs=get;list
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackcontrolplanes,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances,verbs=get;list;watch

// Reconcile - ConfigGenerator
func (r *OpenStackConfigGeneratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstackconfiggenerator", req.NamespacedName)

	// Fetch the instance
	instance := &ospdirectorv1beta1.OpenStackConfigGenerator{}
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

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, instance.IsReady())

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("OpenStackClient %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	//
	// initialize condition
	//
	cond := instance.Status.Conditions.InitCondition()

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
		instance.Status.CurrentState = cond.Type
		instance.Status.CurrentReason = shared.ConditionReason(cond.Message)

		instance.Status.Conditions.UpdateCurrentCondition(
			cond.Type,
			cond.Reason,
			cond.Message,
		)

		if statusChanged() {
			if updateErr := r.Status().Update(context.Background(), instance); updateErr != nil {
				common.LogErrorForObject(r, updateErr, "Update status", instance)
			}
		}

		// log current status message to operator log
		common.LogForObject(r, cond.Message, instance)
	}(cond)

	envVars := make(map[string]common.EnvSetter)
	templateParameters := make(map[string]interface{})
	cmLabels := common.GetLabels(instance, openstackconfiggenerator.AppLabel, map[string]string{})

	//
	// Get OSPVersion from OSControlPlane status
	//
	// unified OSPVersion from ControlPlane CR
	// which means also get either 16.2 or 17.0 for upstream versions
	controlPlane, ctrlResult, err := ospdirectorv1beta1.GetControlPlane(r.Client, instance)
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = shared.ControlPlaneReasonNetNotFound
		cond.Type = shared.ConfigGeneratorCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrlResult, err
	}
	OSPVersion, err := ospdirectorv1beta1.GetOSPVersion(string(controlPlane.Status.OSPVersion))
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = shared.ControlPlaneReasonNotSupportedVersion
		cond.Type = shared.ConfigGeneratorCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrlResult, err
	}

	templateParameters["OSPVersion"] = OSPVersion

	//
	//  wait for the controlplane VMs to be provisioned
	//
	if controlPlane.Status.ProvisioningStatus.State != shared.ProvisioningState(shared.ControlPlaneProvisioned) {
		cond.Message = fmt.Sprintf("Control plane %s VMs are not yet provisioned. Requeing...", controlPlane.Name)
		cond.Reason = shared.ConfigGeneratorCondReasonCMNotFound
		cond.Type = shared.ConfigGeneratorCondTypeWaiting
		common.LogForObject(
			r,
			cond.Message,
			instance,
		)

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	//
	// check if heat-env-config (customizations provided by administrator) exist if it does not exist, requeue
	//
	tripleoCustomDeployCM, _, err := common.GetConfigMapAndHashWithName(ctx, r, instance.Spec.HeatEnvConfigMap, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			cond.Message = fmt.Sprintf("The ConfigMap %s doesn't yet exist. Requeing...", instance.Spec.HeatEnvConfigMap)
			cond.Reason = shared.ConfigGeneratorCondReasonCMNotFound
			cond.Type = shared.ConfigGeneratorCondTypeWaiting
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// add ConfigGeneratorInputLabel if required
	err = r.addConfigGeneratorInputLabel(ctx, instance, cond, tripleoCustomDeployCM)
	if err != nil {
		return ctrl.Result{}, err
	}

	tripleoCustomDeployFiles := tripleoCustomDeployCM.Data
	templateParameters["TripleoCustomDeployFiles"] = tripleoCustomDeployFiles

	//
	// Tarball configmap - extra stuff if custom tarballs are provided
	//
	var tripleoTarballCM *corev1.ConfigMap
	if instance.Spec.TarballConfigMap != "" {
		tripleoTarballCM, _, err = common.GetConfigMapAndHashWithName(ctx, r, instance.Spec.TarballConfigMap, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				cond.Message = "Tarball config map not found, requeuing and waiting. Requeing..."
				cond.Reason = shared.ConfigGeneratorCondReasonCMNotFound
				cond.Type = shared.ConfigGeneratorCondTypeWaiting
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}
		tripleoTarballFiles := tripleoTarballCM.BinaryData
		templateParameters["TripleoTarballFiles"] = tripleoTarballFiles
	}

	// add ConfigGeneratorInputLabel if required
	err = r.addConfigGeneratorInputLabel(ctx, instance, cond, tripleoTarballCM)
	if err != nil {
		return ctrl.Result{}, err
	}

	// add build-in heat environment files
	var tripleoEnvironmentFiles []string
	for _, f := range instance.Spec.HeatEnvs {
		// Join the path to root first to ensure it doesn't escape the t-h-t environments dir.
		cleanPath := filepath.Join("/", f)
		envPath := filepath.Join("environments", cleanPath)
		tripleoEnvironmentFiles = append(tripleoEnvironmentFiles, envPath)
	}
	templateParameters["TripleoEnvironmentFiles"] = tripleoEnvironmentFiles

	//
	// render OOO environment, create TripleoDeployCM and read the tripleo-deploy-config CM
	//
	tripleoDeployCM, err := r.createTripleoDeployCM(
		ctx,
		instance,
		cond,
		&envVars,
		cmLabels,
		OSPVersion,
		&controlPlane,
		tripleoTarballCM,
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	tripleoDeployFiles := tripleoDeployCM.Data

	//
	// Delete network_data.yaml and all role nic templates from tripleoDeployFiles as it is not an ooo parameter env file
	//
	for k := range tripleoDeployFiles {
		if strings.HasSuffix(k, openstackconfiggenerator.RenderedNicFileTrain) ||
			strings.HasSuffix(k, openstackconfiggenerator.RenderedNicFileWallaby) ||
			strings.HasSuffix(k, openstackconfiggenerator.NetworkDataFile) {
			delete(tripleoDeployFiles, k)
		}
	}

	templateParameters["TripleoDeployFiles"] = tripleoDeployFiles
	templateParameters["HeatServiceName"] = "heat-" + instance.Name

	//
	// openstackconfig-script CM
	//
	cms := []common.Template{
		{
			Name:               "openstackconfig-script-" + instance.Name,
			Namespace:          instance.Namespace,
			Type:               common.TemplateTypeScripts,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{},
			ConfigOptions:      templateParameters,
			Labels:             cmLabels,
		},
	}
	err = common.EnsureConfigMaps(ctx, r, instance, cms, &envVars)
	if err != nil {
		cond.Message = fmt.Sprintf("%s config map not found, requeuing and waiting. Requeing...", "openstackconfig-script-"+instance.Name)
		cond.Reason = shared.ConfigGeneratorCondReasonCMNotFound
		cond.Type = shared.ConfigGeneratorCondTypeWaiting
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrl.Result{}, err
	}

	//
	// Calc config map hash
	//
	hashList := []interface{}{
		tripleoCustomDeployCM.Data,
		tripleoDeployCM.Data,
		tripleoEnvironmentFiles,
	}

	if tripleoTarballCM != nil {
		hashList = append(hashList, tripleoTarballCM.BinaryData)
	}

	configMapHash, err := common.ObjectHash(hashList)
	if err != nil {
		cond.Message = "Error calculating configmap hash"
		cond.Reason = shared.ConfigGeneratorCondReasonCMHashError
		cond.Type = shared.ConfigGeneratorCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrl.Result{}, err
	}

	//
	// Create ephemeral heat
	//
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
	job := openstackconfiggenerator.ConfigJob(instance, configMapHash, OSPVersion)

	var exports string
	if instance.Status.ConfigHash != configMapHash {
		op, err := controllerutil.CreateOrPatch(ctx, r.Client, heat, func() error {
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
			cond.Message = fmt.Sprintf("OpenStackEphemeralHeat successfully created/updated - operation: %s", string(op))
			cond.Reason = shared.ConfigGeneratorCondReasonEphemeralHeatUpdated
			cond.Type = shared.ConfigGeneratorCondTypeInitializing
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		if !heat.Status.Active {
			cond.Message = "Waiting on Ephemeral Heat instance to launch..."
			cond.Reason = shared.ConfigGeneratorCondReasonEphemeralHeatLaunch
			cond.Type = shared.ConfigGeneratorCondTypeInitializing
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		// configMap Hash changed after Ephemeral Heat was created
		if heat.Spec.ConfigHash != configMapHash {
			err = r.Delete(ctx, heat)
			if err != nil && !k8s_errors.IsNotFound(err) {
				cond.Message = err.Error()
				cond.Reason = shared.ConfigGeneratorCondReasonCMHashChanged
				cond.Type = shared.ConfigGeneratorCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return ctrl.Result{}, err
			}

			cond.Message = "ConfigMap has changed. Requeing to start again..."
			cond.Reason = shared.ConfigGeneratorCondReasonCMUpdated
			cond.Type = shared.ConfigGeneratorCondTypeInitializing
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		// check if osvmset and osbms is in status provisioned
		msg, deployed, err := r.verifyNodeResourceStatus(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !deployed {
			cond.Message = msg
			cond.Reason = shared.ConfigGeneratorCondReasonWaitingNodesProvisioned
			cond.Type = shared.ConfigGeneratorCondTypeInitializing
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 20}, err
		}

		op, err = controllerutil.CreateOrPatch(ctx, r.Client, job, func() error {
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
			cond.Message = fmt.Sprintf("Job successfully created/updated - operation: %s", string(op))
			cond.Reason = shared.ConfigGeneratorCondReasonJobCreated
			cond.Type = shared.ConfigGeneratorCondTypeInitializing
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		// configMap Hash changed while job was running (NOTE: configHash is only ENV in the job)
		common.LogForObject(r, fmt.Sprintf("Job Hash : %s", job.Spec.Template.Spec.Containers[0].Env[0].Value), instance)
		common.LogForObject(r, fmt.Sprintf("ConfigMap Hash : %s", configMapHash), instance)

		if configMapHash != job.Spec.Template.Spec.Containers[0].Env[0].Value {
			_, err = common.DeleteJob(ctx, job, r.Kclient, r.Log)
			if err != nil {
				cond.Message = err.Error()
				cond.Reason = shared.ConfigGeneratorCondReasonJobDelete
				cond.Type = shared.ConfigGeneratorCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return ctrl.Result{}, err
			}

			// in this case delete heat too as the database may have been used
			r.Log.Info("Deleting Ephemeral Heat...")
			err = r.Delete(ctx, heat)
			if err != nil && !k8s_errors.IsNotFound(err) {
				cond.Message = err.Error()
				cond.Reason = shared.ConfigGeneratorCondReasonEphemeralHeatDelete
				cond.Type = shared.ConfigGeneratorCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return ctrl.Result{}, err
			}

			cond.Message = "ConfigMap has changed. Requeing to start again..."
			cond.Reason = shared.ConfigGeneratorCondReasonCMUpdated
			cond.Type = shared.ConfigGeneratorCondTypeInitializing
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 5}, nil

		}

		requeue, err := common.WaitOnJob(ctx, job, r.Client, r.Log)
		common.LogForObject(r, "Generating Configs...", instance)

		if err != nil {
			// the job failed in error
			cond.Message = "Job failed... Please check job/pod logs."
			cond.Reason = shared.ConfigGeneratorCondReasonJobFailed
			cond.Type = shared.ConfigGeneratorCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		} else if requeue {
			cond.Message = "Waiting on Config Generation..."
			cond.Reason = shared.ConfigGeneratorCondReasonConfigCreate
			cond.Type = shared.ConfigGeneratorCondTypeGenerating
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}
		// obtain the cltplaneExports from Heat
		exports, err = openstackconfiggenerator.CtlplaneExports("heat-"+instance.Name, r.Log)
		if err != nil && !k8s_errors.IsNotFound(err) {
			cond.Message = err.Error()
			cond.Reason = shared.ConfigGeneratorCondReasonExportFailed
			cond.Type = shared.ConfigGeneratorCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}

	}

	r.setConfigHash(instance, configMapHash)

	_, err = common.DeleteJob(ctx, job, r.Kclient, r.Log)
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = shared.ConfigGeneratorCondReasonJobDelete
		cond.Type = shared.ConfigGeneratorCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrl.Result{}, err
	}

	// cleanup the ephemeral Heat
	err = r.Delete(ctx, heat)
	if err != nil && !k8s_errors.IsNotFound(err) {
		cond.Message = err.Error()
		cond.Reason = shared.ConfigGeneratorCondReasonEphemeralHeatDelete
		cond.Type = shared.ConfigGeneratorCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrl.Result{}, err
	}

	// update ConfigVersions with Git Commits
	configVersions, gerr := openstackconfigversion.SyncGit(ctx, instance, r.Client, r.Log)
	if gerr != nil {
		r.Log.Error(gerr, "ConfigVersions")
		return ctrl.Result{}, gerr
	}

	if err := r.syncConfigVersions(ctx, instance, configVersions, exports); err != nil {
		return ctrl.Result{}, err
	}

	cond.Message = "The OpenStackConfigGenerator job has completed"
	cond.Reason = shared.ConfigGeneratorCondReasonJobFinished
	cond.Type = shared.ConfigGeneratorCondTypeFinished

	return ctrl.Result{}, nil
}

func (r *OpenStackConfigGeneratorReconciler) setConfigHash(instance *ospdirectorv1beta1.OpenStackConfigGenerator, hashStr string) {

	if hashStr != instance.Status.ConfigHash {
		instance.Status.ConfigHash = hashStr
	}
}

func (r *OpenStackConfigGeneratorReconciler) getNormalizedStatus(status *ospdirectorv1beta1.OpenStackConfigGeneratorStatus) *ospdirectorv1beta1.OpenStackConfigGeneratorStatus {

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

func (r *OpenStackConfigGeneratorReconciler) syncConfigVersions(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackConfigGenerator,
	configVersions map[string]ospdirectorv1beta1.OpenStackConfigVersion,
	exports string,
) error {

	for _, version := range configVersions {

		// Check if this ConfigVersion already exists
		foundVersion := &ospdirectorv1beta1.OpenStackConfigVersion{}
		err := r.Get(ctx, types.NamespacedName{Name: version.Name, Namespace: instance.Namespace}, foundVersion)
		if err == nil {
			//FIXME(dprince): update existing?
		} else if err != nil && k8s_errors.IsNotFound(err) {
			// we only add the most recent export to new ConfigVersions (just created...)
			version.Spec.CtlplaneExports = exports
			r.Log.Info("Creating a ConfigVersion", "ConfigVersion.Namespace", instance.Namespace, "ConfigVersion.Name", version.Name)
			err = r.Create(ctx, &version)
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
func (r *OpenStackConfigGeneratorReconciler) SetupWithManager(mgr ctrl.Manager) error {

	//
	// Schedule reconcile on openstackconfiggenerator if any of the objects change where
	// owner label openstackconfiggenerator.ConfigGeneratorInputLabel
	//
	ConfigGeneratorInputLabelWatcher := handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		labels := o.GetLabels()
		//
		// verify object has ConfigGeneratorInputLabel
		//
		owner, ok := labels[openstackconfiggenerator.ConfigGeneratorInputLabel]
		if !ok {
			return []reconcile.Request{}
		}

		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{
				Name:      owner,
				Namespace: o.GetNamespace(),
			}},
		}
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackConfigGenerator{}).
		// TODO: watch ctlplane, osbms for Count change
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, ConfigGeneratorInputLabelWatcher).
		Owns(&corev1.ConfigMap{}).
		Owns(&ospdirectorv1beta1.OpenStackEphemeralHeat{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *OpenStackConfigGeneratorReconciler) verifyNodeResourceStatus(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackConfigGenerator,
) (string, bool, error) {

	msg := ""

	// check if all osvmset are in status provisioned
	vmsetList := &ospdirectorv1beta1.OpenStackVMSetList{}

	listOpts := []client.ListOption{}
	err := r.List(ctx, vmsetList, listOpts...)
	if err != nil {
		return msg, false, err
	}

	for _, vmset := range vmsetList.Items {
		if openstackconfiggenerator.IsRoleIncluded(vmset.Spec.RoleName, instance) {
			if vmset.Status.ProvisioningStatus.State != shared.ProvisioningState(shared.VMSetCondTypeProvisioned) {
				msg := fmt.Sprintf("Waiting on OpenStackVMset %s to be provisioned...", vmset.Name)
				return msg, false, nil
			}
		}
	}

	// check if all osbms are in status provisioned
	bmsetList := &ospdirectorv1beta1.OpenStackBaremetalSetList{}

	listOpts = []client.ListOption{}
	err = r.List(ctx, bmsetList, listOpts...)
	if err != nil {
		return msg, false, err
	}

	for _, bmset := range bmsetList.Items {
		//
		// wait for all BMS be provisioned if baremetalhosts for the bms are requested
		//
		if openstackconfiggenerator.IsRoleIncluded(bmset.Spec.RoleName, instance) {
			if bmset.Status.ProvisioningStatus.State != shared.ProvisioningState(shared.BaremetalSetCondTypeProvisioned) &&
				bmset.Status.ProvisioningStatus.State != shared.ProvisioningState(shared.BaremetalSetCondTypeEmpty) {
				msg := fmt.Sprintf("Waiting on OpenStackBaremetalSet %s to be provisioned...", bmset.Name)
				return msg, false, nil
			}
		}
	}

	return msg, true, nil
}

//
// Fencing considerations
//
func (r *OpenStackConfigGeneratorReconciler) createFencingEnvironmentFiles(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackConfigGenerator,
	cond *shared.Condition,
	controlPlane *ospdirectorv1beta1.OpenStackControlPlane,
	tripleoTarballCM *corev1.ConfigMap,
	cmLabels map[string]string,

) (map[string]string, error) {
	templateParameters := make(map[string]interface{})

	//
	//  default to fencing disabled
	//
	templateParameters["EnableFencing"] = false

	if controlPlane.Spec.EnableFencing {
		fencingRoles := common.GetFencingRoles()

		// First check if custom roles were included that require fencing
		if tripleoTarballCM != nil {
			customFencingRoles, err := common.GetCustomFencingRoles(tripleoTarballCM.BinaryData)
			if err != nil {
				cond.Message = err.Error()
				cond.Reason = shared.ConfigGeneratorCondReasonCustomRolesNotFound
				cond.Type = shared.ConfigGeneratorCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return nil, err
			}

			fencingRoles = append(fencingRoles, customFencingRoles...)
		}

		// TODO: This will likely need refactoring sooner or later
		var virtualMachineInstanceLists []*virtv1.VirtualMachineInstanceList

		for roleName, roleParams := range controlPlane.Spec.VirtualMachineRoles {
			if common.StringInSlice(roleParams.RoleName, fencingRoles) && roleParams.RoleCount == 3 {
				// Get the associated VM instances
				virtualMachineInstanceList, err := common.GetVirtualMachineInstances(ctx, r, instance.Namespace, map[string]string{
					common.OwnerNameLabelSelector: strings.ToLower(roleName),
				})
				if err != nil {
					cond.Message = err.Error()
					cond.Reason = shared.ConfigGeneratorCondReasonVMInstanceList
					cond.Type = shared.ConfigGeneratorCondTypeError
					err = common.WrapErrorForObject(cond.Message, instance, err)

					return nil, err
				}

				virtualMachineInstanceLists = append(virtualMachineInstanceLists, virtualMachineInstanceList)
			}
		}

		if len(virtualMachineInstanceLists) > 0 {
			templateParameters["EnableFencing"] = true

			fencingTemplateParameters, err := controlplane.CreateFencingConfigMapParams(virtualMachineInstanceLists)
			if err != nil {
				cond.Message = err.Error()
				cond.Reason = shared.ConfigGeneratorCondReasonFencingTemplateError
				cond.Type = shared.ConfigGeneratorCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return nil, err
			}

			templateParameters = common.MergeMaps(templateParameters, fencingTemplateParameters)
		}
	}

	fencingTemplate := common.Template{
		Name:         "fencing-config",
		Namespace:    instance.Namespace,
		Type:         common.TemplateTypeNone,
		InstanceType: instance.Kind,
		AdditionalTemplate: map[string]string{
			"fencing.yaml": "/openstackconfiggenerator/config/common/fencing.yaml",
		},
		Labels:        cmLabels,
		ConfigOptions: templateParameters,
	}

	renderedFencingTemplate, err := common.GetTemplateData(fencingTemplate)
	if err != nil {
		return nil, err
	}

	return renderedFencingTemplate, nil
}

//
// generate TripleoDeploy configmap with environment file containing predictible IPs
//
func (r *OpenStackConfigGeneratorReconciler) createTripleoDeployCM(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackConfigGenerator,
	cond *shared.Condition,
	envVars *map[string]common.EnvSetter,
	cmLabels map[string]string,
	ospVersion ospdirectorv1beta1.OSPVersion,
	controlPlane *ospdirectorv1beta1.OpenStackControlPlane,
	tripleoTarballCM *corev1.ConfigMap,
) (*corev1.ConfigMap, error) {
	//
	// generate OOO environment file with predictible IPs
	//
	templateParameters, rolesMap, err := openstackconfiggenerator.CreateConfigMapParams(ctx, r, instance, cond)
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = shared.ConfigGeneratorCondReasonRenderEnvFilesError
		cond.Type = shared.ConfigGeneratorCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return nil, err
	}

	//
	// get clusterServiceIP for fencing routing in the nic templates
	//
	clusterServiceIP, err := r.getClusterServiceEndpoint(
		ctx,
		instance,
		cond,
		"default",
		map[string]string{
			"component": "apiserver",
			"provider":  "kubernetes",
		},
	)
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = shared.ConfigGeneratorCondReasonClusterServiceIPError
		cond.Type = shared.ConfigGeneratorCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return nil, err
	}

	//
	// Render VM role nic templates, but only for tripleo roles
	//
	roleNicTemplates, err := r.createVMRoleNicTemplates(
		instance,
		ospVersion,
		rolesMap,
		clusterServiceIP,
		cmLabels,
	)
	if err != nil {
		return nil, err
	}

	//
	// Render fencing template
	//
	fencingTemplate, err := r.createFencingEnvironmentFiles(
		ctx,
		instance,
		cond,
		controlPlane,
		tripleoTarballCM,
		cmLabels,
	)
	if err != nil {
		return nil, err
	}

	//
	// create tripleo-deploy-config configmap with rendered data
	//
	cm := []common.Template{
		{
			Name:               "tripleo-deploy-config-" + instance.Name,
			Namespace:          instance.Namespace,
			Type:               common.TemplateTypeConfig,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{},
			CustomData: shared.MergeStringMaps(
				roleNicTemplates,
				fencingTemplate,
			),
			Labels:        cmLabels,
			ConfigOptions: templateParameters,
			Version:       ospVersion,
		},
	}

	err = common.EnsureConfigMaps(ctx, r, instance, cm, envVars)
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = shared.ConfigGeneratorCondReasonCMCreateError
		cond.Type = shared.ConfigGeneratorCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return nil, err
	}

	//
	// Read the tripleo-deploy-config CM
	//
	tripleoDeployCM, _, err := common.GetConfigMapAndHashWithName(ctx, r, "tripleo-deploy-config-"+instance.Name, instance.Namespace)
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = shared.ConfigGeneratorCondReasonCMCreateError
		cond.Type = shared.ConfigGeneratorCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return nil, err
	}

	return tripleoDeployCM, nil
}

func (r *OpenStackConfigGeneratorReconciler) getClusterServiceEndpoint(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackConfigGenerator,
	cond *shared.Condition,
	namespace string,
	labelSelector map[string]string,
) (string, error) {

	serviceList, err := common.GetServicesListWithLabel(ctx, r, namespace, labelSelector)
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = shared.CommonCondReasonServiceNotFound
		cond.Type = shared.ConfigGeneratorCondTypeError

		return "", err
	}

	// for now assume there is always only one
	if len(serviceList.Items) > 0 {
		return serviceList.Items[0].Spec.ClusterIP, nil
	}

	cond.Message = fmt.Sprintf("failed to get Cluster ServiceEndpoint - %s", fmt.Sprint(labelSelector))
	cond.Reason = shared.CommonCondReasonServiceNotFound
	cond.Type = shared.ConfigGeneratorCondTypeError

	return "", k8s_errors.NewNotFound(appsv1.Resource("service"), fmt.Sprint(labelSelector))
}

//
// Render VM role nic templates, but only for tripleo roles
//
func (r *OpenStackConfigGeneratorReconciler) createVMRoleNicTemplates(
	instance *ospdirectorv1beta1.OpenStackConfigGenerator,
	ospVersion ospdirectorv1beta1.OSPVersion,
	rolesMap map[string]*openstackconfiggenerator.RoleType,
	clusterServiceIP string,
	cmLabels map[string]string,
) (map[string]string, error) {
	roleNicTemplates := map[string]string{}

	for _, role := range rolesMap {
		// render nic template for VM role which, but only for tripleo roles
		if role.IsVMType && role.IsTripleoRole {
			roleTemplateParameters := make(map[string]interface{})
			roleTemplateParameters["ClusterServiceIP"] = clusterServiceIP
			roleTemplateParameters["Role"] = role

			file := ""
			nicTemplate := ""
			if ospVersion == ospdirectorv1beta1.OSPVersion(ospdirectorv1beta1.TemplateVersion16_2) {
				file = fmt.Sprintf("%s-%s", role.NameLower, openstackconfiggenerator.RenderedNicFileTrain)
				nicTemplate = fmt.Sprintf("/openstackconfiggenerator/config/%s/nic/%s", ospVersion, openstackconfiggenerator.NicTemplateTrain)
			}
			if ospVersion == ospdirectorv1beta1.OSPVersion(ospdirectorv1beta1.TemplateVersion17_0) {
				file = fmt.Sprintf("%s-%s", role.NameLower, openstackconfiggenerator.RenderedNicFileWallaby)
				nicTemplate = fmt.Sprintf("/openstackconfiggenerator/config/%s/nic/%s", ospVersion, openstackconfiggenerator.NicTemplateWallaby)
			}

			r.Log.Info(fmt.Sprintf("NIC template %s added to tripleo-deploy-config CM for role %s. Add a custom %s NIC template to the tarballConfigMap to overwrite.", file, role.Name, file))

			roleTemplate := common.Template{
				Name:         "tripleo-nic-template",
				Namespace:    instance.Namespace,
				Type:         common.TemplateTypeNone,
				InstanceType: instance.Kind,
				AdditionalTemplate: map[string]string{
					file: nicTemplate,
				},
				Labels:        cmLabels,
				ConfigOptions: roleTemplateParameters,
				Version:       ospVersion,
			}

			renderedTemplates, err := common.GetTemplateData(roleTemplate)
			if err != nil {
				return nil, err
			}

			for k, v := range renderedTemplates {
				roleNicTemplates[k] = v
			}
		}
	}

	return roleNicTemplates, nil
}

//
// Verify if ConfigGeneratorInputLabel label is set on the CM which is used to limit
// the CMs to watch
//
func (r *OpenStackConfigGeneratorReconciler) addConfigGeneratorInputLabel(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackConfigGenerator,
	cond *shared.Condition,
	cm *corev1.ConfigMap,
) error {

	labelSelector := map[string]string{
		openstackconfiggenerator.ConfigGeneratorInputLabel: "true",
	}

	if _, ok := cm.Labels[openstackconfiggenerator.ConfigGeneratorInputLabel]; !ok {
		op, err := controllerutil.CreateOrPatch(ctx, r.GetClient(), cm, func() error {
			cm.SetLabels(labels.Merge(cm.GetLabels(), labelSelector))

			return nil
		})
		if err != nil {
			cond.Message = err.Error()
			cond.Reason = shared.ConfigGeneratorCondReasonInputLabelError
			cond.Type = shared.ConfigGeneratorCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}
		common.LogForObject(
			r,
			fmt.Sprintf("%s updated with %s label: %s", cm.Name, openstackconfiggenerator.ConfigGeneratorInputLabel, op),
			instance,
		)
	}

	return nil

}
