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
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

	//
	// initialize condition
	//
	cond := &ospdirectorv1beta1.Condition{}

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

	defer func(cond *ospdirectorv1beta1.Condition) {
		//
		// Update object conditions
		//
		instance.Status.CurrentState = ospdirectorv1beta1.ConfigGeneratorState(cond.Type)
		instance.Status.Conditions.UpdateCurrentCondition(
			cond.Type,
			cond.Reason,
			cond.Message,
		)

		if statusChanged() {
			if updateErr := r.Client.Status().Update(context.Background(), instance); updateErr != nil {
				if err == nil {
					err = common.WrapErrorForObject(
						"Update Status", instance, updateErr)
				} else {
					common.LogErrorForObject(r, updateErr, "Update status", instance)
				}
			}
		}

	}(cond)

	envVars := make(map[string]common.EnvSetter)
	templateParameters := make(map[string]interface{})
	cmLabels := common.GetLabels(instance, openstackconfiggenerator.AppLabel, map[string]string{})

	//
	// Get OSPVersion from OSControlPlane status
	//
	// unified OSPVersion from ControlPlane CR
	// which means also get either 16.2 or 17.0 for upstream versions
	controlPlane, ctrlResult, err := common.GetControlPlane(r, instance)
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ControlPlaneReasonNetNotFound)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrlResult, err
	}
	OSPVersion, err := ospdirectorv1beta1.GetOSPVersion(string(controlPlane.Status.OSPVersion))
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ControlPlaneReasonNotSupportedVersion)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrlResult, err
	}

	templateParameters["OSPVersion"] = OSPVersion

	//
	// check if heat-env-config (customizations provided by administrator) exist if it does not exist, requeue
	//
	tripleoCustomDeployCM, _, err := common.GetConfigMapAndHashWithName(r, instance.Spec.HeatEnvConfigMap, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			cond.Message = fmt.Sprintf("The ConfigMap %s doesn't yet exist. Requeing...", instance.Spec.HeatEnvConfigMap)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonCMNotFound)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorWaiting)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	tripleoCustomDeployFiles := tripleoCustomDeployCM.Data
	templateParameters["TripleoCustomDeployFiles"] = tripleoCustomDeployFiles

	//
	// Read the tripleo-deploy-config CM
	//
	tripleoDeployCM, _, err := common.GetConfigMapAndHashWithName(r, "tripleo-deploy-config", instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			cond.Message = "The tripleo-deploy-config map doesn't yet exist. Requeing..."
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonCMNotFound)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorWaiting)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	tripleoDeployFiles := tripleoDeployCM.Data

	//
	// Delete network_data.yaml from tripleoDeployFiles as it is not an ooo parameter env file
	//
	delete(tripleoDeployFiles, "network_data.yaml")

	//
	// Delete all role nic templates from tripleoDeployFiles as it is not an ooo parameter env file
	//
	for k := range tripleoDeployFiles {
		if strings.HasSuffix(k, "nic-template.yaml") || strings.HasSuffix(k, "nic-template.j2") {
			delete(tripleoDeployFiles, k)
		}
	}

	templateParameters["TripleoDeployFiles"] = tripleoDeployFiles
	templateParameters["HeatServiceName"] = "heat-" + instance.Name

	//
	// Tarball configmap - extra stuff if custom tarballs are provided
	//
	var tripleoTarballCM *corev1.ConfigMap
	if instance.Spec.TarballConfigMap != "" {
		tripleoTarballCM, _, err = common.GetConfigMapAndHashWithName(r, instance.Spec.TarballConfigMap, instance.Namespace)
		if err != nil {
			if errors.IsNotFound(err) {
				cond.Message = "Tarball config map not found, requeuing and waiting. Requeing..."
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonCMNotFound)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorWaiting)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}
		tripleoTarballFiles := tripleoTarballCM.BinaryData
		templateParameters["TripleoTarballFiles"] = tripleoTarballFiles
	}

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
	err = common.EnsureConfigMaps(r, instance, cms, &envVars)
	if err != nil {
		cond.Message = "Tarball config map not found, requeuing and waiting. Requeing..."
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonCMNotFound)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorWaiting)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrl.Result{}, err
	}

	//
	// Fencing considerations
	//
	fencingCM, ctrlResult, err := r.createFencingEnvironmentFiles(
		instance,
		cond,
		envVars,
		&controlPlane,
		tripleoTarballCM,
		cmLabels,
	)
	if (err != nil) || (ctrlResult != reconcile.Result{}) {
		return ctrlResult, err
	}

	//
	// Calc config map hash
	//
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
		cond.Message = "Error calculating configmap hash"
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonCMHashError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
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

	if instance.Status.ConfigHash != configMapHash {
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
			cond.Message = fmt.Sprintf("OpenStackEphemeralHeat successfully created/updated - operation: %s", string(op))
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonEphemeralHeatUpdated)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorInitializing)
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		if !heat.Status.Active {
			cond.Message = "Waiting on Ephemeral Heat instance to launch..."
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonEphemeralHeatLaunch)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorInitializing)
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		// configMap Hash changed after Ephemeral Heat was created
		if heat.Spec.ConfigHash != configMapHash {
			err = r.Client.Delete(context.TODO(), heat)
			if err != nil && !k8s_errors.IsNotFound(err) {
				cond.Message = err.Error()
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonCMHashChanged)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return ctrl.Result{}, err
			}

			cond.Message = "ConfigMap has changed. Requeing to start again..."
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonCMUpdated)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorInitializing)
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		// check if osvmset and osbms is in status provisioned
		msg, deployed, err := r.verifyNodeResourceStatus(instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !deployed {
			cond.Message = msg
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonWaitingNodesProvisioned)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorInitializing)
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 20}, err
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
			cond.Message = fmt.Sprintf("Job successfully created/updated - operation: %s", string(op))
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonJobCreated)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorInitializing)
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		// configMap Hash changed while job was running (NOTE: configHash is only ENV in the job)
		common.LogForObject(r, fmt.Sprintf("Job Hash : %s", job.Spec.Template.Spec.Containers[0].Env[0].Value), instance)
		common.LogForObject(r, fmt.Sprintf("ConfigMap Hash : %s", configMapHash), instance)

		if configMapHash != job.Spec.Template.Spec.Containers[0].Env[0].Value {
			_, err = common.DeleteJob(job, r.Kclient, r.Log)
			if err != nil {
				cond.Message = err.Error()
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonJobDelete)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return ctrl.Result{}, err
			}

			// in this case delete heat too as the database may have been used
			r.Log.Info("Deleting Ephemeral Heat...")
			err = r.Client.Delete(context.TODO(), heat)
			if err != nil && !k8s_errors.IsNotFound(err) {
				cond.Message = err.Error()
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonEphemeralHeatDelete)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return ctrl.Result{}, err
			}

			cond.Message = "ConfigMap has changed. Requeing to start again..."
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonCMUpdated)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorInitializing)
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 5}, nil

		}

		requeue, err := common.WaitOnJob(job, r.Client, r.Log)
		common.LogForObject(r, "Generating Configs...", instance)

		if err != nil {
			// the job failed in error
			cond.Message = "Job failed... Please check job/pod logs."
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonJobFailed)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		} else if requeue {
			cond.Message = "Waiting on Config Generation..."
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonConfigCreate)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorGenerating)
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
	}

	r.setConfigHash(instance, configMapHash)

	_, err = common.DeleteJob(job, r.Kclient, r.Log)
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonJobDelete)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrl.Result{}, err
	}

	// cleanup the ephemeral Heat
	err = r.Client.Delete(context.TODO(), heat)
	if err != nil && !k8s_errors.IsNotFound(err) {
		cond.Message = err.Error()
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonEphemeralHeatDelete)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

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

	cond.Message = "The OpenStackConfigGenerator job has completed"
	cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonJobFinished)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorFinished)

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

func (r *OpenStackConfigGeneratorReconciler) syncConfigVersions(instance *ospdirectorv1beta1.OpenStackConfigGenerator, configVersions map[string]ospdirectorv1beta1.OpenStackConfigVersion) error {

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
func (r *OpenStackConfigGeneratorReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// watch for objects in the same namespace as the controller CR
	namespacedFn := handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// get all CRs from the same namespace (there should only be one)
		crs := &ospdirectorv1beta1.OpenStackConfigGeneratorList{}
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
		For(&ospdirectorv1beta1.OpenStackConfigGenerator{}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, namespacedFn).
		Owns(&corev1.ConfigMap{}).
		Owns(&ospdirectorv1beta1.OpenStackEphemeralHeat{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *OpenStackConfigGeneratorReconciler) verifyNodeResourceStatus(instance *ospdirectorv1beta1.OpenStackConfigGenerator) (string, bool, error) {

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

//
// Fencing considerations
//
func (r *OpenStackConfigGeneratorReconciler) createFencingEnvironmentFiles(
	instance *ospdirectorv1beta1.OpenStackConfigGenerator,
	cond *ospdirectorv1beta1.Condition,
	envVars map[string]common.EnvSetter,
	controlPlane *ospdirectorv1beta1.OpenStackControlPlane,
	tripleoTarballCM *corev1.ConfigMap,
	cmLabels map[string]string,

) (*corev1.ConfigMap, ctrl.Result, error) {
	templateParameters := make(map[string]interface{})

	//
	//  default to fencing disabled
	//
	templateParameters["EnableFencing"] = false

	if controlPlane.Status.ProvisioningStatus.State != ospdirectorv1beta1.ControlPlaneProvisioned {
		cond.Message = fmt.Sprintf("Control plane %s VMs are not yet provisioned. Requeing...", controlPlane.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonCMNotFound)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorWaiting)
		common.LogForObject(
			r,
			cond.Message,
			instance,
		)

		return nil, ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if controlPlane.Spec.EnableFencing {
		fencingRoles := common.GetFencingRoles()

		// First check if custom roles were included that require fencing
		if tripleoTarballCM != nil {
			customFencingRoles, err := common.GetCustomFencingRoles(tripleoTarballCM.BinaryData)
			if err != nil {
				cond.Message = err.Error()
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonCustomRolesNotFound)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return nil, ctrl.Result{}, err
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
					cond.Message = err.Error()
					cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonVMInstanceList)
					cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
					err = common.WrapErrorForObject(cond.Message, instance, err)

					return nil, ctrl.Result{}, err
				}

				virtualMachineInstanceLists = append(virtualMachineInstanceLists, virtualMachineInstanceList)
			}
		}

		if len(virtualMachineInstanceLists) > 0 {
			templateParameters["EnableFencing"] = true

			fencingTemplateParameters, err := controlplane.CreateFencingConfigMapParams(virtualMachineInstanceLists)
			if err != nil {
				cond.Message = err.Error()
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonFencingTemplateError)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return nil, ctrl.Result{}, err
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

	err := common.EnsureConfigMaps(r, instance, cm, &envVars)
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonCMCreateError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return nil, ctrl.Result{}, err
	}

	fencingCM, _, err := common.GetConfigMapAndHashWithName(r, "fencing-config", instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// This should really never happen, but putting the check here anyhow
			cond.Message = "The fencing-config map doesn't yet exist. Requeing..."
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ConfigGeneratorCondReasonCMNotFound)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ConfigGeneratorError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return fencingCM, ctrl.Result{RequeueAfter: time.Second * 10}, err
		}
		return nil, ctrl.Result{}, err
	}

	return fencingCM, ctrl.Result{}, nil
}
