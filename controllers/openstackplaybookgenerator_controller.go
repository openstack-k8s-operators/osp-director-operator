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
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstackplaybookgenerator "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackplaybookgenerator"
)

// OpenStackPlaybookGeneratorReconciler reconciles a OpenStackPlaybookGenerator object
type OpenStackPlaybookGeneratorReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackPlaybookGeneratorReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackPlaybookGeneratorReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackPlaybookGeneratorReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackPlaybookGeneratorReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackplaybookgenerators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackplaybookgenerators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackplaybookgenerators/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;create;update;delete;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenStackPlaybookGenerator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *OpenStackPlaybookGeneratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstackplaybookgenerator", req.NamespacedName)

	// Fetch the controller VM instance
	instance := &ospdirectorv1beta1.OpenStackPlaybookGenerator{}
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
	//err = r.Client.Status().Update(context.TODO(), instance)
	//if err != nil {
	//r.Log.Error(err, "Failed to update CR status %v")
	//return ctrl.Result{}, err
	//}
	templateParameters := make(map[string]interface{})
	cmLabels := common.GetLabels(instance.Name, openstackplaybookgenerator.AppLabel)
	cms := []common.Template{
		// Custom CM holding Tripleo deployment environment parameter files
		{
			Name:      "tripleo-deploy-config-custom",
			Namespace: instance.Namespace,
			Type:      common.TemplateTypeCustom,
			Labels:    cmLabels,
		},
		// Custom CM holding Tripleo net-config files then used in parameter files
		{
			Name:      "tripleo-net-config",
			Namespace: instance.Namespace,
			Type:      common.TemplateTypeCustom,
			Labels:    cmLabels,
		},
	}
	err = common.EnsureConfigMaps(r, instance, cms, &envVars)
	if err != nil {
		return ctrl.Result{}, nil
	}

	// get tripleo-net-config, created/rendered by admin
	tripleoNetCM, _, err := common.GetConfigMapAndHashWithName(r, "tripleo-net-config", instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// get tripleo-deploy-config-custom, created/rendered by openstackipset controller
	tripleoCustomDeployCM, _, err := common.GetConfigMapAndHashWithName(r, "tripleo-deploy-config-custom", instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	tripleoCustomDeployFiles := tripleoCustomDeployCM.Data
	templateParameters["TripleoCustomDeployFiles"] = tripleoCustomDeployFiles

	// get tripleo-deploy-config, created/rendered by openstackipset controller
	tripleoDeployCM, _, err := common.GetConfigMapAndHashWithName(r, "tripleo-deploy-config", instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}
		return ctrl.Result{}, err
	}

	tripleoDeployFiles := tripleoDeployCM.Data
	templateParameters["TripleoDeployFiles"] = tripleoDeployFiles

	cms = []common.Template{
		{
			Name:           "openstackplaybook-sh",
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

	// Define a new Job object
	configMapHash, err := common.ObjectHash([]corev1.ConfigMap{*tripleoNetCM, *tripleoCustomDeployCM, *tripleoDeployCM})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error calculating configmap hash: %v", err)
	}
	job := openstackplaybookgenerator.PlaybookJob(instance, configMapHash)
	jobHash, err := common.ObjectHash([]corev1.ConfigMap{*tripleoNetCM, *tripleoCustomDeployCM, *tripleoDeployCM})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error calculating configmap hash: %v", err)
	}

	if instance.Status.PlaybookHash != jobHash {
		requeue, err := common.EnsureJob(job, r.Client, r.Log)
		r.Log.Info("Generating Playbooks...")
		if err != nil {
			return ctrl.Result{}, err
		} else if requeue {
			r.Log.Info("Waiting on Playbook Generation...")
			if err := controllerutil.SetControllerReference(instance, job, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: time.Second * 5}, err
		}
	}
	// db sync completed... okay to store the hash to disable it
	if err := r.setPlaybookHash(instance, jobHash); err != nil {
		return ctrl.Result{}, err
	}
	// delete the job
	_, err = common.DeleteJob(job, r.Kclient, r.Log)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *OpenStackPlaybookGeneratorReconciler) setPlaybookHash(instance *ospdirectorv1beta1.OpenStackPlaybookGenerator, hashStr string) error {

	if hashStr != instance.Status.PlaybookHash {
		instance.Status.PlaybookHash = hashStr
		if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
			return err
		}
	}
	return nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackPlaybookGeneratorReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// watch for objects in the same namespace as the controller CR
	//namespacedFn := handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
	namespacedFn := handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// get all CRs from the same namespace (there should only be one)
		crs := &ospdirectorv1beta1.OpenStackPlaybookGeneratorList{}
		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.Client.List(context.Background(), crs, listOpts...); err != nil {
			r.Log.Error(err, "Unable to retrieve CRs %v")
			return nil
		}

		for _, cr := range crs.Items {
			if o.GetNamespace() == cr.Namespace {
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
		For(&ospdirectorv1beta1.OpenStackPlaybookGenerator{}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, namespacedFn).
		Watches(&source.Kind{Type: &corev1.Secret{}}, namespacedFn).
		Complete(r)
}
