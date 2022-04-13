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
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackdeploy"
)

// OpenStackDeployReconciler reconciles a OpenStackDeploy object
type OpenStackDeployReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackDeployReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackDeployReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackDeployReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackDeployReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackdeploys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackdeploys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackdeploys/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;create;update;delete;watch

// Reconcile - OpenStackDeploy
func (r *OpenStackDeployReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Fetch the OpenStackDeploy instance
	instance := &ospdirectorv1beta1.OpenStackDeploy{}
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
		instance.Status.CurrentState = shared.ProvisioningState(cond.Type)
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
	}(cond)

	//
	// Get OSPVersion from OSControlPlane status
	//
	// unified OSPVersion from ControlPlane CR
	// which means also get either 16.2 or 17.0 for upstream versions
	controlPlane, ctrlResult, err := ospdirectorv1beta1.GetControlPlane(r.Client, instance)
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = shared.ControlPlaneReasonNetNotFound
		cond.Type = shared.DeployCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrlResult, err
	}
	OSPVersion, err := shared.GetOSPVersion(string(controlPlane.Status.OSPVersion))
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = shared.ControlPlaneReasonNotSupportedVersion
		cond.Type = shared.DeployCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrlResult, err
	}

	// look up the git secret from the configGenerator
	configGenerator := &ospdirectorv1beta1.OpenStackConfigGenerator{}
	err = r.GetClient().Get(ctx, types.NamespacedName{Name: instance.Spec.ConfigGenerator, Namespace: instance.Namespace}, configGenerator)
	if err != nil && k8s_errors.IsNotFound(err) {
		cond.Message = err.Error()
		cond.Reason = shared.DeployCondReasonConfigVersionNotFound
		cond.Type = shared.DeployCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrl.Result{}, err
	}

	advancedSettings := instance.Spec.AdvancedSettings.DeepCopy()
	if advancedSettings.Playbook == "" {
		switch instance.Spec.Mode {
		case "update":
			advancedSettings.Playbook = "update_steps_playbook.yaml"
		case "externalUpdate":
			advancedSettings.Playbook = "external_update_steps_playbook.yaml"
		default:
			advancedSettings.Playbook = "deploy_steps_playbook.yaml"
		}
	}

	// FIXME: is ConfigVersion set on the status?
	// Should changing ansible settings restart a job
	// Need to make interrupting a job safe
	if instance.Status.ConfigVersion != instance.Spec.ConfigVersion {
		// Define a new Job object
		job := openstackdeploy.DeployJob(
			instance,
			"openstackclient",
			instance.Spec.ConfigVersion,
			configGenerator.Spec.GitSecret,
			advancedSettings,
			OSPVersion,
		)

		op, err := controllerutil.CreateOrPatch(ctx, r.Client, job, func() error {
			err := controllerutil.SetControllerReference(instance, job, r.Scheme)
			if err != nil {

				cond.Message = err.Error()
				cond.Reason = shared.DeployCondReasonJobCreated
				cond.Type = shared.DeployCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err

			}

			return nil
		})
		if err != nil && !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			cond.Message = fmt.Sprintf("Job successfully created/updated - operation: %s", string(op))
			cond.Reason = shared.DeployCondReasonJobCreated
			cond.Type = shared.DeployCondTypeInitializing
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		// configVersion Hash changed while job was running (NOTE: configVersion is the 1st ENV in the job)
		common.LogForObject(r, fmt.Sprintf("Job Hash : %s", job.Spec.Template.Spec.Containers[0].Env[0].Value), instance)
		if instance.Spec.ConfigVersion != job.Spec.Template.Spec.Containers[0].Env[0].Value {
			_, err = common.DeleteJob(ctx, job, r.Kclient, r.Log)
			if err != nil {
				cond.Message = err.Error()
				cond.Reason = shared.DeployCondReasonJobDelete
				cond.Type = shared.DeployCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return ctrl.Result{}, err
			}

			cond.Message = "ConfigVersion has changed. Requeing to start again..."
			cond.Reason = shared.DeployCondReasonCVUpdated
			cond.Type = shared.DeployCondTypeInitializing
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 5}, nil

		}

		requeue, err := common.WaitOnJob(ctx, job, r.Client, r.Log)
		common.LogForObject(r, "Deploying OpenStack...", instance)

		if err != nil {
			// the job failed in error
			cond.Message = "Job failed... Please check job/pod logs."
			cond.Reason = shared.DeployCondReasonJobFailed
			cond.Type = shared.DeployCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		} else if requeue {
			cond.Message = "Waiting on OpenStack Deployment..."
			cond.Reason = shared.DeployCondReasonConfigCreate
			cond.Type = shared.DeployCondTypeRunning
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}
	}

	cond.Message = "The OpenStackDeploy Finished."
	cond.Reason = shared.DeployCondReasonJobFinished
	cond.Type = shared.DeployCondTypeFinished
	common.LogForObject(r, cond.Message, instance)

	// set ownership on the ConfigMap created by our Deployment container
	err = openstackdeploy.SetConfigMapOwner(r, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *OpenStackDeployReconciler) getNormalizedStatus(status *ospdirectorv1beta1.OpenStackDeployStatus) *ospdirectorv1beta1.OpenStackDeployStatus {

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

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackDeployReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackDeploy{}).
		Complete(r)
}
