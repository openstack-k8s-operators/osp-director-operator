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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstackclient "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackclient"
	openstackephemeralheat "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackephemeralheat"
)

// OpenStackEphemeralHeatReconciler reconciles a OpenStackEphemeralHeat object
type OpenStackEphemeralHeatReconciler struct {
	client.Client
	Kclient kubernetes.Interface

	Log    logr.Logger
	Scheme *runtime.Scheme
}

// GetClient -
func (r *OpenStackEphemeralHeatReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackEphemeralHeatReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackEphemeralHeatReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackEphemeralHeatReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackephemeralheats,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackephemeralheats/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackephemeralheats/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *OpenStackEphemeralHeatReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstackephemeralheat", req.NamespacedName)

	// Fetch the controller VM instance
	instance := &ospdirectorv1beta1.OpenStackEphemeralHeat{}
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

	cmLabels := common.GetLabels(instance.Name, openstackclient.AppLabel)
	envVars := make(map[string]common.EnvSetter)

	// ConfigMaps for all services (MariaDB/Rabbit/Heat)
	cms := []common.Template{
		// ScriptsConfigMap
		{
			Name:           "openstackephemeralheat",
			Namespace:      instance.Namespace,
			Type:           common.TemplateTypeScripts,
			InstanceType:   instance.Kind,
			AdditionalData: map[string]string{},
			ConfigOptions:  make(map[string]interface{}),
			Labels:         cmLabels,
		},
	}
	err = common.EnsureConfigMaps(r, instance, cms, &envVars)
	if err != nil {
		return ctrl.Result{}, nil
	}

	// MariaDB Pod
	mariadbPod := openstackephemeralheat.MariadbPod(instance)
	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, mariadbPod, func() error {
		err := controllerutil.SetControllerReference(instance, mariadbPod, r.Scheme)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("MariaDB Pod %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// MariaDB Service
	mariadbService := openstackephemeralheat.MariadbService(instance, r.Scheme)
	foundService := &corev1.Service{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: mariadbService.Name, Namespace: mariadbService.Namespace}, foundService)
	if err != nil && k8s_errors.IsNotFound(err) {

		r.Log.Info("Creating MariaDB Service", "Service.Namespace", mariadbService.Namespace, "Service.Name", mariadbService.Name)
		err = r.Client.Create(context.TODO(), mariadbService)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// RabbitMQ Pod
	rabbitmqPod := openstackephemeralheat.RabbitmqPod(instance)
	op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, rabbitmqPod, func() error {
		err := controllerutil.SetControllerReference(instance, rabbitmqPod, r.Scheme)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("RabbitMQ Pod %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// RabbitMQ Service
	rabbitMQService := openstackephemeralheat.RabbitmqService(instance, r.Scheme)
	foundService = &corev1.Service{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: rabbitMQService.Name, Namespace: rabbitMQService.Namespace}, foundService)
	if err != nil && k8s_errors.IsNotFound(err) {

		r.Log.Info("Creating RabbitMQ Service", "Service.Namespace", rabbitMQService.Namespace, "Service.Name", rabbitMQService.Name)
		err = r.Client.Create(context.TODO(), rabbitMQService)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	} else if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackEphemeralHeatReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackEphemeralHeat{}).
		Complete(r)
}
