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
	vmset "github.com/openstack-k8s-operators/osp-director-operator/pkg/vmset"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// +kubebuilder:rbac:groups=hco.kubevirt.io,namespace=openstack,resources="*",verbs="*"
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=create;delete;get;list;patch;update;watch

// Reconcile - control plane
func (r *OpenStackControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("controlplane", req.NamespacedName)

	// Fetch the controller VM instance
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

	envVars := make(map[string]common.EnvSetter)

	// Secret - used for deployment to ssh into the overcloud nodes,
	//          gets added to the controller VMs cloud-admin user using cloud-init
	deploymentSecretName := strings.ToLower(controlplane.AppLabel) + "-ssh-keys"

	deploymentSecret, secretHash, err := common.GetSecret(r, deploymentSecretName, instance.Namespace)
	if err != nil && k8s_errors.IsNotFound(err) {
		var op controllerutil.OperationResult

		r.Log.Info(fmt.Sprintf("Creating deployment ssh secret: %s", deploymentSecretName))
		deploymentSecret, err = common.SSHKeySecret(deploymentSecretName, instance.Namespace, map[string]string{deploymentSecretName: ""})
		if err != nil {
			return ctrl.Result{}, err
		}
		secretHash, op, err = common.CreateOrUpdateSecret(r, instance, deploymentSecret)
		if err != nil {
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			r.Log.Info(fmt.Sprintf("Secret %s successfully reconciled - operation: %s", deploymentSecret.Name, string(op)))
		}
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("error get secret %s: %v", deploymentSecretName, err)
	}
	envVars[deploymentSecret.Name] = common.EnvValue(secretHash)

	if instance.Spec.PasswordSecret != "" {
		// check if specified password secret exists before creating the controlplane
		_, _, err = common.GetSecret(r, instance.Spec.PasswordSecret, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("PasswordSecret %s not found but specified in CR, next reconcile in 30s", instance.Spec.PasswordSecret)
			}
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("PasswordSecret %s exists", instance.Spec.PasswordSecret))
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

	for _, vmRole := range instance.Spec.VirtualMachineRoles {
		// create VIP for all networks for any of the VM roles
		ipsetDetails := common.IPSet{
			Networks:            vmRole.Networks,
			Role:                controlplane.Role,
			HostCount:           controlplane.Count,
			VIP:                 true,
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

		if len(ipset.Status.HostIPs) < controlplane.Count {
			r.Log.Info(fmt.Sprintf("IPSet has not yet reached the required replicas %d", controlplane.Count))
			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}

		// update VIP network status
		for _, netName := range vmRole.Networks {
			// TODO: mschuppert, host status format now used for controlplane, openstackclient and vmset, TBD baremetalset
			r.setNetStatus(instance, &hostnameDetails, netName, ipset.Status.HostIPs[hostnameDetails.Hostname].IPAddresses[netName])
		}

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
		vmSet := &ospdirectorv1beta1.OpenStackVMSet{
			ObjectMeta: metav1.ObjectMeta{
				// use the role name as the VM CR name
				Name:      strings.ToLower(vmRole.RoleName),
				Namespace: instance.Namespace,
			},
		}

		op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, vmSet, func() error {
			vmSet.Spec.BaseImageURL = vmRole.BaseImageURL
			vmSet.Spec.VMCount = vmRole.RoleCount
			vmSet.Spec.Cores = vmRole.Cores
			vmSet.Spec.Memory = vmRole.Memory
			vmSet.Spec.DiskSize = vmRole.DiskSize
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
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			r.Log.Info(fmt.Sprintf("VMSet CR %s successfully reconciled - operation: %s", instance.Name, string(op)))
		}
	}

	// get PodIP's from the OSP controller VMs and update openstackclient
	controllerPodList, err := common.GetAllPodsWithLabel(r, map[string]string{
		"openstackvmsets.osp-director.openstack.org/ospcontroller": "True",
	}, instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO:
	// - check vm container status and update CR.Status.VMsReady
	// - change CR.Status.VMs to be struct with name + Pod IP of the controllers

	// Create openstack client pod
	openstackclient := &ospdirectorv1beta1.OpenStackClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openstackclient",
			Namespace: instance.Namespace,
		},
	}
	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, openstackclient, func() error {
		openstackclient.Spec.ImageURL = instance.Spec.OpenStackClientImageURL
		openstackclient.Spec.DeploymentSSHSecret = deploymentSecretName
		openstackclient.Spec.CloudName = instance.Name
		openstackclient.Spec.StorageClass = instance.Spec.OpenStackClientStorageClass
		openstackclient.Spec.GitSecret = instance.Spec.GitSecret

		if len(instance.Spec.OpenStackClientNetworks) > 0 {
			openstackclient.Spec.Networks = instance.Spec.OpenStackClientNetworks
		}
		openstackclient.Spec.HostAliases = common.HostAliasesFromPodlist(controllerPodList)

		err := controllerutil.SetControllerReference(instance, openstackclient, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("OpenStackClient CR successfully reconciled - operation: %s", string(op)))
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
