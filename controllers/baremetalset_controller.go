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
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
)

// BaremetalSetReconciler reconciles a BaremetalSet object
type BaremetalSetReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=baremetalsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=baremetalsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=baremetalsets/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=provisionservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=provisionservers/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=provisionservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts/status,verbs=get;update;patch

// Reconcile baremetalset
func (r *BaremetalSetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("baremetalset", req.NamespacedName)
	ctx := context.Background()

	// Fetch the instance
	baremetalset := &ospdirectorv1beta1.BaremetalSet{}
	if err := r.Client.Get(ctx, req.NamespacedName, baremetalset); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile
			// request.  Owned objects are automatically garbage collected.  For
			// additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// First deploy the provisioning image (Apache) server
	provisionServer, op, err := r.provisionServerCreateOrUpdate(baremetalset)

	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("BaremetalSet %s ProvisionServer successfully reconciled - operation: %s", baremetalset.Name, string(op)))
		return ctrl.Result{}, nil
	}

	if provisionServer.Status.LocalImageURL == "" {
		r.Log.Info(fmt.Sprintf("BaremetalSet %s ProvisionServer local image URL not yet available, requeuing and waiting", baremetalset.Name))
		return ctrl.Result{RequeueAfter: time.Second * 30}, err
	}

	// Search for baremetalhosts that don't have consumerRef or Online set
	availableBaremetalHosts := []string{}
	baremetalHostsList := &metal3v1alpha1.BareMetalHostList{}
	listOpts := []client.ListOption{
		client.InNamespace("openshift-machine-api"),
	}
	err = r.Client.List(context.TODO(), baremetalHostsList, listOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, baremetalHost := range baremetalHostsList.Items {

		if baremetalHost.Spec.ConsumerRef != nil || baremetalHost.Spec.Online == true {
			continue
		}
		r.Log.Info(fmt.Sprintf("Available BaremetalHost: %s", baremetalHost.ObjectMeta.Name))
		availableBaremetalHosts = append(availableBaremetalHosts, baremetalHost.ObjectMeta.Name)
	}

	// for each available BaremetalHost we Update the reference to use our image
	for _, availableHost := range availableBaremetalHosts {

		foundBaremetalHost := &metal3v1alpha1.BareMetalHost{}
		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: availableHost, Namespace: "openshift-machine-api"}, foundBaremetalHost)

		if err != nil {
			return ctrl.Result{}, err
		}

		r.Log.Info(fmt.Sprintf("Updating BaremetalHost: %s", availableHost))
		foundBaremetalHost.Spec.Online = true
		foundBaremetalHost.Spec.ConsumerRef = &corev1.ObjectReference{Name: baremetalset.Name, Kind: "BaremetalSet"}
		foundBaremetalHost.Spec.Image = &metal3v1alpha1.Image{
			URL:      provisionServer.Status.LocalImageURL,
			Checksum: fmt.Sprintf("%s.md5sum", provisionServer.Status.LocalImageURL),
		}
		err = r.Client.Update(context.TODO(), foundBaremetalHost)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Second * 5}, err

	}

	return ctrl.Result{}, nil
}

// SetupWithManager - prepare controller for use with operator manager
func (r *BaremetalSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.BaremetalSet{}).
		Owns(&ospdirectorv1beta1.ProvisionServer{}).
		Complete(r)
}

func (r *BaremetalSetReconciler) provisionServerCreateOrUpdate(instance *ospdirectorv1beta1.BaremetalSet) (*ospdirectorv1beta1.ProvisionServer, controllerutil.OperationResult, error) {
	provisionServer := &ospdirectorv1beta1.ProvisionServer{
		ObjectMeta: v1.ObjectMeta{
			Name:      instance.ObjectMeta.Name + "-provisionserver",
			Namespace: instance.ObjectMeta.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, provisionServer, func() error {
		// TODO: Surface the port in BaremetalSet?
		provisionServer.Spec.Port = 6190
		provisionServer.Spec.RhelImageURL = instance.Spec.RhelImageURL

		err := controllerutil.SetControllerReference(instance, provisionServer, r.Scheme)

		if err != nil {
			return err
		}

		return nil
	})

	return provisionServer, op, err
}
