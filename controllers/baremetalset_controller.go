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
	"net"
	"strings"
	"time"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/baremetalset"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
)

const ()

// BaremetalSetReconciler reconciles a BaremetalSet object
type BaremetalSetReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *BaremetalSetReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *BaremetalSetReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *BaremetalSetReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *BaremetalSetReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=baremetalsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=baremetalsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=baremetalsets/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=provisionservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=provisionservers/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=provisionservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=create;delete;get;list;patch;update;watch

// Reconcile baremetalset
func (r *BaremetalSetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("baremetalset", req.NamespacedName)
	ctx := context.Background()

	// Fetch the instance
	instance := &ospdirectorv1beta1.BaremetalSet{}
	if err := r.Client.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile
			// request.  Owned objects are automatically garbage collected.  For
			// additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	finalizerName := "baremetalset.osp-director.openstack.org-" + instance.Name
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, finalizerName) {
			controllerutil.AddFinalizer(instance, finalizerName)
			if err := r.Update(context.Background(), instance); err != nil {
				return reconcile.Result{}, err
			}
			r.Log.Info(fmt.Sprintf("Finalizer %s added to CR %s", finalizerName, instance.Name))
		}
	} else {
		// 1. check if finalizer is there
		// Reconcile if finalizer got already removed
		if !controllerutil.ContainsFinalizer(instance, finalizerName) {
			return ctrl.Result{}, nil
		}

		// 2. Clean up resources used by the operator
		// BareMetalHost resources in the openshift-machine-api namespace (don't delete, just deprovision)
		err := r.baremetalHostCleanup(instance)
		if err != nil && !errors.IsNotFound(err) {
			// ignore not found errors if the object is already gone
			return ctrl.Result{}, err
		}

		// 3. cleanup resources created by operator
		// a. Delete objects in non openstack namespace which have the owner reference label
		//    - secret objects in openshift-machine-api namespace
		err = r.deleteOwnerRefLabeledObjects(instance)
		if err != nil && !errors.IsNotFound(err) {
			// ignore not found errors if the object is already gone
			return ctrl.Result{}, err
		}

		// 4. as last step remove the finalizer on the operator CR to finish delete
		controllerutil.RemoveFinalizer(instance, finalizerName)
		err = r.Client.Update(context.TODO(), instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("CR %s deleted", instance.Name))
		return ctrl.Result{}, nil
	}

	// First deploy the provisioning image (Apache) server
	provisionServer, op, err := r.provisionServerCreateOrUpdate(instance)

	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("BaremetalSet %s ProvisionServer successfully reconciled - operation: %s", instance.Name, string(op)))
		return ctrl.Result{}, nil
	}

	if provisionServer.Status.LocalImageURL == "" {
		r.Log.Info(fmt.Sprintf("BaremetalSet %s ProvisionServer local image URL not yet available, requeuing and waiting", instance.Name))
		return ctrl.Result{RequeueAfter: time.Second * 30}, err
	}

	// First we need the public SSH key, which should be stored in a secret
	hashes := []ospdirectorv1beta1.Hash{}
	sshSecret, secretHash, err := common.GetSecret(r, instance.Spec.DeploymentSSHSecret, instance.Namespace)
	if err != nil && errors.IsNotFound(err) {
		return ctrl.Result{RequeueAfter: time.Second * 20}, fmt.Errorf("DeploymentSSHSecret secret does not exist: %v", err)
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// TODO: Put hashes in BaremetalSet status?
	hashes = append(hashes, ospdirectorv1beta1.Hash{Name: sshSecret.Name, Hash: secretHash})

	// If BaremetalHosts status map is nil, create it
	if instance.Status.BaremetalHosts == nil {
		instance.Status.BaremetalHosts = map[string]ospdirectorv1beta1.BaremetalHostStatus{}
	}

	// Provision / deprovision requested replicas
	if err := r.ensureBaremetalHosts(instance, provisionServer, sshSecret); err != nil {
		return ctrl.Result{}, err
	}

	err = r.Client.Status().Update(context.TODO(), instance)

	if err != nil {
		r.Log.Error(err, "Failed to update CR status %v")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager - prepare controller for use with operator manager
func (r *BaremetalSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	openshiftMachineApiSecretsFn := handler.ToRequestsFunc(func(o handler.MapObject) []reconcile.Request {
		cc := mgr.GetClient()
		result := []reconcile.Request{}
		secret := &corev1.Secret{}
		key := client.ObjectKey{Namespace: o.Meta.GetNamespace(), Name: o.Meta.GetName()}
		err := cc.Get(context.Background(), key, secret)
		if err != nil && !errors.IsNotFound(err) {
			r.Log.Error(err, "Unable to retrieve Secret %v")
			return nil
		}

		label := secret.ObjectMeta.GetLabels()
		// verify object has ownerUIDLabelSelector
		if uid, ok := label[baremetalset.OwnerUIDLabelSelector]; ok {
			r.Log.Info(fmt.Sprintf("Secret object %s marked with OSP owner ref: %s", o.Meta.GetName(), uid))
			// return namespace and Name of CR
			name := client.ObjectKey{
				Namespace: label[baremetalset.OwnerNameSpaceLabelSelector],
				Name:      label[baremetalset.OwnerNameLabelSelector],
			}
			result = append(result, reconcile.Request{NamespacedName: name})
		}
		if len(result) > 0 {
			return result
		}
		return nil
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.BaremetalSet{}).
		Owns(&ospdirectorv1beta1.ProvisionServer{}).
		Watches(&source.Kind{Type: &corev1.Secret{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: openshiftMachineApiSecretsFn,
			}).
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

// Provision or deprovision BaremetalHost resources based on replica count
func (r *BaremetalSetReconciler) ensureBaremetalHosts(instance *ospdirectorv1beta1.BaremetalSet, provisionServer *ospdirectorv1beta1.ProvisionServer, sshSecret *corev1.Secret) error {
	// Get old BaremetalHosts status map
	oldBaremetalHostsStatusMap := instance.Status.BaremetalHosts

	// Deallocate existing BaremetalHosts to match the requested replica count, if necessary.
	// TODO: Add something to CR to allow user to specify which BaremetalHosts are removed
	for i := len(oldBaremetalHostsStatusMap); i > instance.Spec.Replicas; i-- {
		for oldBmhName, oldBmhStatus := range oldBaremetalHostsStatusMap {
			// Just delete the first one we find until the TODO from above is implemented
			err := r.baremetalHostDeprovision(instance, oldBmhName, oldBmhStatus.SecretName)

			if err != nil {
				return err
			}

			delete(oldBaremetalHostsStatusMap, oldBmhName)
			break
		}
	}

	// How many new BaremetalHost allocations do we need (if any)?
	newBmhsNeededCount := instance.Spec.Replicas - len(oldBaremetalHostsStatusMap)

	if newBmhsNeededCount <= 0 {
		// We're done
		r.Log.Info("All requested BaremetalHost replicas have been provisioned")
		return nil
	}

	// We have new replicas requested, so search for baremetalhosts that don't have consumerRef or Online set
	availableBaremetalHosts := []string{}
	baremetalHostsList := &metal3v1alpha1.BareMetalHostList{}
	listOpts := []client.ListOption{
		client.InNamespace("openshift-machine-api"),
	}
	err := r.Client.List(context.TODO(), baremetalHostsList, listOpts...)
	if err != nil {
		return err
	}

	for _, baremetalHost := range baremetalHostsList.Items {
		if baremetalHost.Spec.Online == true || baremetalHost.Spec.ConsumerRef != nil {
			continue
		}
		r.Log.Info(fmt.Sprintf("Available BaremetalHost: %s", baremetalHost.ObjectMeta.Name))
		availableBaremetalHosts = append(availableBaremetalHosts, baremetalHost.ObjectMeta.Name)
	}

	// If we can't satisfy the new requested replica count, explicitly state so
	if newBmhsNeededCount > len(availableBaremetalHosts) {
		r.Log.Info(fmt.Sprintf("WARNING: Unable to find %d requested BaremetalHost replicas (%d in use, %d available)", instance.Spec.Replicas, len(oldBaremetalHostsStatusMap), len(availableBaremetalHosts)))
	}

	// For each available BaremetalHost we update the reference to use our image
	// and set the user data to use our cloud-init secret.  Then we add the status
	// to store the BMH name, cloud-init secret name and management IP for the
	// particular worker
	for i := 0; i < len(availableBaremetalHosts); i++ {
		err := r.baremetalHostProvision(instance, availableBaremetalHosts[i], provisionServer.Status.LocalImageURL, sshSecret)

		if err != nil {
			return err
		}
	}

	return nil
}

// Provision a BaremetalHost via Metal3 (and create its bootstrapping secret)
func (r *BaremetalSetReconciler) baremetalHostProvision(instance *ospdirectorv1beta1.BaremetalSet, bmh string, localImageURL string, sshSecret *corev1.Secret) error {
	// Prepare cloudinit (create secret)
	sts := []common.Template{}
	_, network, err := net.ParseCIDR(instance.Spec.MgmtCIDR)

	if err != nil {
		return err
	}

	secretLabels := common.GetLabels(instance.Name, baremetalset.AppLabel)

	mgmtIP, err := findNewMgmtIP(instance)

	if err != nil {
		return err
	}

	if mgmtIP == "" {
		return fmt.Errorf("Unable to find free IP for CIDR %s", instance.Spec.MgmtCIDR)
	}

	templateParameters := make(map[string]string)
	templateParameters["AuthorizedKeys"] = strings.TrimSuffix(string(sshSecret.Data["authorized_keys"]), "\n")
	templateParameters["MgmtIp"] = mgmtIP
	templateParameters["MgmtInterface"] = instance.Spec.MgmtInterface
	templateParameters["MgmtNetwork"] = network.IP.String()
	templateParameters["MgmtNetmask"] = strings.Split(instance.Spec.MgmtCIDR, "/")[1]
	// TODO: Either generate the gateway from the network object or allow user specification via CR?

	secretName := fmt.Sprintf("%s-cloudinit-%s", instance.Name, bmh)

	st := common.Template{
		Name:           secretName,
		Namespace:      "openshift-machine-api",
		Type:           common.TemplateTypeConfig,
		InstanceType:   instance.Kind,
		AdditionalData: map[string]string{},
		Labels:         secretLabels,
		ConfigOptions:  templateParameters,
	}

	sts = append(sts, st)

	err = common.EnsureSecrets(r, instance, sts, &map[string]common.EnvSetter{})

	if err != nil {
		return err
	}

	// Provision the BaremetalHost
	foundBaremetalHost := &metal3v1alpha1.BareMetalHost{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: bmh, Namespace: "openshift-machine-api"}, foundBaremetalHost)

	if err != nil {
		return err
	}

	r.Log.Info(fmt.Sprintf("Allocating BaremetalHost: %s", bmh))
	foundBaremetalHost.Spec.Online = true
	foundBaremetalHost.Spec.ConsumerRef = &corev1.ObjectReference{Name: instance.Name, Kind: "BaremetalSet"}
	foundBaremetalHost.Spec.Image = &metal3v1alpha1.Image{
		URL:      localImageURL,
		Checksum: fmt.Sprintf("%s.md5sum", localImageURL),
	}
	foundBaremetalHost.Spec.UserData = &corev1.SecretReference{
		Name:      secretName,
		Namespace: "openshift-machine-api",
	}
	err = r.Client.Update(context.TODO(), foundBaremetalHost)
	if err != nil {
		return err
	}

	// Set status (add this BaremetalHost entry)
	instance.Status.BaremetalHosts[bmh] = ospdirectorv1beta1.BaremetalHostStatus{
		SecretName: secretName,
		MgmtIP:     mgmtIP,
	}

	return nil
}

// Deprovision a BaremetalHost via Metal3 (and delete its bootstrapping secret)
func (r *BaremetalSetReconciler) baremetalHostDeprovision(instance *ospdirectorv1beta1.BaremetalSet, bmh string, userDataSecret string) error {
	baremetalHost := &metal3v1alpha1.BareMetalHost{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: bmh, Namespace: "openshift-machine-api"}, baremetalHost)

	if err != nil {
		return err
	}

	r.Log.Info(fmt.Sprintf("Deallocating BaremetalHost: %s", bmh))
	baremetalHost.Spec.Online = false
	baremetalHost.Spec.ConsumerRef = nil
	baremetalHost.Spec.Image = nil
	baremetalHost.Spec.UserData = nil
	err = r.Client.Update(context.TODO(), baremetalHost)
	if err != nil {
		return err
	}

	// Also remove userdata secret
	secret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      userDataSecret,
			Namespace: "openshift-machine-api",
		},
	}
	err = r.Client.Delete(context.Background(), secret, &client.DeleteOptions{})
	if err != nil {
		return err
	}
	r.Log.Info(fmt.Sprintf("Secret deleted: name %s", secret.Name))

	// Set status (remove this BaremetalHost entry)
	delete(instance.Status.BaremetalHosts, bmh)

	return nil
}

// Deprovision all associated BaremetalHosts for this BaremetalSet via Metal3
func (r *BaremetalSetReconciler) baremetalHostCleanup(instance *ospdirectorv1beta1.BaremetalSet) error {
	if instance.Status.BaremetalHosts != nil {
		for bmhName, bmhStatus := range instance.Status.BaremetalHosts {
			err := r.baremetalHostDeprovision(instance, bmhName, bmhStatus.SecretName)

			if err != nil {
				return err
			}
		}
	}

	return nil
}

/* deleteOwnerRefLabeledObjects - cleans up namespaced objects outside the default namespace
   using the owner reference labels added.
   List of objects which get cleaned:
   - user-data secret, openshift-machine-api namespace
*/
func (r *BaremetalSetReconciler) deleteOwnerRefLabeledObjects(instance *ospdirectorv1beta1.BaremetalSet) error {
	labelSelectorMap := map[string]string{
		baremetalset.OwnerUIDLabelSelector:       string(instance.UID),
		baremetalset.OwnerNameSpaceLabelSelector: instance.Namespace,
		baremetalset.OwnerNameLabelSelector:      instance.Name,
	}

	// delete secrets in openshift-machine-api namespace
	secrets, err := common.GetSecrets(r, "openshift-machine-api", labelSelectorMap)
	if err != nil {
		return err
	}
	for idx := range secrets.Items {
		secret := &secrets.Items[idx]

		err = r.Client.Delete(context.Background(), secret, &client.DeleteOptions{})
		if err != nil {
			return err
		}
		r.Log.Info(fmt.Sprintf("Secret deleted: name %s - %s", secret.Name, secret.UID))
	}

	return nil
}

// Get an unused IP from the management network for the BaremetalSet
func findNewMgmtIP(instance *ospdirectorv1beta1.BaremetalSet) (string, error) {
	_, network, err := net.ParseCIDR(instance.Spec.MgmtCIDR)

	if err != nil {
		return "", err
	}

	for i := uint64(2); i <= cidr.AddressCount(network); i++ {
		mgmtIP, err := cidr.Host(network, int(i))

		if err != nil {
			return "", err
		}

		found := false

		for _, bmhStatus := range instance.Status.BaremetalHosts {
			if bmhStatus.MgmtIP == mgmtIP.String() {
				found = true
				break
			}
		}

		if !found {
			return mgmtIP.String(), nil
		}
	}

	return "", nil
}
