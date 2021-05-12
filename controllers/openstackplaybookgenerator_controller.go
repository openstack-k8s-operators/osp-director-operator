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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackplaybookgenerators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackplaybookgenerators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackplaybookgenerators/finalizers,verbs=update
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

	// Fetch the instance
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
	templateParameters := make(map[string]interface{})
	cmLabels := common.GetLabels(instance.Name, openstackplaybookgenerator.AppLabel)
	cms := []common.Template{
		// Custom CM holding Tripleo deployment environment parameter files
		{
			Name:      instance.Spec.HeatEnvConfigMap,
			Namespace: instance.Namespace,
			Type:      common.TemplateTypeCustom,
			Labels:    cmLabels,
		},
	}

	if instance.Spec.TarballConfigMap != "" {
		cms = append(cms,
			common.Template{
				Name:      instance.Spec.TarballConfigMap,
				Namespace: instance.Namespace,
				Type:      common.TemplateTypeCustom,
				Labels:    cmLabels,
			},
		)
	}

	err = common.EnsureConfigMaps(r, instance, cms, &envVars)
	if err != nil {
		return ctrl.Result{}, nil
	}

	// get tripleo-deploy-config-custom (customizations provided by administrator)
	tripleoCustomDeployCM, _, err := common.GetConfigMapAndHashWithName(r, instance.Spec.HeatEnvConfigMap, instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	tripleoCustomDeployFiles := tripleoCustomDeployCM.Data
	// we remove the RolesFile from custom templates to avoid having Heat process it directly (FIXME: should we have a separate ConfigMap for the custom Roles file?)
	if len(instance.Spec.RolesFile) > 0 {
		delete(tripleoCustomDeployFiles, instance.Spec.RolesFile)
	}
	templateParameters["TripleoCustomDeployFiles"] = tripleoCustomDeployFiles

	// get tripleo-deploy-config, created/rendered by openstackipset controller
	tripleoDeployCM, _, err := common.GetConfigMapAndHashWithName(r, "tripleo-deploy-config", instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			r.Log.Info("The tripleo-deploy-config map doesn't yet exist. Requeing...")
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}
		return ctrl.Result{}, err
	}

	tripleoDeployFiles := tripleoDeployCM.Data
	templateParameters["TripleoDeployFiles"] = tripleoDeployFiles
	templateParameters["HeatServiceName"] = "heat-" + instance.Name
	templateParameters["RolesFile"] = instance.Spec.RolesFile

	// config hash
	configMapHash, err := common.ObjectHash([]interface{}{tripleoCustomDeployCM.Data, tripleoDeployCM.Data})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error calculating configmap hash: %v", err)
	}

	// extra stuff if custom tarballs are provided
	if instance.Spec.TarballConfigMap != "" {
		tripleoTarballCM, _, err := common.GetConfigMapAndHashWithName(r, instance.Spec.TarballConfigMap, instance.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		// adjust the configMapHash
		configMapHash, err = common.ObjectHash([]interface{}{tripleoCustomDeployCM.Data, tripleoDeployCM.Data, tripleoTarballCM.BinaryData})
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error calculating configmap hash: %v", err)
		}
		tripleoTarballFiles := tripleoTarballCM.BinaryData
		templateParameters["TripleoTarballFiles"] = tripleoTarballFiles
	}

	cms = []common.Template{
		{
			Name:           "openstackplaybook-script-" + instance.Name,
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

	// Create ephemeral heat
	heat := &ospdirectorv1beta1.OpenStackEphemeralHeat{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: ospdirectorv1beta1.OpenStackEphemeralHeatSpec{
			ConfigHash:         configMapHash,
			HeatAPIImageURL:    instance.Spec.EphemeralHeatSettings.HeatAPIImageURL,
			HeatEngineImageURL: instance.Spec.EphemeralHeatSettings.HeatEngineImageURL,
			MariadbImageURL:    instance.Spec.EphemeralHeatSettings.MariadbImageURL,
			RabbitImageURL:     instance.Spec.EphemeralHeatSettings.RabbitImageURL,
			HeatEngineReplicas: instance.Spec.EphemeralHeatSettings.HeatEngineReplicas,
		},
	}

	// Define a new Job object
	job := openstackplaybookgenerator.PlaybookJob(instance, configMapHash)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating playbook job: %v", err)
	}

	if instance.Status.PlaybookHash != configMapHash {

		op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, heat, func() error {
			err := controllerutil.SetControllerReference(instance, heat, r.Scheme)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			r.Log.Info(fmt.Sprintf("OpenStackEphemeralHeat successfully created/updated - operation: %s", string(op)))
			if err := r.setCurrentState(instance, "initializing"); err != nil {
				return ctrl.Result{}, err
			}
		}
		if !heat.Status.Active {
			r.Log.Info("Waiting on Ephemeral Heat instance to launch...")
			return ctrl.Result{RequeueAfter: time.Second * 5}, err
		}

		// configMap Hash changed after Ephemeral Heat was created
		if heat.Spec.ConfigHash != configMapHash {
			err = r.Client.Delete(context.TODO(), heat)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: time.Second * 5}, err
		}

		op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, job, func() error {

			err := controllerutil.SetControllerReference(instance, job, r.Scheme)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			r.Log.Info(fmt.Sprintf("Job successfully created/updated - operation: %s", string(op)))
		}

		// configMap Hash changed while job was running (NOTE: configHash is only ENV in the job)
		r.Log.Info(fmt.Sprintf("Job Hash : %s", job.Spec.Template.Spec.Containers[0].Env[0].Value))
		r.Log.Info(fmt.Sprintf("ConfigMap Hash : %s", configMapHash))
		if configMapHash != job.Spec.Template.Spec.Containers[0].Env[0].Value {
			_, err = common.DeleteJob(job, r.Kclient, r.Log)
			if err != nil {
				return ctrl.Result{}, err
			}

			// in this case delete heat too as the database may have been used
			err = r.Client.Delete(context.TODO(), heat)
			if err != nil {
				return ctrl.Result{}, err
			}
			r.Log.Info("ConfigMap has changed. Requeing to start again...")
			return ctrl.Result{RequeueAfter: time.Second * 5}, err

		}

		requeue, err := common.WaitOnJob(job, r.Client, r.Log)
		r.Log.Info("Generating Playbooks...")
		if err != nil {
			// the job failed in error
			deleteErr := r.Client.Delete(context.TODO(), heat)
			if err != nil {
				return ctrl.Result{}, deleteErr
			}
			return ctrl.Result{}, err
		} else if requeue {
			r.Log.Info("Waiting on Playbook Generation...")
			if err := r.setCurrentState(instance, "generating"); err != nil {
				return ctrl.Result{}, err
			}
			if err := controllerutil.SetControllerReference(instance, job, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: time.Second * 5}, err
		}
	}

	if err := r.setPlaybookHash(instance, configMapHash); err != nil {
		return ctrl.Result{}, err
	}
	_, err = common.DeleteJob(job, r.Kclient, r.Log)
	if err != nil {
		return ctrl.Result{}, err
	}

	// cleanup the ephemeral Heat
	err = r.Client.Delete(context.TODO(), heat)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if err := r.setCurrentState(instance, "finished"); err != nil {
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

func (r *OpenStackPlaybookGeneratorReconciler) setCurrentState(instance *ospdirectorv1beta1.OpenStackPlaybookGenerator, currentState string) error {

	if currentState != instance.Status.CurrentState {
		instance.Status.CurrentState = currentState
		if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
			return err
		}
	}
	return nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackPlaybookGeneratorReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// watch for objects in the same namespace as the controller CR
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
		Owns(&corev1.ConfigMap{}).
		Owns(&ospdirectorv1beta1.OpenStackEphemeralHeat{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
