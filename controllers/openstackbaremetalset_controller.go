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
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/registry/generic/registry"
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
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=create;delete;get;list;patch;update;watch

// Reconcile baremetalset
func (r *OpenStackBaremetalSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstackbaremetalset", req.NamespacedName)

	// Fetch the instance
	instance := &ospdirectorv1beta1.OpenStackBaremetalSet{}
	if err := r.Client.Get(ctx, req.NamespacedName, instance); err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile
			// request.  Owned objects are automatically garbage collected.  For
			// additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	finalizerName := "baremetalset.osp-director.openstack.org-" + instance.Name
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, finalizerName) {
			controllerutil.AddFinalizer(instance, finalizerName)
			if err := r.Update(context.Background(), instance); err != nil {
				return reconcile.Result{}, err
			}
			r.Log.Info(fmt.Sprintf("Finalizer %s added to CR %s", finalizerName, instance.Name))
		}
	} else {
		// 1. check if finalizer is there
		// Reconcile if finalizer got already removed
		if !controllerutil.ContainsFinalizer(instance, finalizerName) {
			return ctrl.Result{}, nil
		}

		// 2. Clean up resources used by the operator
		// BareMetalHost resources in the openshift-machine-api namespace (don't delete, just deprovision)
		err := r.baremetalHostCleanup(instance)
		if err != nil && !k8s_errors.IsNotFound(err) {
			// ignore not found errors if the object is already gone
			return ctrl.Result{}, err
		}

		// 3. cleanup resources created by operator
		// a. Delete objects in non openstack namespace which have the owner reference label
		//    - secret objects in openshift-machine-api namespace
		err = r.deleteOwnerRefLabeledObjects(instance)
		if err != nil && !k8s_errors.IsNotFound(err) {
			// ignore not found errors if the object is already gone
			return ctrl.Result{}, err
		}

		// 4. as last step remove the finalizer on the operator CR to finish delete
		controllerutil.RemoveFinalizer(instance, finalizerName)
		err = r.Client.Update(context.TODO(), instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("CR %s deleted", instance.Name))
		return ctrl.Result{}, nil
	}

	// get a copy if the CR ProvisioningStatus
	actualProvisioningState := instance.Status.DeepCopy().ProvisioningStatus

	var err error
	var passwordSecret *corev1.Secret

	if instance.Spec.PasswordSecret != "" {
		// check if specified password secret exists before creating the computes
		passwordSecret, _, err = common.GetSecret(r, instance.Spec.PasswordSecret, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				timeout := 30
				msg := fmt.Sprintf("PasswordSecret %s not found but specified in CR, next reconcile in %d s", instance.Spec.PasswordSecret, timeout)

				actualProvisioningState.State = ospdirectorv1beta1.BaremetalSetWaiting
				actualProvisioningState.Reason = msg

				err := r.setProvisioningStatus(instance, actualProvisioningState)

				return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
			}
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("PasswordSecret %s exists", instance.Spec.PasswordSecret))
	}

	provisionServer := &ospdirectorv1beta1.OpenStackProvisionServer{}

	// NOTE: webook validates that either ProvisionServerName or baseImageUrl is set
	if instance.Spec.ProvisionServerName == "" {
		// Next deploy the provisioning image (Apache) server
		pv, op, err := r.provisionServerCreateOrUpdate(instance)
		provisionServer = pv

		if err != nil {
			return ctrl.Result{}, err
		}

		if op != controllerutil.OperationResultNone {
			r.Log.Info(fmt.Sprintf("OpenStackBaremetalSet %s OpenStackProvisionServer successfully reconciled - operation: %s", instance.Name, string(op)))
		}

	} else {
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.ProvisionServerName, Namespace: instance.Namespace}, provisionServer)
		if err != nil && k8s_errors.IsNotFound(err) {
			timeout := 10
			msg := fmt.Sprintf("OpenStackProvisionServer %s not found reconcile again in %d seconds", instance.Spec.ProvisionServerName, timeout)

			actualProvisioningState.State = ospdirectorv1beta1.BaremetalSetWaiting
			actualProvisioningState.Reason = msg

			err := r.setProvisioningStatus(instance, actualProvisioningState)

			return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
		} else if err != nil {
			return ctrl.Result{}, err
		}
	}

	if provisionServer.Status.LocalImageURL == "" {
		timeout := 30
		msg := fmt.Sprintf("OpenStackBaremetalSet %s OpenStackProvisionServer local image URL not yet available, requeuing and waiting %d seconds", instance.Name, timeout)

		actualProvisioningState.State = ospdirectorv1beta1.BaremetalSetWaiting
		actualProvisioningState.Reason = msg

		err = r.setProvisioningStatus(instance, actualProvisioningState)

		return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
	}

	// First we need the public SSH key, which should be stored in a secret
	sshSecret, _, err := common.GetSecret(r, instance.Spec.DeploymentSSHSecret, instance.Namespace)
	if err != nil && k8s_errors.IsNotFound(err) {
		timeout := 20
		msg := fmt.Sprintf("DeploymentSSHSecret secret does not exist: %v", err)

		actualProvisioningState.State = ospdirectorv1beta1.BaremetalSetWaiting
		actualProvisioningState.Reason = msg

		err := r.setProvisioningStatus(instance, actualProvisioningState)

		return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// If BaremetalHosts status map is nil, create it
	if instance.Status.BaremetalHosts == nil {
		instance.Status.BaremetalHosts = map[string]ospdirectorv1beta1.OpenStackBaremetalHostStatus{}
	}

	//
	// create hostnames for the requested number of systems
	//
	//   create new host allocations count
	//
	newCount := instance.Spec.Count - len(instance.Status.BaremetalHosts)

	//
	//   create hostnames for the newCount
	//

	currentStatus := instance.Status.DeepCopy()
	for i := 0; i < newCount; i++ {
		hostnameDetails := common.Hostname{
			Basename: instance.Spec.RoleName,
			VIP:      false,
		}

		err := common.CreateOrGetHostname(instance, &hostnameDetails)
		if err != nil {
			return ctrl.Result{}, err
		}

		if _, ok := instance.Status.BaremetalHosts[hostnameDetails.Hostname]; !ok {
			instance.Status.BaremetalHosts[hostnameDetails.Hostname] = ospdirectorv1beta1.OpenStackBaremetalHostStatus{
				Hostname: hostnameDetails.Hostname,
				HostRef:  hostnameDetails.HostRef,
			}
		}
		r.Log.Info(fmt.Sprintf("BaremetalSet hostname created %s", hostnameDetails.Hostname))
	}

	//
	// update status if new hostnames got created
	//
	actualStatus := instance.Status
	if !reflect.DeepEqual(currentStatus.BaremetalHosts, actualStatus.BaremetalHosts) {
		r.Log.Info(fmt.Sprintf("Updating CR status new hostnames got created - diff %s", diff.ObjectReflectDiff(currentStatus.BaremetalHosts, actualStatus.BaremetalHosts)))

		err := r.setStatus(instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	hostnameRefs := instance.GetHostnames()

	//
	//   check/update instance status for annotated for deletion marged BMH
	//

	// check for deletion marked BMH
	currentStatus = instance.Status.DeepCopy()
	deletionAnnotatedBMHs, err := common.GetDeletionAnnotatedBmhHosts(r, "openshift-machine-api", map[string]string{
		common.OwnerControllerNameLabelSelector: baremetalset.AppLabel,
		common.OwnerUIDLabelSelector:            string(instance.GetUID()),
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, bmhStatus := range instance.Status.DeepCopy().BaremetalHosts {
		bmhName := bmhStatus.HostRef
		if len(deletionAnnotatedBMHs) > 0 && common.StringInSlice(bmhName, deletionAnnotatedBMHs) {
			// set annotatedForDeletion status of the BMH to true, if not already
			if !instance.Status.BaremetalHosts[bmhStatus.Hostname].AnnotatedForDeletion {
				bmhStatus.AnnotatedForDeletion = true
				//updateStatus = true
				r.Log.Info(fmt.Sprintf("Host deletion annotation set on BMH %s", bmhName))
			}
		} else {
			// check if the BMH was previously flagged as annotated and revert it
			if bmhStatus.AnnotatedForDeletion {
				bmhStatus.AnnotatedForDeletion = false
				//updateStatus = true
				r.Log.Info(fmt.Sprintf("Host deletion annotation removed on BMH %s", bmhName))
			}
		}
		actualBMHStatus := instance.Status.BaremetalHosts[bmhStatus.Hostname]
		if !reflect.DeepEqual(&actualBMHStatus, bmhStatus) {
			instance.Status.BaremetalHosts[bmhStatus.Hostname] = bmhStatus
		}
	}
	actualStatus = instance.Status
	if !reflect.DeepEqual(currentStatus.BaremetalHosts, actualStatus.BaremetalHosts) {
		r.Log.Info(fmt.Sprintf("Updating CR status annotated for deletion - diff %s", diff.ObjectReflectDiff(currentStatus.BaremetalHosts, actualStatus.BaremetalHosts)))

		err = r.setStatus(instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	//
	//   Create/Update IPSet for the BMSet CR
	//

	ipsetDetails := common.IPSet{
		Networks:            instance.Spec.Networks,
		Role:                instance.Spec.RoleName,
		HostCount:           instance.Spec.Count,
		AddToPredictableIPs: true,
		HostNameRefs:        hostnameRefs,
	}
	ipset, op, err := common.OvercloudipsetCreateOrUpdate(r, instance, ipsetDetails)
	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("IPSet for %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	if len(ipset.Status.HostIPs) < instance.Spec.Count {
		timeout := 10
		msg := fmt.Sprintf("OpenStackIPSet has not yet reached the required count %d", instance.Spec.Count)

		actualProvisioningState.State = ospdirectorv1beta1.BaremetalSetWaiting
		actualProvisioningState.Reason = msg

		err := r.setProvisioningStatus(instance, actualProvisioningState)

		return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
	}

	//
	//   Provision / deprovision requested replicas
	//
	if err := r.ensureBaremetalHosts(instance, provisionServer, sshSecret, passwordSecret, ipset, &actualProvisioningState); err != nil {
		// As we know, this transient "object has been modified, please try again" error occurs from time
		// to time in the reconcile loop.  Let's avoid counting that as an error for the purposes of
		// status display
		if strings.Contains(err.Error(), registry.OptimisticLockErrorMsg) {
			return ctrl.Result{RequeueAfter: time.Second * 1}, nil
		}

		msg := fmt.Sprintf("Error encountered: %v", err)

		instance.Status.ProvisioningStatus.State = ospdirectorv1beta1.BaremetalSetError
		actualProvisioningState.Reason = msg

		// The next line could return an error, but we log it in the "setProvisioningStatus" func anyhow,
		// and we're more interested in returning the error from "ensureBaremetalHosts"
		_ = r.setProvisioningStatus(instance, actualProvisioningState)
		return ctrl.Result{}, err
	}

	// Calculate overall provisioning status
	readyCount := 0
	bmhErrors := 0

	for _, bmh := range instance.Status.BaremetalHosts {
		if strings.EqualFold(bmh.ProvisioningState, string(ospdirectorv1beta1.BaremetalSetProvisioned)) {
			readyCount++
		} else if strings.EqualFold(bmh.ProvisioningState, string(ospdirectorv1beta1.BaremetalSetError)) {
			bmhErrors++
		}
	}

	actualProvisioningState.ReadyCount = readyCount

	if bmhErrors > 0 {
		actualProvisioningState.State = ospdirectorv1beta1.BaremetalSetError
		actualProvisioningState.Reason = fmt.Sprintf("%d BaremetalHost(s) encountered an error", bmhErrors)
	} else if readyCount == instance.Spec.Count {
		if instance.Spec.Count == 0 {
			actualProvisioningState.State = ospdirectorv1beta1.BaremetalSetEmpty
			actualProvisioningState.Reason = "No BaremetalHosts have been requested"
		} else {
			actualProvisioningState.State = ospdirectorv1beta1.BaremetalSetProvisioned
			actualProvisioningState.Reason = "All requested BaremetalHosts have been provisioned"
		}
	} else if actualProvisioningState.State != ospdirectorv1beta1.BaremetalSetInsufficient {
		if readyCount < instance.Spec.Count {
			// Only set this if readyCount is less than spec Count, and this reconciliation did not previously
			// encounter the "ospdirectorv1beta1.BaremetalSetInsufficient" state, then provisioning is in progress
			actualProvisioningState.State = ospdirectorv1beta1.BaremetalSetProvisioning
			actualProvisioningState.Reason = "Provisioning of BaremetalHosts in progress"
		} else {
			// Only set this if readyCount is less than spec Count, and this reconciliation did not previously
			// encounter the "ospdirectorv1beta1.BaremetalSetInsufficient" state, then deprovisioning is in progress
			actualProvisioningState.State = ospdirectorv1beta1.BaremetalSetDeprovisioning
			actualProvisioningState.Reason = "Deprovisioning of BaremetalHosts in progress"
		}
	}

	if err := r.setProvisioningStatus(instance, actualProvisioningState); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *OpenStackBaremetalSetReconciler) setProvisioningStatus(instance *ospdirectorv1beta1.OpenStackBaremetalSet, actualState ospdirectorv1beta1.OpenStackBaremetalSetProvisioningStatus) error {
	// get current ProvisioningStatus
	currentState := instance.Status.ProvisioningStatus

	// if the current ProvisioningStatus is different from the actual, store the update
	// otherwise, just log the status again
	if !reflect.DeepEqual(currentState, actualState) {
		r.Log.Info(fmt.Sprintf("%s - diff %s", actualState.Reason, diff.ObjectReflectDiff(currentState, actualState)))
		instance.Status.ProvisioningStatus = actualState

		instance.Status.Conditions = ospdirectorv1beta1.ConditionList{}
		instance.Status.Conditions.Set(ospdirectorv1beta1.ConditionType(actualState.State), corev1.ConditionTrue, ospdirectorv1beta1.ConditionReason(actualState.Reason), actualState.Reason)

		if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
			r.Log.Error(err, "Failed to update CR status %v")
			return err
		}
	} else if actualState.Reason != "" {
		r.Log.Info(actualState.Reason)
	}

	return nil
}

func (r *OpenStackBaremetalSetReconciler) setStatus(instance *ospdirectorv1beta1.OpenStackBaremetalSet) error {
	if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
		r.Log.Error(err, "Failed to update CR status %v")
		return err
	}

	return nil
}

// SetupWithManager - prepare controller for use with operator manager
func (r *OpenStackBaremetalSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	openshiftMachineAPIBareMetalHostsFn := handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		result := []reconcile.Request{}
		label := o.GetLabels()
		// verify object has ownerUIDLabelSelector
		if uid, ok := label[common.OwnerUIDLabelSelector]; ok {
			r.Log.Info(fmt.Sprintf("BareMetalHost object %s marked with OSP owner ref: %s", o.GetName(), uid))
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

func (r *OpenStackBaremetalSetReconciler) provisionServerCreateOrUpdate(instance *ospdirectorv1beta1.OpenStackBaremetalSet) (*ospdirectorv1beta1.OpenStackProvisionServer, controllerutil.OperationResult, error) {
	provisionServer := &ospdirectorv1beta1.OpenStackProvisionServer{
		ObjectMeta: v1.ObjectMeta{
			Name:      instance.ObjectMeta.Name + "-provisionserver",
			Namespace: instance.ObjectMeta.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, provisionServer, func() error {
		// Assign the prov server its existing port if this is an update, otherwise pick a new one
		// based on what is available
		err := provisionServer.AssignProvisionServerPort(r.Client, provisionserver.DefaultPort)

		if err != nil {
			return err
		}

		provisionServer.Spec.BaseImageURL = instance.Spec.BaseImageURL

		err = controllerutil.SetControllerReference(instance, provisionServer, r.Scheme)

		if err != nil {
			return err
		}

		return nil
	})

	return provisionServer, op, err
}

// Provision or deprovision BaremetalHost resources based on replica count
func (r *OpenStackBaremetalSetReconciler) ensureBaremetalHosts(instance *ospdirectorv1beta1.OpenStackBaremetalSet, provisionServer *ospdirectorv1beta1.OpenStackProvisionServer, sshSecret *corev1.Secret, passwordSecret *corev1.Secret, ipset *ospdirectorv1beta1.OpenStackIPSet, actualProvisioningState *ospdirectorv1beta1.OpenStackBaremetalSetProvisioningStatus) error {
	// Get all openshift-machine-api BaremetalHosts
	baremetalHostsList := &metal3v1alpha1.BareMetalHostList{}
	listOpts := []client.ListOption{
		client.InNamespace("openshift-machine-api"),
	}

	if len(instance.Spec.BmhLabelSelector) > 0 {
		labels := client.MatchingLabels(instance.Spec.BmhLabelSelector)
		listOpts = append(listOpts, labels)
	}

	err := r.Client.List(context.TODO(), baremetalHostsList, listOpts...)
	if err != nil {
		return err
	}

	existingBaremetalHosts := map[string]string{}
	removalAnnotatedBaremetalHosts := []string{}

	// Find the current BaremetalHosts belonging to this OpenStackBaremetalSet
	for _, baremetalHost := range baremetalHostsList.Items {
		if baremetalHost.Spec.ConsumerRef != nil && (baremetalHost.Spec.ConsumerRef.Kind == instance.Kind && baremetalHost.Spec.ConsumerRef.Name == instance.Name && baremetalHost.Spec.ConsumerRef.Namespace == instance.Namespace) {
			r.Log.Info(fmt.Sprintf("Existing BaremetalHost: %s", baremetalHost.ObjectMeta.Name))
			// Storing the existing BaremetalHosts in a map like this just makes selected replica scale-down easier below
			existingBaremetalHosts[baremetalHost.ObjectMeta.Name] = baremetalHost.ObjectMeta.Name

			// Prep for possible replica scale-down below
			if val, ok := baremetalHost.Annotations[common.HostRemovalAnnotation]; ok && (strings.ToLower(val) == "yes" || strings.ToLower(val) == "true") {
				removalAnnotatedBaremetalHosts = append(removalAnnotatedBaremetalHosts, baremetalHost.ObjectMeta.Name)
			}
		}
	}

	// Deallocate existing BaremetalHosts to match the requested replica count, if necessary.  First we
	// choose BaremetalHosts with the "osp-director.openstack.org/delete-host=true"
	// annotation.  Then, if there are still BaremetalHosts left to deprovision based on the requested
	// replica count, we will log a warning indicating that we cannot (fully or partially) honor the
	// scale-down.

	// How many new BaremetalHost de-allocations do we need (if any)?
	oldBmhsToRemoveCount := len(existingBaremetalHosts) - instance.Spec.Count

	if oldBmhsToRemoveCount > 0 {
		oldBmhsRemovedCount := 0

		for i := 0; i < oldBmhsToRemoveCount; i++ {
			// First choose BaremetalHosts to remove from the prepared list of BaremetalHosts
			// that have the "osp-director.openstack.org/delete-host=true" annotation
			var op controllerutil.OperationResult

			if len(removalAnnotatedBaremetalHosts) > 0 {
				// get OSBms status corresponding to bmh
				bmhStatus, err := r.getBmhHostRefStatus(instance, removalAnnotatedBaremetalHosts[0])
				if err != nil && k8s_errors.IsNotFound(err) {
					return err
				}

				err = r.baremetalHostDeprovision(instance, bmhStatus)
				if err != nil {
					return err
				}

				// Remove the removal-annotated BaremetalHost from the existingBaremetalHosts map
				delete(existingBaremetalHosts, removalAnnotatedBaremetalHosts[0])

				// Remove the removal-annotated BaremetalHost from the removalAnnotatedBaremetalHosts list
				if len(removalAnnotatedBaremetalHosts) > 1 {
					removalAnnotatedBaremetalHosts = removalAnnotatedBaremetalHosts[1:]
				} else {
					removalAnnotatedBaremetalHosts = []string{}
				}

				ipsetDetails := common.IPSet{
					Networks:            instance.Spec.Networks,
					Role:                instance.Spec.RoleName,
					HostCount:           instance.Spec.Count,
					AddToPredictableIPs: true,
					HostNameRefs:        instance.GetHostnames(),
				}
				ipset, op, err = common.OvercloudipsetCreateOrUpdate(r, instance, ipsetDetails)
				if err != nil {
					return err
				}

				if op != controllerutil.OperationResultNone {
					r.Log.Info(fmt.Sprintf("OpenStackIPSet for %s successfully reconciled - operation: %s", instance.Name, string(op)))
				}

				// We removed a removal-annotated BaremetalHost, so increment the removed count
				oldBmhsRemovedCount++
			} else {
				// Just break, as any further iterations of the loop have nothing upon
				// which to operate (we'll report this as a warning just below)
				break
			}
		}

		// If we can't satisfy the requested scale-down, explicitly state so
		if oldBmhsRemovedCount < oldBmhsToRemoveCount {
			msg := fmt.Sprintf("Unable to find sufficient amount of BaremetalHost replicas annotated for scale-down (%d found and removed, %d requested)", oldBmhsRemovedCount, oldBmhsToRemoveCount)
			actualProvisioningState.State = ospdirectorv1beta1.BaremetalSetInsufficient
			actualProvisioningState.Reason = msg
			r.Log.Info(msg)
		} else {
			// Set actual provisioning state to empty string (it will be recalculated at the
			// end of the main reconcile loop, if needed)
			actualProvisioningState.State = ""
			actualProvisioningState.Reason = ""
		}
	}

	// How many new BaremetalHost allocations do we need (if any)?
	newBmhsNeededCount := instance.Spec.Count - len(existingBaremetalHosts)

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
				r.Log.Info(fmt.Sprintf("BaremetalHost %s does not match hardware requirements for OpenStackBaremetalSet %s", baremetalHost.ObjectMeta.Name, instance.Name))
				continue
			}

			r.Log.Info(fmt.Sprintf("Available BaremetalHost: %s", baremetalHost.ObjectMeta.Name))
			availableBaremetalHosts = append(availableBaremetalHosts, baremetalHost.ObjectMeta.Name)
		}

		// If we can't satisfy the new requested replica count, explicitly state so
		if newBmhsNeededCount > len(availableBaremetalHosts) {
			msg := fmt.Sprintf("Unable to find %d requested BaremetalHost count (%d in use, %d available)", instance.Spec.Count, len(existingBaremetalHosts), len(availableBaremetalHosts))
			actualProvisioningState.State = ospdirectorv1beta1.BaremetalSetInsufficient
			actualProvisioningState.Reason = msg
			r.Log.Info(msg)
		} else {
			// Set actual provisioning state to empty string (it will be recalculated at the
			// end of the main reconcile loop, if needed)
			actualProvisioningState.State = ""
			actualProvisioningState.Reason = ""
		}

		// Sort the list of available BaremetalHosts
		sort.Strings(availableBaremetalHosts)

		// For each available BaremetalHost that we need to allocate, we update the
		// reference to use our image and set the user data to use our cloud-init secret.
		// Then we add the status to store the BMH name, cloud-init secret name, management
		// IP and BMH power status for the particular worker
		for i := 0; i < len(availableBaremetalHosts) && i < newBmhsNeededCount; i++ {
			err := r.baremetalHostProvision(instance, availableBaremetalHosts[i], provisionServer.Status.LocalImageURL, sshSecret, passwordSecret, ipset)

			if err != nil {
				return err
			}
		}
	}

	// Now reconcile existing BaremetalHosts for this OpenStackBaremetalSet
	for bmhName := range existingBaremetalHosts {
		err := r.baremetalHostProvision(instance, bmhName, provisionServer.Status.LocalImageURL, sshSecret, passwordSecret, ipset)

		if err != nil {
			return err
		}
	}

	return nil
}

func (r *OpenStackBaremetalSetReconciler) getBmhHostRefStatus(instance *ospdirectorv1beta1.OpenStackBaremetalSet, bmh string) (ospdirectorv1beta1.OpenStackBaremetalHostStatus, error) {

	for _, bmhStatus := range instance.Status.DeepCopy().BaremetalHosts {
		if bmhStatus.HostRef == bmh {
			return bmhStatus, nil
		}
	}

	return ospdirectorv1beta1.OpenStackBaremetalHostStatus{}, k8s_errors.NewNotFound(corev1.Resource("OpenStackBaremetalHostStatus"), "not found")
}

// Provision a BaremetalHost via Metal3 (and create its bootstrapping secret)
func (r *OpenStackBaremetalSetReconciler) baremetalHostProvision(instance *ospdirectorv1beta1.OpenStackBaremetalSet, bmh string, localImageURL string, sshSecret *corev1.Secret, passwordSecret *corev1.Secret, ipset *ospdirectorv1beta1.OpenStackIPSet) error {
	// Prepare cloudinit (create secret)
	sts := []common.Template{}
	secretLabels := common.GetLabels(instance, baremetalset.AppLabel, map[string]string{})

	//
	//  get already registerd OSBms bmh status for bmh name
	//
	bmhStatus, err := r.getBmhHostRefStatus(instance, bmh)
	//
	//  if bmhStatus is not found, get free hostname from instance.Status.baremetalHosts (HostRef == "unassigned") for the new bmh
	//
	if err != nil && k8s_errors.IsNotFound(err) {
		for _, bmhStatus = range instance.Status.BaremetalHosts {

			if bmhStatus.HostRef == "unassigned" {

				bmhStatus.HostRef = bmh
				instance.Status.BaremetalHosts[bmhStatus.Hostname] = bmhStatus

				// update status with host assignment
				err := r.setStatus(instance)
				if err != nil {
					return err
				}
				r.Log.Info(fmt.Sprintf("Assigned %s to baremetalhost %s", bmhStatus.Hostname, bmh))

				break
			}
		}
	}

	// User data cloud-init secret
	templateParameters := make(map[string]interface{})
	templateParameters["AuthorizedKeys"] = strings.TrimSuffix(string(sshSecret.Data["authorized_keys"]), "\n")
	templateParameters["Hostname"] = bmhStatus.Hostname

	// use same NodeRootPassword paremater as tripleo have
	if passwordSecret != nil && len(passwordSecret.Data["NodeRootPassword"]) > 0 {
		templateParameters["NodeRootPassword"] = string(passwordSecret.Data["NodeRootPassword"])
	}

	userDataSecretName := fmt.Sprintf(baremetalset.CloudInitUserDataSecretName, instance.Name, bmh)

	userDataSt := common.Template{
		Name:           userDataSecretName,
		Namespace:      "openshift-machine-api",
		Type:           common.TemplateTypeConfig,
		InstanceType:   instance.Kind,
		AdditionalData: map[string]string{"userData": "/baremetalset/cloudinit/userdata"},
		Labels:         secretLabels,
		ConfigOptions:  templateParameters,
	}

	sts = append(sts, userDataSt)
	ipCidr := ipset.Status.HostIPs[bmhStatus.Hostname].IPAddresses["ctlplane"]

	ip, network, _ := net.ParseCIDR(ipCidr)
	netMask := network.Mask

	// Network data cloud-init secret

	templateParameters = make(map[string]interface{})
	templateParameters["CtlplaneIp"] = ip.String()
	templateParameters["CtlplaneInterface"] = instance.Spec.CtlplaneInterface
	templateParameters["CtlplaneGateway"] = ipset.Status.Networks["ctlplane"].Gateway
	templateParameters["CtlplaneNetmask"] = fmt.Sprintf("%d.%d.%d.%d", netMask[0], netMask[1], netMask[2], netMask[3])
	templateParameters["CtlplaneDns"] = instance.Spec.BootstrapDNS

	networkDataSecretName := fmt.Sprintf(baremetalset.CloudInitNetworkDataSecretName, instance.Name, bmh)

	networkDataSt := common.Template{
		Name:           networkDataSecretName,
		Namespace:      "openshift-machine-api",
		Type:           common.TemplateTypeConfig,
		InstanceType:   instance.Kind,
		AdditionalData: map[string]string{"networkData": "/baremetalset/cloudinit/networkdata"},
		Labels:         secretLabels,
		ConfigOptions:  templateParameters,
	}

	sts = append(sts, networkDataSt)

	err = common.EnsureSecrets(r, instance, sts, &map[string]common.EnvSetter{})
	if err != nil {
		return err
	}

	// Provision the BaremetalHost
	foundBaremetalHost := &metal3v1alpha1.BareMetalHost{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: bmh, Namespace: "openshift-machine-api"}, foundBaremetalHost)
	if err != nil {
		return err
	}

	r.Log.Info(fmt.Sprintf("Allocating/Updating BaremetalHost: %s", foundBaremetalHost.Name))

	// Set our ownership labels so we can watch this resource
	// Set ownership labels that can be found by the respective controller kind
	labelSelector := common.GetLabels(instance, baremetalset.AppLabel, map[string]string{
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
	err = r.Client.Update(context.TODO(), foundBaremetalHost)
	if err != nil {
		return err
	}

	// Update status with BMH provisioning details
	bmhStatus.UserDataSecretName = userDataSecretName
	bmhStatus.NetworkDataSecretName = networkDataSecretName
	bmhStatus.CtlplaneIP = ipCidr
	bmhStatus.ProvisioningState = string(foundBaremetalHost.Status.Provisioning.State)

	actualBMHStatus := instance.Status.BaremetalHosts[bmhStatus.Hostname]
	if !reflect.DeepEqual(actualBMHStatus, bmhStatus) {
		r.Log.Info(fmt.Sprintf("Updating CR status BMH provisioning details - diff %s", diff.ObjectReflectDiff(actualBMHStatus, bmhStatus)))

		instance.Status.BaremetalHosts[bmhStatus.Hostname] = bmhStatus

		err := r.setStatus(instance)
		if err != nil {
			return err
		}
	}

	return nil

}

// Deprovision a BaremetalHost via Metal3 (and delete its bootstrapping secret)
func (r *OpenStackBaremetalSetReconciler) baremetalHostDeprovision(instance *ospdirectorv1beta1.OpenStackBaremetalSet, bmh ospdirectorv1beta1.OpenStackBaremetalHostStatus) error {
	baremetalHost := &metal3v1alpha1.BareMetalHost{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: bmh.HostRef, Namespace: "openshift-machine-api"}, baremetalHost)

	if err != nil {
		return err
	}

	r.Log.Info(fmt.Sprintf("Deallocating BaremetalHost: %s", bmh.HostRef))

	// Remove our ownership labels
	labels := baremetalHost.GetObjectMeta().GetLabels()
	labelSelector := common.GetLabels(instance, baremetalset.AppLabel, map[string]string{
		common.OSPHostnameLabelSelector: baremetalHost.GetObjectMeta().GetLabels()[common.OSPHostnameLabelSelector],
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
	err = r.Client.Update(context.TODO(), baremetalHost)
	if err != nil {
		return err
	}

	// Also remove userdata and networkdata secrets
	secret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf(baremetalset.CloudInitUserDataSecretName, instance.Name, bmh.HostRef),
			Namespace: "openshift-machine-api",
		},
	}
	err = r.Client.Delete(context.Background(), secret, &client.DeleteOptions{})
	if err != nil {
		return err
	}
	r.Log.Info(fmt.Sprintf("User data secret deleted: name %s", secret.Name))

	secret = &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf(baremetalset.CloudInitNetworkDataSecretName, instance.Name, bmh.HostRef),
			Namespace: "openshift-machine-api",
		},
	}
	err = r.Client.Delete(context.Background(), secret, &client.DeleteOptions{})
	if err != nil {
		return err
	}
	r.Log.Info(fmt.Sprintf("Network data secret deleted: name %s", secret.Name))

	// Set status (remove this BaremetalHost entry)
	delete(instance.Status.BaremetalHosts, bmh.Hostname)

	return nil
}

// Deprovision all associated BaremetalHosts for this OpenStackBaremetalSet via Metal3
func (r *OpenStackBaremetalSetReconciler) baremetalHostCleanup(instance *ospdirectorv1beta1.OpenStackBaremetalSet) error {
	if instance.Status.BaremetalHosts != nil {
		for _, bmh := range instance.Status.BaremetalHosts {
			err := r.baremetalHostDeprovision(instance, bmh)

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
func (r *OpenStackBaremetalSetReconciler) deleteOwnerRefLabeledObjects(instance *ospdirectorv1beta1.OpenStackBaremetalSet) error {

	labelSelectorMap := common.GetLabels(instance, baremetalset.AppLabel, map[string]string{})

	// delete secrets in openshift-machine-api namespace
	secrets, err := common.GetSecrets(r, "openshift-machine-api", labelSelectorMap)
	if err != nil {
		return err
	}
	for idx := range secrets.Items {
		secret := &secrets.Items[idx]

		err = r.Client.Delete(context.Background(), secret, &client.DeleteOptions{})
		if err != nil {
			return err
		}
		r.Log.Info(fmt.Sprintf("Secret deleted: name %s - %s", secret.Name, secret.UID))
	}

	return nil
}

func (r *OpenStackBaremetalSetReconciler) verifyHardwareMatch(instance *ospdirectorv1beta1.OpenStackBaremetalSet, bmh *metal3v1alpha1.BareMetalHost) (bool, error) {
	// If no requested hardware requirements, we're all set
	if instance.Spec.HardwareReqs == (ospdirectorv1beta1.HardwareReqs{}) {
		return true, nil
	}

	// Can't make comparisons if the BMH lacks hardware details
	if bmh.Status.HardwareDetails == nil {
		r.Log.Info(fmt.Sprintf("WARNING: BaremetalHost %s lacks hardware details in status; cannot verify against OpenStackBaremetalSet %s hardware requests!", bmh.Name, instance.Name))
		return false, nil
	}

	cpuReqs := instance.Spec.HardwareReqs.CPUReqs

	// CPU architecture is always exact-match only
	if cpuReqs.Arch != "" && bmh.Status.HardwareDetails.CPU.Arch != cpuReqs.Arch {
		r.Log.Info(fmt.Sprintf("BaremetalHost %s CPU arch %s does not match OpenStackBaremetalSet %s request for '%s'", bmh.Name, bmh.Status.HardwareDetails.CPU.Arch, instance.Name, cpuReqs.Arch))
		return false, nil
	}

	// CPU count can be exact-match or (default) greater
	if cpuReqs.CountReq.Count != 0 && bmh.Status.HardwareDetails.CPU.Count != cpuReqs.CountReq.Count {
		if cpuReqs.CountReq.ExactMatch || cpuReqs.CountReq.Count > bmh.Status.HardwareDetails.CPU.Count {
			r.Log.Info(fmt.Sprintf("BaremetalHost %s CPU count %d does not match OpenStackBaremetalSet %s request for '%d'", bmh.Name, bmh.Status.HardwareDetails.CPU.Count, instance.Name, cpuReqs.CountReq.Count))
			return false, nil
		}
	}

	// CPU clock speed can be exact-match or (default) greater
	if cpuReqs.MhzReq.Mhz != 0 {
		clockSpeed := int(bmh.Status.HardwareDetails.CPU.ClockMegahertz)
		if cpuReqs.MhzReq.Mhz != clockSpeed && (cpuReqs.MhzReq.ExactMatch || cpuReqs.MhzReq.Mhz > clockSpeed) {
			r.Log.Info(fmt.Sprintf("BaremetalHost %s CPU mhz %d does not match OpenStackBaremetalSet %s request for '%d'", bmh.Name, clockSpeed, instance.Name, cpuReqs.MhzReq.Mhz))
			return false, nil
		}
	}

	memReqs := instance.Spec.HardwareReqs.MemReqs

	// Memory GBs can be exact-match or (default) greater
	if memReqs.GbReq.Gb != 0 {

		memGbBms := float64(memReqs.GbReq.Gb)
		memGbBmh := float64(bmh.Status.HardwareDetails.RAMMebibytes) / float64(1024)

		if memGbBmh != memGbBms && (memReqs.GbReq.ExactMatch || memGbBms > memGbBmh) {
			r.Log.Info(fmt.Sprintf("BaremetalHost %s memory size %v does not match OpenStackBaremetalSet %s request for '%v'", bmh.Name, memGbBmh, instance.Name, memGbBms))
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
			r.Log.Info(fmt.Sprintf("BaremetalHost %s does not contain a disk of size %v that matches OpenStackBaremetalSet %s request", bmh.Name, diskGbBms, instance.Name))
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
			r.Log.Info(fmt.Sprintf("BaremetalHost %s does not contain a disk with 'rotational' equal to %v that matches OpenStackBaremetalSet %s request", bmh.Name, diskReqs.SSDReq.SSD, instance.Name))
			return false, nil
		}
	}

	r.Log.Info(fmt.Sprintf("BaremetalHost %s satisfies OpenStackBaremetalSet %s hardware requirements", bmh.Name, instance.Name))

	return true, nil
}
