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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openstack-k8s-operators/cinder-operator/pkg/cinderapi"
	"github.com/openstack-k8s-operators/cinder-operator/pkg/common"
	"github.com/openstack-k8s-operators/lib-common/pkg/util"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	provisionserver "github.com/openstack-k8s-operators/osp-director-operator/pkg/provisionserver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProvisionServerReconciler reconciles a ProvisionServer object
type ProvisionServerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=provisionservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=provisionservers/status,verbs=get;update;patch

func (r *ProvisionServerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("provisionserver", req.NamespacedName)

	// Fetch the ProvisionServer instance
	instance := &ospdirectorv1beta1.ProvisionServer{}
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

	envVars := make(map[string]util.EnvSetter)

	// config maps
	op, err := r.httpdConfigMapCreateOrUpdate(instance)

	if err != nil {
		return ctrl.Result{}, err
	}

	// provisionserver
	// Create or update the Deployment object
	op, err = r.deploymentCreateOrUpdate(instance, envVars)

	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ProvisionServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.ProvisionServer{}).
		Complete(r)
}

func (r *ProvisionServerReconciler) httpdConfigMapCreateOrUpdate(instance *ospdirectorv1beta1.ProvisionServer) (controllerutil.OperationResult, error) {
	cm := provisionserver.HttpdConfigMap(instance)

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, cm, func() error {
		err := controllerutil.SetControllerReference(instance, cm, r.Scheme)

		if err != nil {
			return err
		}

		return nil
	})

	return op, err

}

func (r *ProvisionServerReconciler) deploymentCreateOrUpdate(instance *ospdirectorv1beta1.ProvisionServer, envVars map[string]util.EnvSetter) (controllerutil.OperationResult, error) {

	// Get volumes
	initVolumeMounts := provisionserver.GetInitVolumeMounts(instance.Name)
	volumeMounts := provisionserver.GetVolumeMounts(instance.Name)
	volumes := provisionserver.GetVolumes(instance.Name)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, deployment, func() error {

		// Daemonset selector is immutable so we set this value only if
		// a new object is going to be created
		if deployment.ObjectMeta.CreationTimestamp.IsZero() {
			deployment.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: common.GetLabels(instance.Name, cinderapi.AppLabel),
			}
		}

		if len(deployment.Spec.Template.Spec.Containers) != 1 {
			deployment.Spec.Template.Spec.Containers = make([]corev1.Container, 1)
		}
		envs := util.MergeEnvs(deployment.Spec.Template.Spec.Containers[0].Env, envVars)

		// labels
		common.InitLabelMap(&deployment.Spec.Template.Labels)
		for k, v := range common.GetLabels(instance.Name, cinderapi.AppLabel) {
			deployment.Spec.Template.Labels[k] = v
		}

		replicas := int32(1)

		deployment.Spec.Replicas = &replicas
		deployment.Spec.Template.Spec = corev1.PodSpec{
			Volumes: volumes,
			Containers: []corev1.Container{
				{
					Name:         "osp-httpd",
					Image:        "quay.io/abays/httpd:2.4-alpine",
					Env:          envs,
					VolumeMounts: volumeMounts,
				},
			},
		}

		initContainerDetails := provisionserver.InitContainer{
			ContainerImage: "quay.io/abays/downloader:latest",
			RhelImageURL:   "http://172.22.0.1/images/rhel-guest-image-8.3-417.x86_64.qcow2.gz",
			Privileged:     true,
			VolumeMounts:   initVolumeMounts,
		}

		deployment.Spec.Template.Spec.InitContainers = provisionserver.GetInitContainer(initContainerDetails)

		err := controllerutil.SetControllerReference(instance, deployment, r.Scheme)

		if err != nil {
			return err
		}

		return nil
	})

	return op, err
}
