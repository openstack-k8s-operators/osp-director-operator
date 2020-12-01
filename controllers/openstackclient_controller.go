/*
Copyright 2020 Red Hat

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
	"k8s.io/apimachinery/pkg/api/errors"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstackclient "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackClientReconciler reconciles a OpenStackClient object
type OpenStackClientReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackClientReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackClientReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackClientReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackClientReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=create;delete;get;list;patch;update;watch

// Reconcile - openstackclient
func (r *OpenStackClientReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("openstackclient", req.NamespacedName)

	// Fetch the controller VM instance
	instance := &ospdirectorv1beta1.OpenStackClient{}
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

	// check for required secrets
	hashes := []ospdirectorv1beta1.Hash{}
	sshSecret, secretHash, err := common.GetSecret(r, instance.Spec.DeploymentSSHSecret, instance.Namespace)
	if err != nil && errors.IsNotFound(err) {
		return ctrl.Result{RequeueAfter: time.Second * 20}, fmt.Errorf("DeploymentSSHSecret secret does not exist: %v", err)
	} else if err != nil {
		return ctrl.Result{}, err
	}

	hashes = append(hashes, ospdirectorv1beta1.Hash{Name: sshSecret.Name, Hash: secretHash})

	// Create or update the pod object
	op, err := r.podCreateOrUpdate(instance, envVars)
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Pod %s successfully reconciled - operation: %s", instance.Name, string(op)))
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager -
func (r *OpenStackClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackClient{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

func (r *OpenStackClientReconciler) podCreateOrUpdate(instance *ospdirectorv1beta1.OpenStackClient, envVars map[string]common.EnvSetter) (controllerutil.OperationResult, error) {
	var terminationGracePeriodSeconds int64 = 0

	// TODO: the overcloud deploy end with create of clouds.yaml
	//       add clouds.yaml into a secret and mount to this pod
	envVars["OS_CLOUDNAME"] = common.EnvValue(instance.Spec.CloudName)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}
	pod.Spec = corev1.PodSpec{
		TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
		Volumes:                       openstackclient.GetVolumes(instance),
		Containers: []corev1.Container{
			{
				Name:            "openstackclient",
				Image:           instance.Spec.ImageURL,
				ImagePullPolicy: corev1.PullAlways,
				Command: []string{
					"/bin/bash",
					"-c",
					"/bin/sleep infinity",
				},
				Env:          common.MergeEnvs([]corev1.EnvVar{}, envVars),
				VolumeMounts: openstackclient.GetVolumeMounts(),
			},
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, pod, func() error {
		// HostAliases
		pod.Spec.HostAliases = instance.Spec.HostAliases
		pod.Spec.Containers[0].Env = common.MergeEnvs([]corev1.EnvVar{}, envVars)

		// labels
		common.InitMap(&pod.Labels)
		for k, v := range common.GetLabels(instance.Name, openstackclient.AppLabel) {
			pod.Labels[k] = v
		}

		err := controllerutil.SetControllerReference(instance, pod, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil && errors.IsInvalid(err) {
		// Delete pod when an unsupported change was requested, like
		// e.g. additional controller VM got up. We just re-create the
		// openstackclient pod
		if err := r.Client.Delete(context.TODO(), pod); err != nil {
			return op, err
		}

		r.Log.Info("openstackclient pod deleted due to spec change")
		return controllerutil.OperationResultUpdated, nil
	}
	return op, err
}
