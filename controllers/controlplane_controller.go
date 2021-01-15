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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ControlPlaneReconciler reconciles a ControlPlane object
type ControlPlaneReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *ControlPlaneReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *ControlPlaneReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *ControlPlaneReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *ControlPlaneReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=controlplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=controlplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=controlplanes/finalizers,verbs=update
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=controllervms,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=controllervms/finalizers,verbs=update
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hco.kubevirt.io,namespace=openstack,resources="*",verbs="*"
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=create;delete;get;list;patch;update;watch

// Reconcile - control plane
func (r *ControlPlaneReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("controlplane", req.NamespacedName)

	// Fetch the controller VM instance
	instance := &ospdirectorv1beta1.ControlPlane{}
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
	if err != nil && errors.IsNotFound(err) {
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
			return ctrl.Result{}, nil
		}
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("error get secret %s: %v", deploymentSecretName, err)
	}
	envVars[deploymentSecret.Name] = common.EnvValue(secretHash)

	// Create or update the controllerVM CR object
	ospControllerVM := &ospdirectorv1beta1.ControllerVM{
		ObjectMeta: metav1.ObjectMeta{
			// use the role name as the VM CR name
			Name:      strings.ToLower(instance.Spec.Controller.Role),
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, ospControllerVM, func() error {
		ospControllerVM.Spec.BaseImageURL = instance.Spec.Controller.BaseImageURL
		ospControllerVM.Spec.ControllerCount = instance.Spec.Controller.ControllerCount
		ospControllerVM.Spec.Cores = instance.Spec.Controller.Cores
		ospControllerVM.Spec.Memory = instance.Spec.Controller.Memory
		ospControllerVM.Spec.DiskSize = instance.Spec.Controller.DiskSize
		ospControllerVM.Spec.StorageClass = instance.Spec.Controller.StorageClass
		ospControllerVM.Spec.DeploymentSSHSecret = deploymentSecretName
		ospControllerVM.Spec.OSPNetwork = instance.Spec.Controller.OSPNetwork
		ospControllerVM.Spec.Networks = instance.Spec.Controller.Networks
		ospControllerVM.Spec.Role = instance.Spec.Controller.Role

		err := controllerutil.SetControllerReference(instance, ospControllerVM, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("ControllerVM CR %s successfully reconciled - operation: %s", instance.Name, string(op)))
		return ctrl.Result{}, nil
	}

	// get PodIP's from the OSP controller VMs and update openstackclient
	controllerPodList, err := common.GetAllPodsWithLabel(r, map[string]string{
		"controllervms.osp-director.openstack.org/ospcontroller": "True",
	}, instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO:
	// - check vm container status and update CR.Status.ControllersReady
	// - change CR.Status.Controllers to be struct with name + Pod IP of the controllers

	// Create openstack client pod
	openstackclient := &ospdirectorv1beta1.OpenStackClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openstackclient",
			Namespace: instance.Namespace,
		},
	}
	op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, openstackclient, func() error {
		openstackclient.Spec.ImageURL = instance.Spec.OpenStackClientImageURL
		openstackclient.Spec.DeploymentSSHSecret = deploymentSecretName
		openstackclient.Spec.CloudName = instance.Name
		// TODO: change with ctrlplan IPs from IPAM when integrated
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
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager -
func (r *ControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// watch for objects in the same namespace as the controller CR
	namespacedFn := handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
		result := []reconcile.Request{}

		// get all CRs from the same namespace
		crs := &ospdirectorv1beta1.ControlPlaneList{}
		listOpts := []client.ListOption{
			client.InNamespace(obj.Meta.GetNamespace()),
		}
		if err := r.Client.List(context.Background(), crs, listOpts...); err != nil {
			r.Log.Error(err, "Unable to retrieve CRs %v")
			return nil
		}

		for _, cr := range crs.Items {
			if obj.Meta.GetNamespace() == cr.Namespace {
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
		For(&ospdirectorv1beta1.ControlPlane{}).
		Owns(&corev1.Secret{}).
		Owns(&ospdirectorv1beta1.ControllerVM{}).
		Owns(&ospdirectorv1beta1.OpenStackClient{}).
		// watch pods in the same namespace as we want to reconcile if
		// e.g. a controller vm gets destroyed
		Watches(&source.Kind{Type: &corev1.Pod{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: namespacedFn,
			}).
		Complete(r)
}
