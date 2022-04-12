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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackbackup"
)

// OpenStackBackupRequestReconciler reconciles a OpenStackBackupRequest object
type OpenStackBackupRequestReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackBackupRequestReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackBackupRequestReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackBackupRequestReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackBackupRequestReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbackuprequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbackuprequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbackuprequests/finalizers,verbs=update
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbackups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbackups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbaremetalsets,verbs=get;list;watch;update;delete;deletecollection
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackbaremetalsets/status,verbs=get
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients,verbs=get;list;watch;update;delete;deletecollection
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients/status,verbs=get
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackconfiggenerators,verbs=deletecollection
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackcontrolplanes,verbs=get;list;watch;update;delete;deletecollection
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackcontrolplanes/status,verbs=get
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackmacaddresses,verbs=get;list;watch;update;delete;deletecollection
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackmacaddresses/status,verbs=get
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets,verbs=get;list;watch;update;delete;deletecollection
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets/status,verbs=get
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknetattachments,verbs=get;list;watch;update;delete;deletecollection
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknetattachments/status,verbs=get
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknetconfigs,verbs=get;list;watch;update;delete;deletecollection
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknetconfigs/status,verbs=get
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackprovisionservers,verbs=get;list;watch;update;delete;deletecollection
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackprovisionservers/status,verbs=get
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackvmsets,verbs=get;list;watch;update;delete;deletecollection
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackvmsets/status,verbs=get
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackplaybookgenerators,verbs=delete;deletecollection
//+kubebuilder:rbac:groups=core,namespace=openstack,resources=secrets;configmaps,verbs=create;delete;get;list;patch;update;watch

// Reconcile -
func (r *OpenStackBackupRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstackbackuprequest", req.NamespacedName)

	// Fetch the instance
	instance := &ospdirectorv1beta1.OpenStackBackupRequest{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile
			// request.  Owned objects are automatically garbage collected.  For
			// additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// get a copy of the current CR status
	oldStatus := instance.Status.DeepCopy()

	if instance.Spec.Mode == ospdirectorv1beta1.BackupSave &&
		(instance.Status.CurrentState != ospdirectorv1beta1.BackupSaveError &&
			instance.Status.CurrentState != ospdirectorv1beta1.BackupSaved) {
		if err := r.saveBackup(ctx, instance, oldStatus); err != nil && !k8s_errors.IsConflict(err) {
			instance.Status.CurrentState = ospdirectorv1beta1.BackupSaveError
			_ = r.setStatus(ctx, instance, oldStatus, err.Error())
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
	} else if (instance.Spec.Mode == ospdirectorv1beta1.BackupRestore ||
		instance.Spec.Mode == ospdirectorv1beta1.BackupCleanRestore) &&
		(instance.Status.CurrentState != ospdirectorv1beta1.BackupRestoreError &&
			instance.Status.CurrentState != ospdirectorv1beta1.BackupRestored) {
		if err := r.restoreBackup(ctx, instance, oldStatus); err != nil && !k8s_errors.IsConflict(err) {
			instance.Status.CurrentState = ospdirectorv1beta1.BackupRestoreError
			_ = r.setStatus(ctx, instance, oldStatus, err.Error())
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackBackupRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackBackupRequest{}).
		Owns(&ospdirectorv1beta1.OpenStackBackup{}).
		Owns(&ospdirectorv1beta1.OpenStackBaremetalSet{}).
		Owns(&ospdirectorv1beta1.OpenStackClient{}).
		Owns(&ospdirectorv1beta1.OpenStackControlPlane{}).
		Owns(&ospdirectorv1beta1.OpenStackMACAddress{}).
		Owns(&ospdirectorv1beta1.OpenStackNet{}).
		Owns(&ospdirectorv1beta1.OpenStackNetAttachment{}).
		Owns(&ospdirectorv1beta1.OpenStackNetConfig{}).
		Owns(&ospdirectorv1beta1.OpenStackProvisionServer{}).
		Owns(&ospdirectorv1beta1.OpenStackVMSet{}).
		Complete(r)
}

func (r *OpenStackBackupRequestReconciler) setStatus(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBackupRequest,
	oldStatus *ospdirectorv1beta1.OpenStackBackupRequestStatus,
	msg string,
) error {

	if msg != "" {
		r.Log.Info(msg)
	}

	if !reflect.DeepEqual(instance.Status, oldStatus) {
		instance.Status.Conditions.UpdateCurrentCondition(shared.ConditionType(instance.Status.CurrentState), shared.ConditionReason(msg), msg)
		if err := r.Status().Update(ctx, instance); err != nil {
			r.Log.Error(err, "OpenStackBackupRequest update status error: %v")
			return err
		}
	}
	return nil
}

func (r *OpenStackBackupRequestReconciler) saveBackup(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBackupRequest,
	oldStatus *ospdirectorv1beta1.OpenStackBackupRequestStatus,
) error {
	// We always need all the OSP-D operator CRs if we get here, regardless of whether we are
	// actually saving or just quiescing for the moment
	crLists, err := openstackbackup.GetCRLists(ctx, r, instance.Namespace)

	if err != nil {
		return err
	}

	if instance.Status.CurrentState == "" {
		// If no state is set yet, then we are just beginning.
		// Set the state to quiescing to indicate to other controllers that they should finish
		// provisioning any CRs that haven't already completed.  This will also prevent their
		// associated webhooks from allowing certain actions.
		instance.Status.CurrentState = ospdirectorv1beta1.BackupQuiescing

		if err := r.setStatus(ctx, instance, oldStatus, "OpenStackBackupRequest is waiting for other controllers to quiesce"); err != nil {
			return err
		}
	} else if instance.Status.CurrentState == ospdirectorv1beta1.BackupQuiescing {
		// If we aren't saving yet, then we are trying to quiesce all other controllers.
		// Get all OSP-D operator CRs for all CRDs in the namespace and check all of their
		// statuses.  If all CRs have all reached their respective "finished" states, then
		// we are ready to save.

		if quiesced, holdUps := openstackbackup.GetAreControllersQuiesced(instance, crLists); !quiesced {
			holdUpsSb := strings.Builder{}

			for index, holdUp := range holdUps {
				holdUpsSb.WriteString(holdUp.GetObjectKind().GroupVersionKind().Kind)
				holdUpsSb.WriteString(": ")
				holdUpsSb.WriteString(holdUp.GetName())

				if index != len(holdUps)-1 {
					holdUpsSb.WriteString(", ")
				}
			}

			r.Log.Info(fmt.Sprintf("Quiesce for save for OpenStackBackupRequest %s is waiting for: [%s]", instance.Name, holdUpsSb.String()))
		} else {
			// We're ready to save the actual backup, so place all CRs, CMs, Secrets, etc into the spec
			// of an OpenStackBackup instance

			// Get config maps and secrets we want to save
			cmList, err := openstackbackup.GetConfigMapList(ctx, r, instance, &crLists)

			if err != nil {
				return err
			}

			secretList, err := openstackbackup.GetSecretList(ctx, r, instance, &crLists)

			if err != nil {
				return err
			}

			backup := &ospdirectorv1beta1.OpenStackBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%d", instance.Name, time.Now().Unix()),
					Namespace: instance.Namespace,
				},
			}

			op, err := controllerutil.CreateOrPatch(ctx, r.Client, backup, func() error {
				backup.Spec.Crs = crLists
				backup.Spec.ConfigMaps = cmList
				backup.Spec.Secrets = secretList

				return nil
			})
			if err != nil {
				instance.Status.CurrentState = ospdirectorv1beta1.BackupSaveError

				_ = r.setStatus(ctx, instance, oldStatus, fmt.Sprintf("OpenStackBackup %s save failed: %s", instance.Name, err))
				return err
			}
			if op != controllerutil.OperationResultNone {
				r.Log.Info(fmt.Sprintf("OpenStackBackup CR successfully reconciled - operation: %s", string(op)))

				instance.Status.CompletionTimestamp = metav1.Now()
				instance.Status.CurrentState = ospdirectorv1beta1.BackupSaved
				if err := r.setStatus(ctx, instance, oldStatus, fmt.Sprintf("OpenStackBackup %s has been saved", instance.Name)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (r *OpenStackBackupRequestReconciler) restoreBackup(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBackupRequest,
	oldStatus *ospdirectorv1beta1.OpenStackBackupRequestStatus,
) error {
	// Get the backup we are restoring
	backup := &ospdirectorv1beta1.OpenStackBackup{}

	if err := r.Get(ctx, types.NamespacedName{Name: instance.Spec.RestoreSource, Namespace: instance.Namespace}, backup); err != nil {
		return err
	}

	if instance.Status.CurrentState == "" {
		// If no state is set yet, then we are just beginning
		action := "loading"

		if instance.Spec.Mode == ospdirectorv1beta1.BackupRestore {
			// If mode is "restore", set the state to loading to indicate to other controllers that they should pause all
			// reconcile activity
			instance.Status.CurrentState = ospdirectorv1beta1.BackupLoading
		} else if instance.Spec.Mode == ospdirectorv1beta1.BackupCleanRestore {
			// If mode is "cleanRestore", set the state to cleaning to indicate to other controllers' webhooks that they should
			// not allow any new resources to be created (we allow reconciles to continue so that deletes of CRs issued by this
			// controller will be processed)
			instance.Status.CurrentState = ospdirectorv1beta1.BackupCleaning
			action = "cleaning to prepare for loading"
		}

		if err := r.setStatus(ctx, instance, oldStatus, fmt.Sprintf("OpenStackBackupRequest %s is %s OpenStackBackup %s", instance.Name, action, backup.Name)); err != nil {
			return err
		}
	} else if instance.Status.CurrentState == ospdirectorv1beta1.BackupCleaning {
		// Delete all OSP-D-operator-generated resources in the namespace

		// Get CRs, config maps and secrets we want to delete
		crLists, err := openstackbackup.GetCRLists(ctx, r, instance.Namespace)

		if err != nil {
			return err
		}

		cmList, err := openstackbackup.GetConfigMapList(ctx, r, instance, &crLists)

		if err != nil {
			return err
		}

		secretList, err := openstackbackup.GetSecretList(ctx, r, instance, &crLists)

		if err != nil {
			return err
		}

		// If all lists are empty, set the state to restoring to indicate to other controllers that they should pause all
		// reconcile activity
		clean, err := openstackbackup.CleanNamespace(ctx, r, instance.Namespace, crLists, cmList, secretList)

		if err != nil {
			return err
		}

		if clean {
			instance.Status.CurrentState = ospdirectorv1beta1.BackupLoading

			if err := r.setStatus(ctx, instance, oldStatus, fmt.Sprintf("OpenStackBackupRequest %s is loading OpenStackBackup %s", instance.Name, backup.Name)); err != nil {
				return err
			}
		}
	} else if instance.Status.CurrentState == ospdirectorv1beta1.BackupLoading {
		// Attempt to restore the backup (apply CRs, ConfigMaps and Secrets)
		if err := r.ensureLoadBackup(ctx, instance, oldStatus, backup); err != nil {
			// Ignore "object has been modified errors"
			if !k8s_errors.IsConflict(err) {
				return err
			}
		}
	} else if instance.Status.CurrentState == ospdirectorv1beta1.BackupReconciling {
		// Check status of all the backup's resources that are now reconciling
		if err := r.ensureReconcileBackup(ctx, instance, oldStatus, backup); err != nil {
			return err
		}
	}

	return nil
}

func (r *OpenStackBackupRequestReconciler) ensureLoadBackup(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBackupRequest,
	oldStatus *ospdirectorv1beta1.OpenStackBackupRequestStatus,
	backup *ospdirectorv1beta1.OpenStackBackup,
) error {
	// Create all CRs in the spec first, and set their status to some initial state

	msg := fmt.Sprintf("OpenStackBackup %s initial load", backup.Name)

	// OpenStackNets
	for _, item := range backup.Spec.Crs.OpenStackNets.Items {
		if err := r.ensureLoadBackupResource(ctx, &item); err != nil {
			return err
		}

		// Now try to update the status
		item.Status.CurrentState = shared.NetWaiting
		item.Status.Conditions.UpdateCurrentCondition(item.Status.CurrentState, shared.CommonCondReasonInit, msg)
		if err := r.Status().Update(ctx, &item, &client.UpdateOptions{}); err != nil {
			return err
		}
	}

	// OpenStackNetAttachments
	for _, item := range backup.Spec.Crs.OpenStackNetAttachments.Items {
		if err := r.ensureLoadBackupResource(ctx, &item); err != nil {
			return err
		}

		// Now try to update the status
		item.Status.CurrentState = shared.NetAttachWaiting
		item.Status.Conditions.UpdateCurrentCondition(item.Status.CurrentState, shared.CommonCondReasonInit, msg)
		if err := r.Status().Update(ctx, &item, &client.UpdateOptions{}); err != nil {
			return err
		}
	}

	// OpenStackNetConfigs
	for _, item := range backup.Spec.Crs.OpenStackNetConfigs.Items {
		if err := r.ensureLoadBackupResource(ctx, &item); err != nil {
			return err
		}

		// Now try to update the status
		item.Status.ProvisioningStatus.State = shared.ProvisioningState(shared.NetConfigWaiting)
		item.Status.ProvisioningStatus.Reason = msg
		item.Status.Conditions.UpdateCurrentCondition(shared.ConditionType(item.Status.ProvisioningStatus.State), shared.CommonCondReasonInit, msg)
		if err := r.Status().Update(ctx, &item, &client.UpdateOptions{}); err != nil {
			return err
		}
	}

	// OpenStackMACAddresses
	for _, item := range backup.Spec.Crs.OpenStackMACAddresses.Items {
		if err := r.ensureLoadBackupResource(ctx, &item); err != nil {
			return err
		}

		// Now try to update the status
		item.Status.CurrentState = shared.MACCondTypeWaiting
		item.Status.Conditions.UpdateCurrentCondition(item.Status.CurrentState, shared.CommonCondReasonInit, msg)
		if err := r.Status().Update(ctx, &item, &client.UpdateOptions{}); err != nil {
			return err
		}
	}

	// OpenStackProvisionServers
	for _, item := range backup.Spec.Crs.OpenStackProvisionServers.Items {
		if err := r.ensureLoadBackupResource(ctx, &item); err != nil {
			return err
		}

		// Now try to update the status
		item.Status.ProvisioningStatus.State = shared.ProvisioningState(shared.ProvisionServerCondTypeWaiting)
		item.Status.ProvisioningStatus.Reason = msg
		item.Status.Conditions.UpdateCurrentCondition(shared.ConditionType(item.Status.ProvisioningStatus.State), shared.CommonCondReasonInit, msg)
		if err := r.Status().Update(ctx, &item, &client.UpdateOptions{}); err != nil {
			return err
		}
	}

	// OpenStackBaremetalSets
	for _, item := range backup.Spec.Crs.OpenStackBaremetalSets.Items {
		if err := r.ensureLoadBackupResource(ctx, &item); err != nil {
			return err
		}

		// Now try to update the status
		item.Status.ProvisioningStatus.State = shared.ProvisioningState(shared.BaremetalSetCondTypeWaiting)
		item.Status.ProvisioningStatus.Reason = msg
		item.Status.Conditions.UpdateCurrentCondition(shared.ConditionType(item.Status.ProvisioningStatus.State), shared.CommonCondReasonInit, msg)
		if err := r.Status().Update(ctx, &item, &client.UpdateOptions{}); err != nil {
			return err
		}
	}

	// OpenStackClients
	for _, item := range backup.Spec.Crs.OpenStackClients.Items {
		if err := r.ensureLoadBackupResource(ctx, &item); err != nil {
			return err
		}

		// Now try to update the status
		item.Status.Conditions.UpdateCurrentCondition(shared.CommonCondTypeWaiting, shared.CommonCondReasonInit, msg)
		if err := r.Status().Update(ctx, &item, &client.UpdateOptions{}); err != nil {
			return err
		}
	}

	// OpenStackVMSets
	for _, item := range backup.Spec.Crs.OpenStackVMSets.Items {
		if err := r.ensureLoadBackupResource(ctx, &item); err != nil {
			return err
		}

		// Now try to update the status
		item.Status.ProvisioningStatus.State = shared.ProvisioningState(shared.VMSetCondTypeWaiting)
		item.Status.ProvisioningStatus.Reason = msg
		item.Status.Conditions.UpdateCurrentCondition(shared.ConditionType(item.Status.ProvisioningStatus.State), shared.CommonCondReasonInit, msg)
		if err := r.Status().Update(ctx, &item, &client.UpdateOptions{}); err != nil {
			return err
		}
	}

	// OpenStackControlPlanes
	for _, item := range backup.Spec.Crs.OpenStackControlPlanes.Items {
		if err := r.ensureLoadBackupResource(ctx, &item); err != nil {
			return err
		}

		// Now try to update the status
		item.Status.ProvisioningStatus.State = shared.ProvisioningState(shared.ControlPlaneWaiting)
		item.Status.ProvisioningStatus.Reason = msg
		item.Status.Conditions.UpdateCurrentCondition(shared.ConditionType(item.Status.ProvisioningStatus.State), shared.CommonCondReasonInit, msg)
		if err := r.Status().Update(ctx, &item, &client.UpdateOptions{}); err != nil {
			return err
		}
	}

	// Now create ConfigMaps and Secrets
	for _, item := range backup.Spec.ConfigMaps.Items {
		if err := r.ensureLoadBackupResource(ctx, &item); err != nil {
			return err
		}
	}

	for _, item := range backup.Spec.Secrets.Items {
		if err := r.ensureLoadBackupResource(ctx, &item); err != nil {
			return err
		}
	}

	// If we get here, everything has been loaded and we can transition to the reconciliation phase
	instance.Status.CurrentState = ospdirectorv1beta1.BackupReconciling
	if err := r.setStatus(ctx, instance, oldStatus, fmt.Sprintf("OpenStackBackup %s is now reconciling", instance.Name)); err != nil {
		return err
	}

	return nil
}

func (r *OpenStackBackupRequestReconciler) ensureLoadBackupResource(ctx context.Context, item client.Object) error {
	op, err := controllerutil.CreateOrPatch(ctx, r.Client, item, func() error {
		return nil
	})

	if err != nil {
		return err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("%s %s successfully reconciled - operation: %s", item.GetObjectKind().GroupVersionKind().Kind, item.GetName(), string(op)))
	}

	return nil
}

func (r *OpenStackBackupRequestReconciler) ensureReconcileBackup(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackBackupRequest,
	oldStatus *ospdirectorv1beta1.OpenStackBackupRequestStatus,
	backup *ospdirectorv1beta1.OpenStackBackup,
) error {
	// Check each CR in the OpenStackBackup spec that has a state and wait for the "finished" equivalent for that CRD

	// Get all existing CRs
	crLists, err := openstackbackup.GetCRLists(ctx, r, instance.Namespace)

	if err != nil {
		return err
	}

	if restored, holdUps := openstackbackup.GetAreResourcesRestored(backup, crLists); !restored {
		holdUpsSb := strings.Builder{}

		for index, holdUp := range holdUps {
			holdUpsSb.WriteString(holdUp.GetObjectKind().GroupVersionKind().Kind)
			holdUpsSb.WriteString(": ")
			holdUpsSb.WriteString(holdUp.GetName())

			if index != len(holdUps)-1 {
				holdUpsSb.WriteString(", ")
			}
		}

		r.Log.Info(fmt.Sprintf("Restore of OpenStackBackup %s for OpenStackBackupRequest %s is waiting for: [%s]", backup.Name, instance.Name, holdUpsSb.String()))
	} else {
		// If we reach this point, all CRs from the backup have been successfully restored/configured
		instance.Status.CompletionTimestamp = metav1.Now()
		instance.Status.CurrentState = ospdirectorv1beta1.BackupRestored
		if err := r.setStatus(ctx, instance, oldStatus, fmt.Sprintf("OpenStackBackup %s has been successfully restored", backup.Name)); err != nil {
			return err
		}
	}

	return nil
}
