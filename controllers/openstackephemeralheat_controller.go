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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	k8s_rand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
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

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackephemeralheats,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackephemeralheats/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackephemeralheats/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=create;update;get;list;watch;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=create;update;get;list;watch;patch;deletecollection
// +kubebuilder:rbac:groups=core,resources=services,verbs=create;update;get;list;watch;patch;deletecollection
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=create;update;get;list;watch;patch;deletecollection
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=create;update;get;list;watch;patch;deletecollection

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

	// examine DeletionTimestamp to determine if object is under deletion
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, openstackephemeralheat.FinalizerName) {
			controllerutil.AddFinalizer(instance, openstackephemeralheat.FinalizerName)
			if err := r.Update(context.Background(), instance); err != nil {
				return ctrl.Result{}, err
			}
			r.Log.Info(fmt.Sprintf("Finalizer %s added to CR %s", openstackephemeralheat.FinalizerName, instance.Name))
		}
	} else {
		// 1. check if finalizer is there
		// Reconcile if finalizer got already removed
		if !controllerutil.ContainsFinalizer(instance, openstackephemeralheat.FinalizerName) {
			return ctrl.Result{}, nil
		}

		// 2. Clean up resources used by the operator
		err = r.resourceCleanup(instance)
		if err != nil {
			return ctrl.Result{}, err
		}

		// 3. as last step remove the finalizer on the operator CR to finish delete
		controllerutil.RemoveFinalizer(instance, openstackephemeralheat.FinalizerName)
		err = r.Client.Update(context.TODO(), instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("CR %s deleted", instance.Name))
		return ctrl.Result{}, nil
	}

	cmLabels := common.GetLabels(instance, openstackephemeralheat.AppLabel, map[string]string{})
	// only generate the password secret once
	passwordSecret, _, err := common.GetSecret(r, "ephemeral-heat-"+instance.Name, instance.Namespace)
	if err != nil && k8s_errors.IsNotFound(err) {
		passwordSecret = openstackephemeralheat.PasswordSecret("ephemeral-heat-"+instance.Name, instance.Namespace, cmLabels, k8s_rand.String(10))
		_, op, err := common.CreateOrUpdateSecret(r, instance, passwordSecret)
		if err != nil {
			return ctrl.Result{}, err
		}

		if op != controllerutil.OperationResultNone {
			r.Log.Info(fmt.Sprintf("Secret %s successfully reconciled - operation: %s", instance.Name, string(op)))
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}

	envVars := make(map[string]common.EnvSetter)
	templateParameters := make(map[string]interface{})
	templateParameters["MariaDBHost"] = "mariadb-" + instance.Name
	templateParameters["RabbitMQHost"] = "rabbitmq-" + instance.Name
	templateParameters["MariaDBPassword"] = string(passwordSecret.Data["password"])

	// ConfigMaps for all services (MariaDB/Rabbit/Heat)
	cms := []common.Template{
		// ScriptsConfigMap
		{
			Name:           "openstackephemeralheat-" + instance.Name,
			Namespace:      instance.Namespace,
			Type:           common.TemplateTypeScripts,
			InstanceType:   instance.Kind,
			AdditionalData: map[string]string{},
			ConfigOptions:  templateParameters,
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
		r.Log.Info(fmt.Sprintf("MariaDB Pod %s created or updated - operation: %s", instance.Name, string(op)))
		return ctrl.Result{RequeueAfter: time.Second * 1}, err
	}
	if len(mariadbPod.Status.ContainerStatuses) > 0 && !mariadbPod.Status.ContainerStatuses[0].Ready {
		r.Log.Info(fmt.Sprintf("Waiting on MariaDB to start for: %s", instance.Name))
		return ctrl.Result{RequeueAfter: time.Second * 3}, err
	}

	// MariaDB Service
	mariadbService := openstackephemeralheat.MariadbService(instance, r.Scheme)
	op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, mariadbService, func() error {
		err := controllerutil.SetControllerReference(instance, mariadbService, r.Scheme)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("MariaDB Service %s created or updated - operation: %s", instance.Name, string(op)))
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
		r.Log.Info(fmt.Sprintf("RabbitMQ Pod %s created or updated - operation: %s", instance.Name, string(op)))
		return ctrl.Result{RequeueAfter: time.Second * 1}, err
	}
	if len(rabbitmqPod.Status.ContainerStatuses) > 0 && !rabbitmqPod.Status.ContainerStatuses[0].Ready {
		r.Log.Info(fmt.Sprintf("Waiting on Rabbitmq pod to start for: %s", instance.Name))
		return ctrl.Result{RequeueAfter: time.Second * 3}, err
	}

	// RabbitMQ Service
	rabbitMQService := openstackephemeralheat.RabbitmqService(instance, r.Scheme)
	op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, rabbitMQService, func() error {
		err := controllerutil.SetControllerReference(instance, rabbitMQService, r.Scheme)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Rabbitmq Service %s created or updated - operation: %s", instance.Name, string(op)))
	}

	// Heat API (this creates the Heat Database and runs DBsync)
	heatAPIPod := openstackephemeralheat.HeatAPIPod(instance)
	op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, heatAPIPod, func() error {
		err := controllerutil.SetControllerReference(instance, heatAPIPod, r.Scheme)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Heat API Pod %s created or updated - operation: %s", instance.Name, string(op)))
		return ctrl.Result{RequeueAfter: time.Second * 5}, err // heat init containers take time to launch
	}
	if len(heatAPIPod.Status.ContainerStatuses) > 0 && !heatAPIPod.Status.ContainerStatuses[0].Ready {
		r.Log.Info(fmt.Sprintf("Waiting on Heat API pod to start for: %s", instance.Name))
		return ctrl.Result{RequeueAfter: time.Second * 3}, err
	}

	// Heat Service
	heatAPIService := openstackephemeralheat.HeatAPIService(instance, r.Scheme)
	op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, heatAPIService, func() error {
		err := controllerutil.SetControllerReference(instance, heatAPIService, r.Scheme)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Heat API Service %s created or updated - operation: %s", instance.Name, string(op)))
	}

	// Heat Engine Replicaset
	heatEngineReplicaset := openstackephemeralheat.HeatEngineReplicaSet(instance)
	op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, heatEngineReplicaset, func() error {
		err := controllerutil.SetControllerReference(instance, heatEngineReplicaset, r.Scheme)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Heat Engine Replicaset %s created or updated - operation: %s", instance.Name, string(op)))
		return ctrl.Result{RequeueAfter: time.Second * 1}, err
	}
	if heatEngineReplicaset.Status.AvailableReplicas < instance.Spec.HeatEngineReplicas {
		r.Log.Info(fmt.Sprintf("Waiting on Heat Engine Replicas to start for: %s", instance.Name))
		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	}
	if err := r.setActive(instance, true); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil

}

func (r *OpenStackEphemeralHeatReconciler) setActive(instance *ospdirectorv1beta1.OpenStackEphemeralHeat, active bool) error {
	if instance.Status.Active != active {
		instance.Status.Active = active
		if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
			return err
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackEphemeralHeatReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackEphemeralHeat{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.ReplicaSet{}).
		Complete(r)
}

func (r *OpenStackEphemeralHeatReconciler) resourceCleanup(instance *ospdirectorv1beta1.OpenStackEphemeralHeat) error {

	labelSelector := common.GetLabels(instance, openstackephemeralheat.AppLabel, map[string]string{})

	// delete pods
	if err := common.DeletePodsWithLabel(r, instance, labelSelector); err != nil {
		return err
	}
	// delete secret
	if err := common.DeleteSecretsWithLabel(r, instance, labelSelector); err != nil {
		return err
	}
	// delete service
	if err := common.DeleteSericesWithLabel(r, instance, labelSelector); err != nil {
		return err
	}
	// delete replicaset
	if err := common.DeleteReplicasetsWithLabel(r, instance, labelSelector); err != nil {
		return err
	}

	return nil
}
