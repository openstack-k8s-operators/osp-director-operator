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
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
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
	cond := &shared.Condition{}

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
			cond.Reason = shared.CommonCondReasonRemoveFinalizerError
			cond.Type = shared.CommonCondTypeError

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
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, instance.IsReady())

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
	if _, ok := currentLabels[shared.OpenStackNetConfigReconcileLabel]; !ok {
		common.LogForObject(r, "osnetcfg reference label not added by webhook, adding it!", instance)
		instance.Labels, err = ospdirectorv1beta1.AddOSNetConfigRefLabel(
			r.Client,
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
			cond.Reason = shared.CommonCondReasonAddOSNetLabelError
			cond.Type = shared.CommonCondTypeError

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
		shared.ConditionDetails{
			ConditionNotFoundType:   shared.CommonCondTypeWaiting,
			ConditionNotFoundReason: shared.CommonCondReasonDeploymentSecretMissing,
			ConditionErrorType:      shared.CommonCondTypeError,
			ConditionErrordReason:   shared.CommonCondReasonDeploymentSecretError,
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
	// Handle BMH removal from BMSet
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
		hostStatus := ospdirectorv1beta1.SyncIPsetStatus(cond, instance.Status.BaremetalHosts, status)
		instance.Status.BaremetalHosts[status.Hostname] = hostStatus
	}

	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	//   Get domain name and dns servers from osNetCfg
	//
	osNetCfg, err := ospdirectorv1beta1.GetOsNetCfg(r.GetClient(), instance.GetNamespace(), instance.GetLabels()[shared.OpenStackNetConfigReconcileLabel])
	if err != nil {
		cond.Type = shared.CommonCondTypeError
		cond.Reason = shared.NetConfigCondReasonError
		cond.Message = fmt.Sprintf("error getting OpenStackNetConfig %s: %s",
			instance.GetLabels()[shared.OpenStackNetConfigReconcileLabel],
			err)

		return ctrl.Result{}, err
	}

	//
	//   Provision requested replicas
	//
	if err := r.ensureBaremetalHosts(
		ctx,
		instance,
		cond,
		osNetCfg,
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
		if strings.EqualFold(string(bmh.ProvisioningState), string(shared.BaremetalSetCondTypeProvisioned)) {
			readyCount++
		} else if strings.EqualFold(string(bmh.ProvisioningState), string(shared.BaremetalSetCondTypeError)) {
			bmhErrors++
		}
	}

	instance.Status.ProvisioningStatus.ReadyCount = readyCount

	if bmhErrors > 0 {
		cond.Type = shared.BaremetalSetCondTypeError
		cond.Reason = shared.BaremetalSetCondReasonProvisioningErrors
		cond.Message = fmt.Sprintf("%d BaremetalHost(s) encountered an error", bmhErrors)
	} else {
		switch readyCount := instance.Status.ProvisioningStatus.ReadyCount; {
		case readyCount == instance.Spec.Count && readyCount == 0:
			cond.Type = shared.BaremetalSetCondTypeEmpty
			cond.Reason = shared.BaremetalSetCondReasonVirtualMachineCountZero
			cond.Message = "No BaremetalHost have been requested"
		case readyCount == instance.Spec.Count:
			cond.Type = shared.BaremetalSetCondTypeProvisioned
			cond.Reason = shared.BaremetalSetCondReasonVirtualMachineProvisioned
			cond.Message = "All requested BaremetalHosts have been provisioned"
		case readyCount < instance.Spec.Count:
			// Only set this if readyCount is less than spec Count, and this reconciliation did not previously
			// encounter the "ospdirectorv1beta1.BaremetalSetInsufficient" state, then provisioning is in progress
			cond.Type = shared.BaremetalSetCondTypeProvisioning
			cond.Reason = shared.BaremetalSetCondReasonVirtualMachineProvisioning
			cond.Message = "Provisioning of BaremetalHosts in progress"
		default:
			// Only set this if readyCount is less than spec Count, and this reconciliation did not previously
			// encounter the "ospdirectorv1beta1.BaremetalSetInsufficient" state, then deprovisioning is in progress
			cond.Type = shared.BaremetalSetCondTypeDeprovisioning
			cond.Reason = shared.BaremetalSetCondReasonVirtualMachineDeprovisioning
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
	cond *shared.Condition,
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
				cond.Reason = shared.CommonCondReasonControllerReferenceError
				cond.Type = shared.CommonCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err
			}

			return nil
		})

		if err != nil {
			cond.Message = fmt.Sprintf("Failed to create or update %s %s ", provisionServer.Kind, provisionServer.Name)
			cond.Reason = shared.OpenStackProvisionServerCondReasonCreateError
			cond.Type = shared.CommonCondTypeError
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
		cond.Reason = shared.OpenStackProvisionServerCondReasonCreated
		cond.Type = shared.CommonCondTypeCreated

	} else {
		err := r.Get(ctx, types.NamespacedName{Name: instance.Spec.ProvisionServerName, Namespace: instance.Namespace}, provisionServer)
		if err != nil && k8s_errors.IsNotFound(err) {
			timeout := 10

			cond.Message = fmt.Sprintf("%s %s not found reconcile again in %d seconds", provisionServer.Kind, instance.Spec.ProvisionServerName, timeout)
			cond.Reason = shared.OpenStackProvisionServerCondReasonNotFound
			cond.Type = shared.CommonCondTypeWaiting

			return provisionServer, ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, nil
		} else if err != nil {
			cond.Message = fmt.Sprintf("Error getting %s %s", provisionServer.Kind, instance.Spec.ProvisionServerName)
			cond.Reason = shared.OpenStackProvisionServerCondReasonGetError
			cond.Type = shared.CommonCondTypeError
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

		cond.Reason = shared.OpenStackProvisionServerCondReasonProvisioning
		cond.Type = shared.CommonCondTypeWaiting

		return provisionServer, ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, nil
	}

	cond.Message = fmt.Sprintf("%s %s %s local image URL %s available",
		instance.Kind,
		instance.Name,
		provisionServer.Kind,
		provisionServer.Status.LocalImageURL,
	)

	cond.Reason = shared.OpenStackProvisionServerCondReasonProvisioned
	cond.Type = shared.ConditionType(shared.BaremetalSetCondTypeProvisioned)

	return provisionServer, ctrl.Result{}, nil
}

// Deprovision BaremetalHost resources based on replica count
func (r *OpenStackBaremetalSetReconciler) doBMHDelete(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *shared.Condition,
	removalAnnotatedBaremetalHosts []string,
) ([]string, error) {
	deletedHosts := []string{}

	// Get all openshift-machine-api BaremetalHosts
	baremetalHostsList, err := ospdirectorv1beta1.GetBmhHosts(
		ctx,
		r.GetClient(),
		"openshift-machine-api",
		map[string]string{
			common.OwnerControllerNameLabelSelector: shared.OpenStackBaremetalSetAppLabel,
			common.OwnerUIDLabelSelector:            string(instance.GetUID()),
		},
	)
	if err != nil {
		cond.Message = "Failed to get list of all BareMetalHost(s)"
		cond.Reason = shared.BaremetalHostCondReasonListError
		cond.Type = shared.BaremetalSetCondTypeError

		return deletedHosts, err
	}

	// Deallocate existing BaremetalHosts to match the requested replica count, if necessary.  First we
	// verify that we have enough annotated-for-removal BMHs to satisfy the scale-down.  Then we scale-down
	// BaremetalHosts with those "osp-director.openstack.org/delete-host=true" annotations.

	err = ospdirectorv1beta1.VerifyBaremetalSetScaleDown(r.GetLogger(), instance, baremetalHostsList, len(removalAnnotatedBaremetalHosts))

	if err != nil {
		cond.Message = err.Error()
		cond.Reason = shared.BaremetalSetCondReasonScaleDownInsufficientAnnotatedHosts
		cond.Type = shared.BaremetalSetCondTypeInsufficient

		common.LogForObject(
			r,
			cond.Message,
			instance,
		)

		return deletedHosts, err
	}

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
	}

	sort.Strings(deletedHosts)

	return deletedHosts, nil
}

// Provision BaremetalHost resources based on replica count
func (r *OpenStackBaremetalSetReconciler) ensureBaremetalHosts(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *shared.Condition,
	osNetCfg *ospdirectorv1beta1.OpenStackNetConfig,
	provisionServer *ospdirectorv1beta1.OpenStackProvisionServer,
	sshSecret string,
	passwordSecret *corev1.Secret,
) error {

	// Get all openshift-machine-api BaremetalHosts
	baremetalHostsList, err := ospdirectorv1beta1.GetBmhHosts(
		ctx,
		r.GetClient(),
		"openshift-machine-api",
		instance.Spec.BmhLabelSelector,
	)
	if err != nil {
		cond.Message = "Failed to get list of all BareMetalHost(s)"
		cond.Reason = shared.BaremetalHostCondReasonListError
		cond.Type = shared.BaremetalSetCondTypeError

		return err
	}

	// Get all existing BaremetalHosts of this CR
	existingBaremetalHosts, err := ospdirectorv1beta1.GetBmhHosts(
		ctx,
		r.GetClient(),
		"openshift-machine-api",
		map[string]string{
			common.OwnerControllerNameLabelSelector: shared.OpenStackBaremetalSetAppLabel,
			common.OwnerUIDLabelSelector:            string(instance.GetUID()),
		},
	)
	if err != nil {
		cond.Message = "Failed to get list of all existing BareMetalHost(s)"
		cond.Reason = shared.BaremetalHostCondReasonListError
		cond.Type = shared.BaremetalSetCondTypeError

		return err
	}

	// Verify that we have enough hosts with the right hardware reqs available for scaling-up
	availableBaremetalHosts, err := ospdirectorv1beta1.VerifyBaremetalSetScaleUp(r.GetLogger(), instance, baremetalHostsList, existingBaremetalHosts)

	if err != nil {
		cond.Message = err.Error()
		cond.Reason = shared.BaremetalSetCondReasonScaleUpInsufficientHosts
		cond.Type = shared.BaremetalSetCondTypeInsufficient

		common.LogForObject(
			r,
			cond.Message,
			instance,
		)

		return err
	}

	// How many new BaremetalHost allocations do we need (if any)?
	newBmhsNeededCount := instance.Spec.Count - len(existingBaremetalHosts.Items)

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
			osNetCfg,
			availableBaremetalHosts[i],
			provisionServer.Status.LocalImageURL,
			sshSecret,
			passwordSecret,
		)

		if err != nil {
			return err
		}
	}

	// Now reconcile existing BaremetalHosts for this OpenStackBaremetalSet
	for _, bmh := range existingBaremetalHosts.Items {
		err := r.baremetalHostProvision(
			ctx,
			instance,
			cond,
			osNetCfg,
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
	cond *shared.Condition,
	bmh string,
) (ospdirectorv1beta1.HostStatus, error) {

	for _, bmhStatus := range instance.Status.DeepCopy().BaremetalHosts {
		if bmhStatus.HostRef == bmh {
			return bmhStatus, nil
		}
	}

	cond.Message = fmt.Sprintf("OpenStackBaremetalHostStatus for %s not found", bmh)
	cond.Reason = shared.BaremetalSetCondReasonBaremetalHostStatusNotFound
	cond.Type = shared.BaremetalSetCondTypeError

	return ospdirectorv1beta1.HostStatus{}, k8s_errors.NewNotFound(corev1.Resource("OpenStackBaremetalHostStatus"), "not found")
}

// Provision a BaremetalHost via Metal3 (and create its bootstrapping secret)
func (r *OpenStackBaremetalSetReconciler) baremetalHostProvision(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *shared.Condition,
	osNetCfg *ospdirectorv1beta1.OpenStackNetConfig,
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
			if bmhStatus.HostRef == shared.HostRefInitState {

				bmhStatus.HostRef = bmh
				instance.Status.BaremetalHosts[bmhStatus.Hostname] = bmhStatus

				// update status with host assignment
				if err := r.Status().Update(context.Background(), instance); err != nil {
					cond.Message = fmt.Sprintf("Failed to update CR status %v", err)
					cond.Reason = shared.CommonCondReasonCRStatusUpdateError
					cond.Type = shared.CommonCondTypeError

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
	templateParameters["DomainName"] = osNetCfg.Spec.DomainName

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
		shared.SubNetNameLabelSelector: netNameLower,
	}

	// get ctlplane network
	ctlPlaneNetwork, err := ospdirectorv1beta1.GetOpenStackNetWithLabel(
		r.Client,
		instance.Namespace,
		labelSelector,
	)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			cond.Message = fmt.Sprintf("OpenStackNet with NameLower %s not found!", netNameLower)
			cond.Reason = shared.CommonCondReasonOSNetNotFound
		} else {
			// Error reading the object - requeue the request.
			cond.Message = fmt.Sprintf("Error getting OSNet with labelSelector %v", labelSelector)
			cond.Reason = shared.CommonCondReasonOSNetError
		}
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	// Network data cloud-init secret
	templateParameters = make(map[string]interface{})
	templateParameters["CtlplaneIp"] = ip.String()
	templateParameters["CtlplaneInterface"] = instance.Spec.CtlplaneInterface
	templateParameters["CtlplaneGateway"] = ctlPlaneNetwork.Spec.Gateway
	templateParameters["CtlplaneNetmask"] = fmt.Sprintf("%d.%d.%d.%d", netMask[0], netMask[1], netMask[2], netMask[3])
	if len(instance.Spec.BootstrapDNS) > 0 {
		templateParameters["CtlplaneDns"] = instance.Spec.BootstrapDNS
	} else {
		templateParameters["CtlplaneDns"] = osNetCfg.Spec.DNSServers
	}

	if len(instance.Spec.DNSSearchDomains) > 0 {
		templateParameters["CtlplaneDnsSearch"] = instance.Spec.DNSSearchDomains
	} else {
		templateParameters["CtlplaneDnsSearch"] = osNetCfg.Spec.DNSSearchDomains
	}

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
		cond.Reason = shared.BaremetalHostCondReasonCloudInitSecretError
		cond.Type = shared.CommonCondTypeError
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
		cond.Reason = shared.BaremetalHostCondReasonGetError
		cond.Type = shared.CommonCondTypeError

		return err
	}

	op, err := controllerutil.CreateOrPatch(ctx, r.Client, foundBaremetalHost, func() error {
		//
		// Set our ownership labels so we can watch this resource
		// Set ownership labels that can be found by the respective controller kind
		labelSelector = common.GetLabels(instance, baremetalset.AppLabel, map[string]string{
			common.OSPHostnameLabelSelector: bmhStatus.Hostname,
		})
		foundBaremetalHost.Labels = shared.MergeStringMaps(
			foundBaremetalHost.GetLabels(),
			labelSelector,
		)

		//
		// Update the BMH spec once when ConsumerRef is nil to only perform one time provision.
		//
		if foundBaremetalHost.Spec.ConsumerRef == nil {
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
		}

		return nil
	})
	if err != nil {
		cond.Message = fmt.Sprintf("Error update %s BMH %s", foundBaremetalHost.Kind, foundBaremetalHost.Name)
		cond.Reason = shared.BaremetalHostCondReasonUpdateError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)
		common.LogForObject(r, fmt.Sprintf("%s: %+v", cond.Message, foundBaremetalHost), foundBaremetalHost)

		return err
	}

	if op != controllerutil.OperationResultNone {
		common.LogForObject(
			r,
			fmt.Sprintf("BaremetalHost %s successfully reconciled - operation: %s", foundBaremetalHost.Name, string(op)),
			instance,
		)
	}

	//
	// Update status with BMH provisioning details
	//
	bmhStatus.UserDataSecretName = userDataSecretName
	bmhStatus.NetworkDataSecretName = networkDataSecretName
	bmhStatus.CtlplaneIP = ipCidr
	bmhStatus.ProvisioningState = shared.ProvisioningState(foundBaremetalHost.Status.Provisioning.State)

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
	cond *shared.Condition,
	bmh ospdirectorv1beta1.HostStatus,
) (string, error) {
	baremetalHost := &metal3v1alpha1.BareMetalHost{}
	err := r.Get(ctx, types.NamespacedName{Name: bmh.HostRef, Namespace: "openshift-machine-api"}, baremetalHost)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to get %s %s", baremetalHost.Kind, baremetalHost.Name)
		cond.Reason = shared.BaremetalHostCondReasonGetError
		cond.Type = shared.CommonCondTypeError

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
	delete(annotations, shared.HostRemovalAnnotation)
	baremetalHost.GetObjectMeta().SetAnnotations(annotations)

	baremetalHost.Spec.Online = false
	baremetalHost.Spec.ConsumerRef = nil
	baremetalHost.Spec.Image = nil
	baremetalHost.Spec.UserData = nil
	baremetalHost.Spec.NetworkData = nil
	err = r.Update(ctx, baremetalHost)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to update %s %s", baremetalHost.Kind, baremetalHost.Name)
		cond.Reason = shared.BaremetalHostCondReasonUpdateError
		cond.Type = shared.CommonCondTypeError

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
	cond *shared.Condition,
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
	cond *shared.Condition,
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
			cond.Reason = shared.CommonCondReasonOwnerRefLabeledObjectsDeleteError
			cond.Type = shared.CommonCondTypeError
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

func (r *OpenStackBaremetalSetReconciler) getPasswordSecret(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBaremetalSet,
	cond *shared.Condition,
) (*corev1.Secret, reconcile.Result, error) {
	// check if specified password secret exists before creating the computes
	passwordSecret, _, err := common.GetSecret(ctx, r, instance.Spec.PasswordSecret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			timeout := 30
			cond.Message = fmt.Sprintf("PasswordSecret %s not found but specified in CR, next reconcile in %d s", instance.Spec.PasswordSecret, timeout)
			cond.Reason = shared.CommonCondReasonSecretMissing
			cond.Type = shared.ConditionType(shared.BaremetalSetCondTypeWaiting)

			return passwordSecret, ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, nil
		}
		// Error reading the object - requeue the request.
		cond.Message = fmt.Sprintf("Error getting TripleoPasswordsSecret %s", instance.Spec.PasswordSecret)
		cond.Reason = shared.ControlPlaneReasonTripleoPasswordsSecretCreateError
		cond.Type = shared.ConditionType(shared.CommonCondTypeError)
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
	cond *shared.Condition,
) ([]string, error) {

	// check for deletion marked BMH
	currentBMHostsStatus := instance.Status.DeepCopy().BaremetalHosts
	deletionAnnotatedBMHs, err := ospdirectorv1beta1.GetDeletionAnnotatedBmhHosts(
		ctx,
		r.GetClient(),
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
			cond.Message = "Failed to update CR status for annotated for deletion marked BMHs"
			cond.Reason = shared.CommonCondReasonCRStatusUpdateError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return deletionAnnotatedBMHs, err
		}
	}

	return deletionAnnotatedBMHs, nil
}
