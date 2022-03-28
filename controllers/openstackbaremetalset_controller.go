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
	"net"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/diff"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/baremetalset"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstackipset "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackipset"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/provisionserver"
)

// OpenStackBaremetalSetReconciler reconciles a OpenStackBaremetalSet object
type OpenStackBaremetalSetReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackBaremetalSetReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackBaremetalSetReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackBaremetalSetReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackBaremetalSetReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbaremetalsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbaremetalsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbaremetalsets/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackprovisionservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackprovisionservers/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackprovisionservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbackups,verbs=get;list;watch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbackups/status,verbs=get;watch;update;patch
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=create;delete;get;list;patch;update;watch

// Reconcile baremetalset
func (r *OpenStackBaremetalSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstackbaremetalset", req.NamespacedName)

	// Fetch the instance
	instance := &ospdirectorv1beta1.OpenStackBaremetalSet{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile
			// request.  Owned objects are automatically garbage collected.  For
			// additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	//
	// initialize condition
	//
	cond := &ospdirectorv1beta1.Condition{}

	// If BaremetalHosts status map is nil, create it
	if instance.Status.BaremetalHosts == nil {
		instance.Status.BaremetalHosts = map[string]ospdirectorv1beta1.HostStatus{}
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

	defer func(cond *ospdirectorv1beta1.Condition) {
		//
		// Update object conditions
		//
		instance.Status.Conditions.UpdateCurrentCondition(
			cond.Type,
			cond.Reason,
			cond.Message,
		)

		instance.Status.ProvisioningStatus.Reason = cond.Message
		instance.Status.ProvisioningStatus.State = ospdirectorv1beta1.ProvisioningState(cond.Type)

		if statusChanged() {
			if updateErr := r.Status().Update(context.Background(), instance); updateErr != nil {
				common.LogErrorForObject(r, updateErr, "Update status", instance)
			}
		}

		// log current status message to operator log
		common.LogForObject(r, cond.Message, instance)
	}(cond)

	// examine DeletionTimestamp to determine if object is under deletion
	finalizerName := "baremetalset.osp-director.openstack.org-" + instance.Name
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, finalizerName) {
			controllerutil.AddFinalizer(instance, finalizerName)
			if err := r.Update(ctx, instance); err != nil {
				return reconcile.Result{}, err
			}
			common.LogForObject(
				r,
				fmt.Sprintf("Finalizer %s added to CR %s", finalizerName, instance.Name),
				instance,
			)
		}
	} else {
		// 1. check if finalizer is there
		// Reconcile if finalizer got already removed
		if !controllerutil.ContainsFinalizer(instance, finalizerName) {
			return ctrl.Result{}, nil
		}

		// 2. Clean up resources used by the operator
		// BareMetalHost resources in the openshift-machine-api namespace (don't delete, just deprovision)
		err := r.baremetalHostCleanup(ctx, instance, cond)
		if err != nil && !k8s_errors.IsNotFound(err) {
			// ignore not found errors if the object is already gone
			return ctrl.Result{}, err
		}

		// 3. cleanup resources created by operator
		// a. Delete objects in non openstack namespace which have the owner reference label
		//    - secret objects in openshift-machine-api namespace
		err = r.deleteOwnerRefLabeledObjects(ctx, instance, cond)
		if err != nil && !k8s_errors.IsNotFound(err) {
			// ignore not found errors if the object is already gone
			return ctrl.Result{}, err
		}

		// 4. as last step remove the finalizer on the operator CR to finish delete
		controllerutil.RemoveFinalizer(instance, finalizerName)
		err = r.Update(ctx, instance)
		if err != nil {
			cond.Message = fmt.Sprintf("Failed to update %s %s", instance.Kind, instance.Name)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonRemoveFinalizerError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)

			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}
		common.LogForObject(
			r,
			fmt.Sprintf("CR %s deleted", instance.Name),
			instance,
		)
		return ctrl.Result{}, nil
	}

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := common.OpenStackBackupOverridesReconcile(r.Client, instance)

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		common.LogForObject(
			r,
			fmt.Sprintf("OpenStackBaremetalSet %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name),
			instance,
		)

		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	var ctrlResult reconcile.Result
	currentLabels := instance.DeepCopy().Labels

	//
	// Only kept for running local
	// add osnetcfg CR label reference which is used in the in the osnetcfg
	// controller to watch this resource and reconcile
	//
	if _, ok := currentLabels[ospdirectorv1beta1.OpenStackNetConfigReconcileLabel]; !ok {
		common.LogForObject(r, "osnetcfg reference label not added by webhook, adding it!", instance)
		instance.Labels, err = ospdirectorv1beta1.AddOSNetConfigRefLabel(
			instance.Namespace,
			instance.Spec.Networks[0],
			currentLabels,
		)
		if err != nil {
			return ctrlResult, err
		}
	}

	//
	// add labels of all networks used by this CR
	//
	instance.Labels = ospdirectorv1beta1.AddOSNetNameLowerLabels(r.GetLogger(), instance.Labels, instance.Spec.Networks)

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
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonAddOSNetLabelError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)

			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}
	}

	//
	// get Password Secret if defined
	//
	var passwordSecret *corev1.Secret
	if instance.Spec.PasswordSecret != "" {
		passwordSecret, ctrlResult, err = r.getPasswordSecret(ctx, instance, cond)
		if (err != nil) || (ctrlResult != ctrl.Result{}) {
			return ctrlResult, err
		}
	}

	//
	// get, create or wait for provision server
	//
	provisionServer, ctrlResult, err := r.provisionServerCreateOrUpdate(ctx, instance, cond)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// check for DeploymentSSHSecret and get pub key from DeploymentSSHSecret
	//
	sshSecret, ctrlResult, err := common.GetDataFromSecret(
		ctx,
		r,
		instance,
		cond,
		ospdirectorv1beta1.ConditionDetails{
			ConditionNotFoundType:   ospdirectorv1beta1.CommonCondTypeWaiting,
			ConditionNotFoundReason: ospdirectorv1beta1.CommonCondReasonDeploymentSecretMissing,
			ConditionErrorType:      ospdirectorv1beta1.CommonCondTypeError,
			ConditionErrordReason:   ospdirectorv1beta1.CommonCondReasonDeploymentSecretError,
		},
		instance.Spec.DeploymentSSHSecret,
		20,
		"authorized_keys",
	)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// check/update instance status for annotated for deletion marged BMHs
	//
	deletionAnnotatedBMHs, err := r.checkBMHsAnnotatedForDeletion(
		ctx,
		instance,
		cond,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// Handle BMHM removal from BMSet
	//
	deletedHosts, err := r.doBMHDelete(
		ctx,
		instance,
		cond,
		deletionAnnotatedBMHs,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// create openstackclient IPs for all networks
	//
	ipsetStatus, ctrlResult, err := openstackipset.EnsureIPs(
		ctx,
		r,
		instance,
		cond,
		instance.Spec.RoleName,
		instance.Spec.Networks,
		instance.Spec.Count,
		false,
		false,
		deletedHosts,
		true,
	)

	for _, status := range ipsetStatus {
		hostStatus := openstackipset.SyncIPsetStatus(cond, instance.Status.BaremetalHosts, status)
		instance.Status.BaremetalHosts[status.Hostname] = hostStatus
	}

	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	//   Get domain name and dns servers from controlplane spec
	//
	controlPlane, ctrlResult, err := common.GetControlPlane(ctx, r, instance)
	if err != nil {
		return ctrlResult, err
	}

	//
	//   Provision / deprovision requested replicas
	//
	if err := r.ensureBaremetalHosts(
		ctx,
		instance,
		cond,
		&controlPlane,
		provisionServer,
		sshSecret,
		passwordSecret,
	); err != nil {
		return ctrl.Result{}, err
	}

	// Calculate overall provisioning status
	readyCount := 0
	bmhErrors := 0

	for _, bmh := range instance.Status.BaremetalHosts {
		if strings.EqualFold(string(bmh.ProvisioningState), string(ospdirectorv1beta1.BaremetalSetCondTypeProvisioned)) {
			readyCount++
		} else if strings.EqualFold(string(bmh.ProvisioningState), string(ospdirectorv1beta1.BaremetalSetCondTypeError)) {
			bmhErrors++
		}
	}

	instance.Status.ProvisioningStatus.ReadyCount = readyCount

	if bmhErrors > 0 {
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeError)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalSetCondReasonProvisioningErrors)
		cond.Message = fmt.Sprintf("%d BaremetalHost(s) encountered an error", bmhErrors)
	} else {
		switch readyCount := instance.Status.ProvisioningStatus.ReadyCount; {
		case readyCount == instance.Spec.Count && readyCount == 0:
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeEmpty)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalSetCondReasonVirtualMachineCountZero)
			cond.Message = "No BaremetalHost have been requested"
		case readyCount == instance.Spec.Count:
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeProvisioned)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalSetCondReasonVirtualMachineProvisioned)
			cond.Message = "All requested BaremetalHosts have been provisioned"
		case readyCount < instance.Spec.Count:
			// Only set this if readyCount is less than spec Count, and this reconciliation did not previously
			// encounter the "ospdirectorv1beta1.BaremetalSetInsufficient" state, then provisioning is in progress
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeProvisioning)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalSetCondReasonVirtualMachineProvisioning)
			cond.Message = "Provisioning of BaremetalHosts in progress"
		default:
			// Only set this if readyCount is less than spec Count, and this reconciliation did not previously
			// encounter the "ospdirectorv1beta1.BaremetalSetInsufficient" state, then deprovisioning is in progress
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeDeprovisioning)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalSetCondReasonVirtualMachineDeprovisioning)
			cond.Message = "Deprovisioning of BaremetalHosts in progress"
		}
	}

	return ctrl.Result{}, nil
}

func (r *OpenStackBaremetalSetReconciler) getNormalizedStatus(status *ospdirectorv1beta1.OpenStackBaremetalSetStatus) *ospdirectorv1beta1.OpenStackBaremetalSetStatus {

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

// SetupWithManager - prepare controller for use with operator manager
func (r *OpenStackBaremetalSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	openshiftMachineAPIBareMetalHostsFn := handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		result := []reconcile.Request{}
		label := o.GetLabels()
		// verify object has ownerUIDLabelSelector
		if uid, ok := label[common.OwnerUIDLabelSelector]; ok {
			common.LogForObject(
				r,
				fmt.Sprintf("BareMetalHost object %s marked with OSP owner ref: %s", o.GetName(), uid),
				o,
			)

			// return namespace and Name of CR
			name := client.ObjectKey{
				Namespace: label[common.OwnerNameSpaceLabelSelector],
				Name:      label[common.OwnerNameLabelSelector],
			}
			result = append(result, reconcile.Request{NamespacedName: name})
		}
		if len(result) > 0 {
			return result
		}
		return nil
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackBaremetalSet{}).
		Owns(&ospdirectorv1beta1.OpenStackProvisionServer{}).
		Owns(&ospdirectorv1beta1.OpenStackIPSet{}).
		Watches(&source.Kind{Type: &metal3v1alpha1.BareMetalHost{}}, openshiftMachineAPIBareMetalHostsFn).
		Complete(r)
}

func (r *OpenStackBaremetalSetReconciler) provisionServerCreateOrUpdate(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *ospdirectorv1beta1.Condition,
) (*ospdirectorv1beta1.OpenStackProvisionServer, reconcile.Result, error) {
	provisionServer := &ospdirectorv1beta1.OpenStackProvisionServer{}

	// NOTE: webook validates that either ProvisionServerName or baseImageUrl is set
	if instance.Spec.ProvisionServerName == "" {
		// Next deploy the provisioning image (Apache) server
		provisionServer = &ospdirectorv1beta1.OpenStackProvisionServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.ObjectMeta.Name + "-provisionserver",
				Namespace: instance.ObjectMeta.Namespace,
			},
		}

		op, err := controllerutil.CreateOrPatch(ctx, r.Client, provisionServer, func() error {
			// Assign the prov server its existing port if this is an update, otherwise pick a new one
			// based on what is available
			err := provisionServer.AssignProvisionServerPort(
				cond,
				r.Client,
				provisionserver.DefaultPort,
			)
			if err != nil {
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err
			}

			provisionServer.Spec.BaseImageURL = instance.Spec.BaseImageURL

			err = controllerutil.SetControllerReference(instance, provisionServer, r.Scheme)
			if err != nil {
				cond.Message = fmt.Sprintf("Error set controller reference for %s %s", provisionServer.Kind, provisionServer.Name)
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonControllerReferenceError)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err
			}

			return nil
		})

		if err != nil {
			cond.Message = fmt.Sprintf("Failed to create or update %s %s ", provisionServer.Kind, provisionServer.Name)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OpenStackProvisionServerCondReasonCreateError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return provisionServer, ctrl.Result{}, err
		}

		cond.Message = fmt.Sprintf("%s %s %s %s CR successfully reconciled",
			instance.Kind,
			instance.Name,
			provisionServer.Kind,
			provisionServer.Name,
		)

		if op != controllerutil.OperationResultNone {
			cond.Message = fmt.Sprintf("%s - operation: %s", cond.Message, string(op))
		}
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OpenStackProvisionServerCondReasonCreated)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeCreated)

	} else {
		err := r.Get(ctx, types.NamespacedName{Name: instance.Spec.ProvisionServerName, Namespace: instance.Namespace}, provisionServer)
		if err != nil && k8s_errors.IsNotFound(err) {
			timeout := 10

			cond.Message = fmt.Sprintf("%s %s not found reconcile again in %d seconds", provisionServer.Kind, instance.Spec.ProvisionServerName, timeout)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OpenStackProvisionServerCondReasonNotFound)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeWaiting)

			return provisionServer, ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, nil
		} else if err != nil {
			cond.Message = fmt.Sprintf("Error getting %s %s", provisionServer.Kind, instance.Spec.ProvisionServerName)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ProvisionServerCondTypeError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return provisionServer, ctrl.Result{}, err
		}
	}

	if provisionServer.Status.LocalImageURL == "" {
		timeout := 30
		cond.Message = fmt.Sprintf("%s %s %s local image URL not yet available, requeuing and waiting %d seconds",
			instance.Kind,
			instance.Name,
			provisionServer.Kind,
			timeout,
		)

		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OpenStackProvisionServerCondReasonProvisioning)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeWaiting)

		return provisionServer, ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, nil
	}

	cond.Message = fmt.Sprintf("%s %s %s local image URL %s available",
		instance.Kind,
		instance.Name,
		provisionServer.Kind,
		provisionServer.Status.LocalImageURL,
	)

	cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalSetCondTypeProvisioning)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.OpenStackProvisionServerCondReasonProvisioned)

	return provisionServer, ctrl.Result{}, nil
}

// Deprovision BaremetalHost resources based on replica count
func (r *OpenStackBaremetalSetReconciler) doBMHDelete(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *ospdirectorv1beta1.Condition,
	removalAnnotatedBaremetalHosts []string,
) ([]string, error) {
	deletedHosts := []string{}

	// Get all openshift-machine-api BaremetalHosts
	baremetalHostsList, err := common.GetBmhHosts(
		ctx,
		r,
		"openshift-machine-api",
		map[string]string{
			common.OwnerControllerNameLabelSelector: baremetalset.AppLabel,
			common.OwnerUIDLabelSelector:            string(instance.GetUID()),
		},
	)
	if err != nil {
		cond.Message = "Failed to get list of all BareMetalHost(s)"
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalHostCondReasonListError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeError)

		return deletedHosts, err
	}

	// Deallocate existing BaremetalHosts to match the requested replica count, if necessary.  First we
	// choose BaremetalHosts with the "osp-director.openstack.org/delete-host=true"
	// annotation.  Then, if there are still BaremetalHosts left to deprovision based on the requested
	// replica count, we will log a warning indicating that we cannot (fully or partially) honor the
	// scale-down.

	// How many new BaremetalHost de-allocations do we need (if any)?
	bmhsToRemoveCount := len(baremetalHostsList.Items) - instance.Spec.Count

	if bmhsToRemoveCount > 0 {
		bmhsRemovedCount := 0

		for i := 0; i < bmhsToRemoveCount; i++ {
			// First choose BaremetalHosts to remove from the prepared list of BaremetalHosts
			// that have the "osp-director.openstack.org/delete-host=true" annotation

			if len(removalAnnotatedBaremetalHosts) > 0 {
				// get OSBms status corresponding to bmh
				bmhStatus, err := r.getBmhHostRefStatus(instance, cond, removalAnnotatedBaremetalHosts[0])
				if err != nil && k8s_errors.IsNotFound(err) {
					return deletedHosts, err
				}

				deletedHost, err := r.baremetalHostDeprovision(
					ctx,
					instance,
					cond,
					bmhStatus,
				)
				if err != nil {
					return deletedHosts, err
				}

				deletedHosts = append(deletedHosts, deletedHost)

				// Remove the removal-annotated BaremetalHost from the removalAnnotatedBaremetalHosts list
				if len(removalAnnotatedBaremetalHosts) > 1 {
					removalAnnotatedBaremetalHosts = removalAnnotatedBaremetalHosts[1:]
				} else {
					removalAnnotatedBaremetalHosts = []string{}
				}

				// We removed a removal-annotated BaremetalHost, so increment the removed count
				bmhsRemovedCount++
			} else {
				// Just break, as any further iterations of the loop have nothing upon
				// which to operate (we'll report this as a warning just below)
				break
			}
		}

		// If we can't satisfy the requested scale-down, explicitly state so
		if bmhsRemovedCount < bmhsToRemoveCount {
			cond.Message = fmt.Sprintf("Unable to find sufficient amount of BaremetalHost replicas annotated for scale-down (%d found and removed, %d requested)", bmhsRemovedCount, bmhsToRemoveCount)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalSetCondReasonScaleDownInsufficientAnnotatedHosts)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeInsufficient)

			common.LogForObject(
				r,
				cond.Message,
				instance,
			)
		}
	}

	sort.Strings(deletedHosts)

	return deletedHosts, nil
}

// Provision BaremetalHost resources based on replica count
func (r *OpenStackBaremetalSetReconciler) ensureBaremetalHosts(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *ospdirectorv1beta1.Condition,
	controlPlane *ospdirectorv1beta1.OpenStackControlPlane,
	provisionServer *ospdirectorv1beta1.OpenStackProvisionServer,
	sshSecret string,
	passwordSecret *corev1.Secret,
) error {

	// Get all openshift-machine-api BaremetalHosts
	baremetalHostsList, err := common.GetBmhHosts(
		ctx,
		r,
		"openshift-machine-api",
		instance.Spec.BmhLabelSelector,
	)
	if err != nil {
		cond.Message = "Failed to get list of all BareMetalHost(s)"
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalHostCondReasonListError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeError)

		return err
	}

	// Get all existing BaremetalHosts of this CR
	existingBaremetalHosts, err := common.GetBmhHosts(
		ctx,
		r,
		"openshift-machine-api",
		map[string]string{
			common.OwnerControllerNameLabelSelector: baremetalset.AppLabel,
			common.OwnerUIDLabelSelector:            string(instance.GetUID()),
		},
	)
	if err != nil {
		cond.Message = "Failed to get list of all existing BareMetalHost(s)"
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalHostCondReasonListError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeError)

		return err
	}

	// How many new BaremetalHost allocations do we need (if any)?
	newBmhsNeededCount := instance.Spec.Count - len(existingBaremetalHosts.Items)

	if newBmhsNeededCount > 0 {
		// We have new replicas requested, so search for baremetalhosts that don't have consumerRef or Online set
		availableBaremetalHosts := []string{}

		for _, baremetalHost := range baremetalHostsList.Items {
			if baremetalHost.Spec.Online || baremetalHost.Spec.ConsumerRef != nil {
				continue
			}

			hardwareMatch, err := r.verifyHardwareMatch(instance, &baremetalHost)
			if err != nil {
				return err
			}

			if !hardwareMatch {
				common.LogForObject(
					r,
					fmt.Sprintf("BaremetalHost %s does not match hardware requirements for %s %s",
						baremetalHost.ObjectMeta.Name,
						instance.Kind,
						instance.Name),
					instance)

				continue
			}

			common.LogForObject(
				r,
				fmt.Sprintf("Available BaremetalHost: %s", baremetalHost.ObjectMeta.Name),
				instance)

			availableBaremetalHosts = append(availableBaremetalHosts, baremetalHost.ObjectMeta.Name)
		}

		// If we can't satisfy the new requested replica count, explicitly state so
		if newBmhsNeededCount > len(availableBaremetalHosts) {
			cond.Message = fmt.Sprintf("Unable to find %d requested BaremetalHost count (%d in use, %d available)",
				instance.Spec.Count,
				len(existingBaremetalHosts.Items),
				len(availableBaremetalHosts))
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalSetCondReasonScaleUpInsufficientHosts)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeInsufficient)

			common.LogForObject(
				r,
				cond.Message,
				instance,
			)
		}

		// Sort the list of available BaremetalHosts
		sort.Strings(availableBaremetalHosts)

		// For each available BaremetalHost that we need to allocate, we update the
		// reference to use our image and set the user data to use our cloud-init secret.
		// Then we add the status to store the BMH name, cloud-init secret name, management
		// IP and BMH power status for the particular worker
		for i := 0; i < len(availableBaremetalHosts) && i < newBmhsNeededCount; i++ {
			err := r.baremetalHostProvision(
				ctx,
				instance,
				cond,
				controlPlane,
				availableBaremetalHosts[i],
				provisionServer.Status.LocalImageURL,
				sshSecret,
				passwordSecret,
			)

			if err != nil {
				return err
			}
		}
	}

	// Now reconcile existing BaremetalHosts for this OpenStackBaremetalSet
	for _, bmh := range existingBaremetalHosts.Items {
		err := r.baremetalHostProvision(
			ctx,
			instance,
			cond,
			controlPlane,
			bmh.GetName(),
			provisionServer.Status.LocalImageURL,
			sshSecret,
			passwordSecret,
		)

		if err != nil {
			return err
		}
	}

	return nil
}

func (r *OpenStackBaremetalSetReconciler) getBmhHostRefStatus(
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *ospdirectorv1beta1.Condition,
	bmh string,
) (ospdirectorv1beta1.HostStatus, error) {

	for _, bmhStatus := range instance.Status.DeepCopy().BaremetalHosts {
		if bmhStatus.HostRef == bmh {
			return bmhStatus, nil
		}
	}

	cond.Message = fmt.Sprintf("OpenStackBaremetalHostStatus for %s not found", bmh)
	cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalSetCondReasonBaremetalHostStatusNotFound)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeError)

	return ospdirectorv1beta1.HostStatus{}, k8s_errors.NewNotFound(corev1.Resource("OpenStackBaremetalHostStatus"), "not found")
}

// Provision a BaremetalHost via Metal3 (and create its bootstrapping secret)
func (r *OpenStackBaremetalSetReconciler) baremetalHostProvision(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *ospdirectorv1beta1.Condition,
	controlPlane *ospdirectorv1beta1.OpenStackControlPlane,
	bmh string,
	localImageURL string,
	sshSecret string,
	passwordSecret *corev1.Secret,
) error {
	// Prepare cloudinit (create secret)
	sts := []common.Template{}
	secretLabels := common.GetLabels(instance, baremetalset.AppLabel, map[string]string{})

	//
	//  get already registerd OSBms bmh status for bmh name
	//
	bmhStatus, err := r.getBmhHostRefStatus(instance, cond, bmh)
	//
	//  if bmhStatus is not found, get free hostname from instance.Status.baremetalHosts (HostRef == ospdirectorv1beta1.HostRefInitState ("unassigned")) for the new bmh
	//
	if err != nil && k8s_errors.IsNotFound(err) {
		for _, bmhStatus = range instance.Status.BaremetalHosts {
			if bmhStatus.HostRef == ospdirectorv1beta1.HostRefInitState {

				bmhStatus.HostRef = bmh
				instance.Status.BaremetalHosts[bmhStatus.Hostname] = bmhStatus

				// update status with host assignment
				if err := r.Status().Update(context.Background(), instance); err != nil {
					cond.Message = fmt.Sprintf("Failed to update CR status %v", err)
					cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonCRStatusUpdateError)
					cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)

					err = common.WrapErrorForObject(cond.Message, instance, err)

					return err
				}

				common.LogForObject(
					r,
					fmt.Sprintf("Assigned %s to baremetalhost %s", bmhStatus.Hostname, bmh),
					instance,
				)

				break
			}
		}
	}

	// User data cloud-init secret
	templateParameters := make(map[string]interface{})
	templateParameters["AuthorizedKeys"] = sshSecret
	templateParameters["Hostname"] = bmhStatus.Hostname

	//
	//   Get Domain and DNSServers from controlplane spec
	//
	domainName := controlPlane.Spec.DomainName
	DNSServers := instance.Spec.BootstrapDNS
	if DNSServers == nil {
		DNSServers = controlPlane.Spec.DNSServers
	}
	DNSSearchDomains := controlPlane.Spec.DNSSearchDomains
	if domainName != "" {
		templateParameters["DomainName"] = domainName
	}

	//
	// use same NodeRootPassword paremater as tripleo have
	//
	if passwordSecret != nil && len(passwordSecret.Data["NodeRootPassword"]) > 0 {
		templateParameters["NodeRootPassword"] = string(passwordSecret.Data["NodeRootPassword"])
	}

	userDataSecretName := fmt.Sprintf(baremetalset.CloudInitUserDataSecretName, instance.Name, bmh)

	userDataSt := common.Template{
		Name:               userDataSecretName,
		Namespace:          "openshift-machine-api",
		Type:               common.TemplateTypeConfig,
		InstanceType:       instance.Kind,
		AdditionalTemplate: map[string]string{"userData": "/baremetalset/cloudinit/userdata"},
		Labels:             secretLabels,
		ConfigOptions:      templateParameters,
	}

	sts = append(sts, userDataSt)
	// TODO mschuppert: get ctlplane network name using ooo-ctlplane-network label
	ipCidr := instance.Status.BaremetalHosts[bmhStatus.Hostname].IPAddresses["ctlplane"]

	ip, network, _ := net.ParseCIDR(ipCidr)
	netMask := network.Mask

	netNameLower := "ctlplane"
	// get network with name_lower label
	labelSelector := map[string]string{
		ospdirectorv1beta1.SubNetNameLabelSelector: netNameLower,
	}

	// get ctlplane network
	ctlPlaneNetwork, err := ospdirectorv1beta1.GetOpenStackNetWithLabel(
		instance.Namespace,
		labelSelector,
	)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			cond.Message = fmt.Sprintf("OpenStackNet with NameLower %s not found!", netNameLower)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonOSNetNotFound)
		} else {
			// Error reading the object - requeue the request.
			cond.Message = fmt.Sprintf("Error getting OSNet with labelSelector %v", labelSelector)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonOSNetError)
		}
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	// Network data cloud-init secret
	templateParameters = make(map[string]interface{})
	templateParameters["CtlplaneIp"] = ip.String()
	templateParameters["CtlplaneInterface"] = instance.Spec.CtlplaneInterface
	templateParameters["CtlplaneGateway"] = ctlPlaneNetwork.Spec.Gateway
	templateParameters["CtlplaneNetmask"] = fmt.Sprintf("%d.%d.%d.%d", netMask[0], netMask[1], netMask[2], netMask[3])
	templateParameters["CtlplaneDns"] = DNSServers
	templateParameters["CtlplaneDnsSearch"] = DNSSearchDomains

	networkDataSecretName := fmt.Sprintf(baremetalset.CloudInitNetworkDataSecretName, instance.Name, bmh)

	// Flag the network data secret as safe to collect with must-gather
	secretLabelsWithMustGather := common.GetLabels(instance, baremetalset.AppLabel, map[string]string{
		common.MustGatherSecret: "yes",
	})

	networkDataSt := common.Template{
		Name:               networkDataSecretName,
		Namespace:          "openshift-machine-api",
		Type:               common.TemplateTypeConfig,
		InstanceType:       instance.Kind,
		AdditionalTemplate: map[string]string{"networkData": "/baremetalset/cloudinit/networkdata"},
		Labels:             secretLabelsWithMustGather,
		ConfigOptions:      templateParameters,
	}

	sts = append(sts, networkDataSt)

	err = common.EnsureSecrets(ctx, r, instance, sts, &map[string]common.EnvSetter{})
	if err != nil {
		cond.Message = "Error creating metal3 cloud-init secrets"
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalHostCondReasonCloudInitSecretError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	//
	// Provision the BaremetalHost
	//
	foundBaremetalHost := &metal3v1alpha1.BareMetalHost{}
	err = r.Get(ctx, types.NamespacedName{Name: bmh, Namespace: "openshift-machine-api"}, foundBaremetalHost)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to get %s %s", foundBaremetalHost.Kind, foundBaremetalHost.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalHostCondReasonGetError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)

		return err
	}

	//
	// create a copy of the foundBaremetalHost to be able to compare after we applied our labels/spec
	//
	actualFoundBaremetalHost := foundBaremetalHost.DeepCopy()

	//
	// Set our ownership labels so we can watch this resource
	// Set ownership labels that can be found by the respective controller kind
	labelSelector = common.GetLabels(instance, baremetalset.AppLabel, map[string]string{
		common.OSPHostnameLabelSelector: bmhStatus.Hostname,
	})

	foundBaremetalHost.GetObjectMeta().SetLabels(labels.Merge(foundBaremetalHost.GetObjectMeta().GetLabels(), labelSelector))

	foundBaremetalHost.Spec.Online = true
	foundBaremetalHost.Spec.ConsumerRef = &corev1.ObjectReference{Name: instance.Name, Kind: instance.Kind, Namespace: instance.Namespace}
	foundBaremetalHost.Spec.Image = &metal3v1alpha1.Image{
		URL:      localImageURL,
		Checksum: fmt.Sprintf("%s.md5sum", localImageURL),
	}
	foundBaremetalHost.Spec.UserData = &corev1.SecretReference{
		Name:      userDataSecretName,
		Namespace: "openshift-machine-api",
	}
	foundBaremetalHost.Spec.NetworkData = &corev1.SecretReference{
		Name:      networkDataSecretName,
		Namespace: "openshift-machine-api",
	}

	//
	// update metal3 object only if labes or spec changed
	//
	if !reflect.DeepEqual(actualFoundBaremetalHost.GetLabels(), foundBaremetalHost.GetLabels()) ||
		!reflect.DeepEqual(actualFoundBaremetalHost.Spec, foundBaremetalHost.Spec) {
		common.LogForObject(
			r,
			fmt.Sprintf("Allocating/Updating BaremetalHost: %s", foundBaremetalHost.Name),
			instance)

		err = r.Update(ctx, foundBaremetalHost)
		if err != nil {
			cond.Message = fmt.Sprintf("Failed to update %s %s", foundBaremetalHost.Kind, foundBaremetalHost.Name)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalHostCondReasonUpdateError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)

			return err
		}
	}

	//
	// Update status with BMH provisioning details
	//
	bmhStatus.UserDataSecretName = userDataSecretName
	bmhStatus.NetworkDataSecretName = networkDataSecretName
	bmhStatus.CtlplaneIP = ipCidr
	bmhStatus.ProvisioningState = ospdirectorv1beta1.ProvisioningState(foundBaremetalHost.Status.Provisioning.State)

	actualBMHStatus := instance.Status.BaremetalHosts[bmhStatus.Hostname]
	if !reflect.DeepEqual(actualBMHStatus, bmhStatus) {
		common.LogForObject(
			r,
			fmt.Sprintf("Updating CR status BMH %s provisioning details - diff %s",
				bmhStatus.Hostname,
				diff.ObjectReflectDiff(actualBMHStatus, bmhStatus)),
			instance)

		instance.Status.BaremetalHosts[bmhStatus.Hostname] = bmhStatus
	}

	return nil

}

// Deprovision a BaremetalHost via Metal3 (and delete its bootstrapping secret)
func (r *OpenStackBaremetalSetReconciler) baremetalHostDeprovision(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *ospdirectorv1beta1.Condition,
	bmh ospdirectorv1beta1.HostStatus,
) (string, error) {
	baremetalHost := &metal3v1alpha1.BareMetalHost{}
	err := r.Get(ctx, types.NamespacedName{Name: bmh.HostRef, Namespace: "openshift-machine-api"}, baremetalHost)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to get %s %s", baremetalHost.Kind, baremetalHost.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalHostCondReasonGetError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)

		return "", err
	}

	common.LogForObject(
		r,
		fmt.Sprintf("Deallocating BaremetalHost: %s", bmh.HostRef),
		instance,
	)

	// Remove our ownership labels
	labels := baremetalHost.GetObjectMeta().GetLabels()
	ospHostname := labels[common.OSPHostnameLabelSelector]
	labelSelector := common.GetLabels(instance, baremetalset.AppLabel, map[string]string{
		common.OSPHostnameLabelSelector: labels[common.OSPHostnameLabelSelector],
	})
	for key := range labelSelector {
		delete(labels, key)
	}
	delete(labels, common.OSPHostnameLabelSelector)
	baremetalHost.GetObjectMeta().SetLabels(labels)

	// Remove deletion annotation (if any)
	annotations := baremetalHost.GetObjectMeta().GetAnnotations()
	delete(annotations, common.HostRemovalAnnotation)
	baremetalHost.GetObjectMeta().SetAnnotations(annotations)

	baremetalHost.Spec.Online = false
	baremetalHost.Spec.ConsumerRef = nil
	baremetalHost.Spec.Image = nil
	baremetalHost.Spec.UserData = nil
	baremetalHost.Spec.NetworkData = nil
	err = r.Update(ctx, baremetalHost)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to update %s %s", baremetalHost.Kind, baremetalHost.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.BaremetalHostCondReasonUpdateError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)

		return ospHostname, err
	}
	r.Log.Info(fmt.Sprintf("BaremetalHost deleted: bmh %s - osp name %s", baremetalHost.Name, ospHostname))

	// Also remove userdata and networkdata secrets
	for _, secret := range []string{
		fmt.Sprintf(baremetalset.CloudInitUserDataSecretName, instance.Name, bmh.HostRef),
		fmt.Sprintf(baremetalset.CloudInitNetworkDataSecretName, instance.Name, bmh.HostRef),
	} {
		err = common.DeleteSecretsWithName(
			ctx,
			r,
			cond,
			secret,
			"openshift-machine-api",
		)
		if err != nil {
			return ospHostname, err
		}

		r.Log.Info(fmt.Sprintf("Network data secret deleted: name %s", secret))
	}

	// Set status (remove this BaremetalHost entry)
	delete(instance.Status.BaremetalHosts, bmh.Hostname)

	return ospHostname, nil
}

// Deprovision all associated BaremetalHosts for this OpenStackBaremetalSet via Metal3
func (r *OpenStackBaremetalSetReconciler) baremetalHostCleanup(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *ospdirectorv1beta1.Condition,
) error {
	if instance.Status.BaremetalHosts != nil {
		for _, bmh := range instance.Status.BaremetalHosts {
			_, err := r.baremetalHostDeprovision(ctx, instance, cond, bmh)

			if err != nil {
				return err
			}
		}
	}

	return nil
}

/* deleteOwnerRefLabeledObjects - cleans up namespaced objects outside the default namespace
   using the owner reference labels added.
   List of objects which get cleaned:
   - user-data secret, openshift-machine-api namespace
*/
func (r *OpenStackBaremetalSetReconciler) deleteOwnerRefLabeledObjects(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *ospdirectorv1beta1.Condition,
) error {

	labelSelectorMap := common.GetLabels(instance, baremetalset.AppLabel, map[string]string{})

	// delete secrets in openshift-machine-api namespace
	secrets, err := common.GetSecrets(ctx, r, "openshift-machine-api", labelSelectorMap)
	if err != nil {
		return err
	}
	for idx := range secrets.Items {
		secret := &secrets.Items[idx]

		err = r.Delete(ctx, secret, &client.DeleteOptions{})
		if err != nil {
			cond.Message = fmt.Sprintf("Error deleting OwnerRefLabeledObjects %s", secret.Name)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonOwnerRefLabeledObjectsDeleteError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}

		common.LogForObject(
			r,
			fmt.Sprintf("Secret deleted: name %s - %s", secret.Name, secret.UID),
			instance,
		)
	}

	return nil
}

func (r *OpenStackBaremetalSetReconciler) verifyHardwareMatch(
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	bmh *metal3v1alpha1.BareMetalHost,
) (bool, error) {
	// If no requested hardware requirements, we're all set
	if instance.Spec.HardwareReqs == (ospdirectorv1beta1.HardwareReqs{}) {
		return true, nil
	}

	// Can't make comparisons if the BMH lacks hardware details
	if bmh.Status.HardwareDetails == nil {
		common.LogForObject(
			r,
			fmt.Sprintf("WARNING: BaremetalHost %s lacks hardware details in status; cannot verify against %s %s hardware requests!",
				bmh.Name,
				instance.Kind,
				instance.Name),
			instance,
		)

		return false, nil
	}

	cpuReqs := instance.Spec.HardwareReqs.CPUReqs

	// CPU architecture is always exact-match only
	if cpuReqs.Arch != "" && bmh.Status.HardwareDetails.CPU.Arch != cpuReqs.Arch {
		common.LogForObject(
			r,
			fmt.Sprintf("BaremetalHost %s CPU arch %s does not match %s %s request for '%s'",
				bmh.Name,
				bmh.Status.HardwareDetails.CPU.Arch,
				instance.Kind,
				instance.Name,
				cpuReqs.Arch),
			instance,
		)

		return false, nil
	}

	// CPU count can be exact-match or (default) greater
	if cpuReqs.CountReq.Count != 0 && bmh.Status.HardwareDetails.CPU.Count != cpuReqs.CountReq.Count {
		if cpuReqs.CountReq.ExactMatch || cpuReqs.CountReq.Count > bmh.Status.HardwareDetails.CPU.Count {
			common.LogForObject(
				r,
				fmt.Sprintf("BaremetalHost %s CPU count %d does not match %s %s request for '%d'",
					bmh.Name,
					bmh.Status.HardwareDetails.CPU.Count,
					instance.Kind,
					instance.Name,
					cpuReqs.CountReq.Count),
				instance,
			)

			return false, nil
		}
	}

	// CPU clock speed can be exact-match or (default) greater
	if cpuReqs.MhzReq.Mhz != 0 {
		clockSpeed := int(bmh.Status.HardwareDetails.CPU.ClockMegahertz)
		if cpuReqs.MhzReq.Mhz != clockSpeed && (cpuReqs.MhzReq.ExactMatch || cpuReqs.MhzReq.Mhz > clockSpeed) {
			common.LogForObject(
				r,
				fmt.Sprintf("BaremetalHost %s CPU mhz %d does not match %s %s request for '%d'",
					bmh.Name,
					clockSpeed,
					instance.Kind,
					instance.Name,
					cpuReqs.MhzReq.Mhz),
				instance,
			)

			return false, nil
		}
	}

	memReqs := instance.Spec.HardwareReqs.MemReqs

	// Memory GBs can be exact-match or (default) greater
	if memReqs.GbReq.Gb != 0 {

		memGbBms := float64(memReqs.GbReq.Gb)
		memGbBmh := float64(bmh.Status.HardwareDetails.RAMMebibytes) / float64(1024)

		if memGbBmh != memGbBms && (memReqs.GbReq.ExactMatch || memGbBms > memGbBmh) {
			common.LogForObject(
				r,
				fmt.Sprintf("BaremetalHost %s memory size %v does not match %s %s request for '%v'",
					bmh.Name,
					memGbBmh,
					instance.Kind,
					instance.Name,
					memGbBms),
				instance,
			)

			return false, nil
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
			common.LogForObject(
				r,
				fmt.Sprintf("BaremetalHost %s does not contain a disk of size %v that matches %s %s request",
					bmh.Name,
					diskGbBms,
					instance.Kind,
					instance.Name),
				instance,
			)

			return false, nil
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
			common.LogForObject(
				r,
				fmt.Sprintf("BaremetalHost %s does not contain a disk with 'rotational' equal to %v that matches %s %s request",
					bmh.Name,
					diskReqs.SSDReq.SSD,
					instance.Kind,
					instance.Name),
				instance,
			)

			return false, nil
		}
	}

	common.LogForObject(
		r,
		fmt.Sprintf("BaremetalHost %s satisfies %s %s hardware requirements",
			bmh.Name,
			instance.Kind,
			instance.Name),
		instance,
	)

	return true, nil
}

func (r *OpenStackBaremetalSetReconciler) getPasswordSecret(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *ospdirectorv1beta1.Condition,
) (*corev1.Secret, reconcile.Result, error) {
	// check if specified password secret exists before creating the computes
	passwordSecret, _, err := common.GetSecret(ctx, r, instance.Spec.PasswordSecret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			timeout := 30
			cond.Message = fmt.Sprintf("PasswordSecret %s not found but specified in CR, next reconcile in %d s", instance.Spec.PasswordSecret, timeout)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonSecretMissing)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.BaremetalSetCondTypeWaiting)

			return passwordSecret, ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, nil
		}
		// Error reading the object - requeue the request.
		cond.Message = fmt.Sprintf("Error getting TripleoPasswordsSecret %s", instance.Spec.PasswordSecret)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ControlPlaneReasonTripleoPasswordsSecretCreateError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return passwordSecret, ctrl.Result{}, err
	}
	r.Log.Info(fmt.Sprintf("PasswordSecret %s exists", instance.Spec.PasswordSecret))

	return passwordSecret, ctrl.Result{}, nil
}

//
//   check/update instance status for annotated for deletion marked BMs
//
func (r *OpenStackBaremetalSetReconciler) checkBMHsAnnotatedForDeletion(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *ospdirectorv1beta1.Condition,
) ([]string, error) {

	// check for deletion marked BMH
	currentBMHostsStatus := instance.Status.DeepCopy().BaremetalHosts
	deletionAnnotatedBMHs, err := common.GetDeletionAnnotatedBmhHosts(
		ctx,
		r,
		"openshift-machine-api",
		map[string]string{
			common.OwnerControllerNameLabelSelector: baremetalset.AppLabel,
			common.OwnerUIDLabelSelector:            string(instance.GetUID()),
		},
	)
	if err != nil {
		return deletionAnnotatedBMHs, err
	}

	for hostname, bmhStatus := range instance.Status.DeepCopy().BaremetalHosts {
		bmhName := bmhStatus.HostRef
		if len(deletionAnnotatedBMHs) > 0 && common.StringInSlice(bmhName, deletionAnnotatedBMHs) {
			// set annotatedForDeletion status of the BMH to true, if not already
			if !bmhStatus.AnnotatedForDeletion {
				bmhStatus.AnnotatedForDeletion = true
				common.LogForObject(
					r,
					fmt.Sprintf("Host deletion annotation set on BMH %s/%s", bmhName, hostname),
					instance,
				)
			}
		} else {
			// check if the BMH was previously flagged as annotated and revert it
			if bmhStatus.AnnotatedForDeletion {
				bmhStatus.AnnotatedForDeletion = false
				common.LogForObject(
					r,
					fmt.Sprintf("Host deletion annotation removed on BMH %s/%s", bmhName, hostname),
					instance,
				)
			}
		}
		actualBMHStatus := instance.Status.BaremetalHosts[hostname]
		if !reflect.DeepEqual(&actualBMHStatus, bmhStatus) {
			instance.Status.BaremetalHosts[hostname] = bmhStatus
		}
	}

	if !reflect.DeepEqual(currentBMHostsStatus, instance.Status.BaremetalHosts) {
		common.LogForObject(
			r,
			fmt.Sprintf("Updating CR status with deletion annotation information - %s",
				diff.ObjectReflectDiff(currentBMHostsStatus, instance.Status.BaremetalHosts)),
			instance,
		)

		err = r.Status().Update(context.Background(), instance)
		if err != nil {
			cond.Message = "Failed to update CR status for annotated for deletion marked VMs"
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonCRStatusUpdateError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return deletionAnnotatedBMHs, err
		}
	}

	return deletionAnnotatedBMHs, nil
}
