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
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	bindatautil "github.com/openstack-k8s-operators/osp-director-operator/pkg/bindata_util"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	vmset "github.com/openstack-k8s-operators/osp-director-operator/pkg/vmset"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	//virtv1 "kubevirt.io/client-go/api/v1"
	//cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

// VMSetReconciler reconciles a VMSet object
type VMSetReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *VMSetReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *VMSetReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *VMSetReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *VMSetReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=vmsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=vmsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=vmsets/finalizers,verbs=update
// +kubebuilder:rbac:groups=osp-director.openstack.org,namespace=openstack,resources=deployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=template.openshift.io,namespace=openstack,resources=securitycontextconstraints,resourceNames=privileged,verbs=use
// +kubebuilder:rbac:groups=core,resources=pods;persistentvolumeclaims;events;configmaps;secrets,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=cdi.kubevirt.io,namespace=openstack,resources=datavolumes,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,namespace=openstack,resources=network-attachment-definitions,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=kubevirt.io,namespace=openstack,resources=virtualmachines,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=nmstate.io,resources=nodenetworkconfigurationpolicies,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=overcloudipsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=overcloudipsets/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=overcloudipsets/status,verbs=get;update;patch

// Reconcile - controller VMs
func (r *VMSetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("vmset", req.NamespacedName)

	// Fetch the controller VM instance
	instance := &ospdirectorv1beta1.VMSet{}
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
	sshSecret, _, err := common.GetSecret(r, instance.Spec.DeploymentSSHSecret, instance.Namespace)
	if err != nil && errors.IsNotFound(err) {
		return ctrl.Result{RequeueAfter: time.Second * 20}, fmt.Errorf("DeploymentSSHSecret secret does not exist: %v", err)
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Create/update secrets from templates
	secretLabels := common.GetLabels(instance.Name, vmset.AppLabel)

	templateParameters := make(map[string]interface{})
	templateParameters["AuthorizedKeys"] = strings.TrimSuffix(string(sshSecret.Data["authorized_keys"]), "\n")

	if instance.Spec.PasswordSecret != "" {
		// check if specified password secret exists before creating the controlplane
		passwordSecret, _, err := common.GetSecret(r, instance.Spec.PasswordSecret, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("PasswordSecret %s not found but specified in CR, next reconcile in 30s", instance.Spec.PasswordSecret)
			}
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}
		// use same NodeRootPassword paremater as tripleo have
		if len(passwordSecret.Data["NodeRootPassword"]) > 0 {
			templateParameters["NodeRootPassword"] = string(passwordSecret.Data["NodeRootPassword"])
		}
	}

	cloudinit := []common.Template{
		// CloudInitSecret
		{
			Name:           fmt.Sprintf("%s-cloudinit", instance.Name),
			Namespace:      instance.Namespace,
			Type:           common.TemplateTypeNone,
			InstanceType:   instance.Kind,
			AdditionalData: map[string]string{"userdata": "/vmset/cloudinit/userdata"},
			Labels:         secretLabels,
			ConfigOptions:  templateParameters,
		},
		// TODO: mschuppert should we have this?
		// CloudInitCustomSecret
		//{
		//	Name:      fmt.Sprintf("%s-cloudinit-custom", instance.Name),
		//	Namespace: instance.Namespace,
		//	Type:      common.TemplateTypeCustom,
		//	Labels:    secretLabels,
		//},
	}

	err = common.EnsureSecrets(r, instance, cloudinit, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	// If VMHosts status map is nil, create it
	if instance.Status.VMHosts == nil {
		instance.Status.VMHosts = map[string]ospdirectorv1beta1.VMHostStatus{}
	}

	ipsetDetails := common.IPSet{
		Networks:            instance.Spec.Networks,
		Role:                instance.Spec.Role,
		HostCount:           instance.Spec.VMCount,
		AddToPredictableIPs: true,
	}
	ipset, op, err := common.OvercloudipsetCreateOrUpdate(r, instance, ipsetDetails)
	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("IPSet for %s successfully reconciled - operation: %s", instance.Name, string(op)))
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	if len(ipset.Status.HostIPs) != instance.Spec.VMCount {
		r.Log.Info(fmt.Sprintf("IPSet has not yet reached the required replicas %d", instance.Spec.VMCount))
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	controllerDetails := map[string]ospdirectorv1beta1.Host{}

	// Generate the Contoller networkdata secrets
	for i := 0; i < instance.Spec.VMCount; i++ {
		// TODO: multi nic support with bindata template

		hostKey := fmt.Sprintf("%s-%d", strings.ToLower(instance.Spec.Role), i)
		hostname, err := common.CreateOrGetHostname(instance, hostKey, instance.Spec.Role)
		r.Log.Info(fmt.Sprintf("VMSet VM hostname set to %s", hostname))
		if err != nil {
			return ctrl.Result{}, err
		}

		networkDataSecretName := fmt.Sprintf("%s-%s-networkdata", instance.Name, hostname)

		controllerDetails[hostname] = ospdirectorv1beta1.Host{
			Hostname:          hostname,
			DomainName:        hostname,
			DomainNameUniq:    fmt.Sprintf("%s-%s", hostname, instance.UID[0:4]),
			IPAddress:         ipset.Status.HostIPs[hostKey].IPAddresses["ctlplane"],
			NetworkDataSecret: networkDataSecretName,
		}

		templateParameters["ControllerIP"] = controllerDetails[hostname].IPAddress
		gateway := ipset.Status.Networks["ctlplane"].Gateway
		if gateway != "" {
			if strings.Contains(gateway, ":") {
				templateParameters["Gateway"] = fmt.Sprintf("gateway6: %s", gateway)
			} else {
				templateParameters["Gateway"] = fmt.Sprintf("gateway4: %s", gateway)
			}
		}
		networkdata := []common.Template{
			{
				Name:           fmt.Sprintf("%s-%s-networkdata", instance.Name, hostname),
				Namespace:      instance.Namespace,
				Type:           common.TemplateTypeNone,
				InstanceType:   instance.Kind,
				AdditionalData: map[string]string{"networkdata": "/vmset/cloudinit/networkdata"},
				Labels:         secretLabels,
				ConfigOptions:  templateParameters,
			},
		}

		err = common.EnsureSecrets(r, instance, networkdata, &envVars)
		if err != nil {
			return ctrl.Result{}, nil
		}

		r.setVMHostStatus(instance, hostname, ipset.Status.HostIPs[hostKey].IPAddresses["ctlplane"])
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Create base image, controller disks get cloned from this
	if instance.Spec.BaseImageVolumeName == "" {
		err = r.cdiCreateBaseDisk(instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Create network config
	err = r.networkCreateAttachmentDefinition(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// wait for the base image conversion job to be finished before we create the VMs
	// we check the pvc for the base image:
	// - import in progress
	//   annotations -> cdi.kubevirt.io/storage.pod.phase: Running
	// - when import done
	//   annotations -> cdi.kubevirt.io/storage.pod.phase: Succeeded
	pvc := &corev1.PersistentVolumeClaim{}
	baseImageName := fmt.Sprintf("osp-vmset-baseimage-%s", instance.UID[0:4])
	if instance.Spec.BaseImageVolumeName != "" {
		baseImageName = instance.Spec.BaseImageVolumeName
	}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: baseImageName, Namespace: instance.Namespace}, pvc)
	if err != nil && errors.IsNotFound(err) {
		r.Log.Info(fmt.Sprintf("PersistentVolumeClaim %s not found reconcil in 10s", baseImageName))
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}
	// mschuppert - do we need to consider any other status
	switch pvc.Annotations["cdi.kubevirt.io/storage.pod.phase"] {
	case "Running":
		r.Log.Info(fmt.Sprintf("VM base image %s creation still in running state, reconcile in 2min", baseImageName))
		instance.Status.BaseImageDVReady = false
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	case "Succeeded":
		instance.Status.BaseImageDVReady = true
	}

	// Create controller VM objects
	if instance.Status.BaseImageDVReady {
		// Generate the Contoller networkdata secrets
		for _, ctl := range controllerDetails {
			err = r.vmCreateInstance(instance, envVars, &ctl)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		r.Log.Info("BaseImageDV is not Ready!")
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	return ctrl.Result{}, nil
}

func (r *VMSetReconciler) setVMHostStatus(instance *ospdirectorv1beta1.VMSet, hostname string, ipaddress string) {
	// Set status for vmHosts
	instance.Status.VMHosts[hostname] = ospdirectorv1beta1.VMHostStatus{
		Hostname:  hostname,
		IPAddress: ipaddress,
	}
}

// SetupWithManager -
func (r *VMSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO: Myabe use filtering functions here since some resource permissions
	// are now cluster-scoped?
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.VMSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

/* mpryc: golangci - comment out unused function
func setDefaults(instance *ospdirectorv1beta1.VMSet) {
	if instance.Spec.VMCount < 1 {
		instance.Spec.VMCount = 1
	}
}
*/

func (r *VMSetReconciler) getRenderData(instance *ospdirectorv1beta1.VMSet) (*bindatautil.RenderData, error) {
	data := bindatautil.MakeRenderData()
	// Base image used to clone the controller VM images from
	// adding first 5 char from instance.UID as identifier
	if instance.Spec.BaseImageVolumeName == "" {
		data.Data["BaseImageVolumeName"] = fmt.Sprintf("osp-vmset-baseimage-%s", instance.UID[0:4])
	} else {
		data.Data["BaseImageVolumeName"] = instance.Spec.BaseImageVolumeName
	}
	data.Data["BaseImageURL"] = instance.Spec.BaseImageURL
	data.Data["DiskSize"] = instance.Spec.DiskSize
	data.Data["Namespace"] = instance.Namespace
	data.Data["Cores"] = instance.Spec.Cores
	data.Data["Memory"] = instance.Spec.Memory
	data.Data["StorageClass"] = instance.Spec.StorageClass
	data.Data["Network"] = instance.Spec.OSPNetwork.Name
	data.Data["BridgeName"] = instance.Spec.OSPNetwork.BridgeName
	data.Data["DesiredState"] = instance.Spec.OSPNetwork.DesiredState.String()

	// get deployment user ssh pub key from Spec.DeploymentSSHSecret
	secret, _, err := common.GetSecret(r, instance.Spec.DeploymentSSHSecret, instance.Namespace)
	if err != nil {
		r.Log.Info("DeploymentSSHSecret secret not found!!")
		return nil, err
	}
	data.Data["AuthorizedKeys"] = secret.Data["authorized_keys"]

	// get deployment userdata from secret
	userdataSecret := fmt.Sprintf("%s-cloudinit", instance.Name)
	secret, _, err = common.GetSecret(r, userdataSecret, instance.Namespace)
	if err != nil {
		r.Log.Info("CloudInit userdata secret not found!!")
		return nil, err
	}
	data.Data["UserDataSecret"] = secret.Name

	return &data, nil
}

func (r *VMSetReconciler) networkCreateAttachmentDefinition(instance *ospdirectorv1beta1.VMSet) error {
	data, err := r.getRenderData(instance)
	if err != nil {
		return err
	}

	objs := []*uns.Unstructured{}
	oref := metav1.NewControllerRef(instance, instance.GroupVersionKind())

	// Labels for all objects
	labelSelector := map[string]string{
		OwnerUIDLabelSelector:       string(instance.UID),
		OwnerNameSpaceLabelSelector: instance.Namespace,
		OwnerNameLabelSelector:      instance.Name,
	}
	for k, v := range common.GetLabels(instance.Name, vmset.AppLabel) {
		labelSelector[k] = v
	}

	// Generate the Network Definition objects
	manifests, err := bindatautil.RenderDir(filepath.Join(ManifestPath, "network"), data)
	if err != nil {
		r.Log.Error(err, "Failed to render network manifests : %v")
		return err
	}
	objs = append(objs, manifests...)

	// Apply the objects to the cluster
	for _, obj := range objs {
		// Set owner reference on objects in the same namespace as the operator
		if obj.GetNamespace() == instance.Namespace {
			obj.SetOwnerReferences([]metav1.OwnerReference{*oref})
		}
		// merge owner ref label into labels on the objects
		obj.SetLabels(labels.Merge(obj.GetLabels(), labelSelector))

		if err := bindatautil.ApplyObject(context.TODO(), r.Client, obj); err != nil {
			r.Log.Error(err, "Failed to apply objects")
			return err
		}
	}

	return nil
}

func (r *VMSetReconciler) cdiCreateBaseDisk(instance *ospdirectorv1beta1.VMSet) error {
	data, err := r.getRenderData(instance)
	if err != nil {
		return err
	}

	objs := []*uns.Unstructured{}
	oref := metav1.NewControllerRef(instance, instance.GroupVersionKind())

	// Labels for all objects
	labelSelector := map[string]string{
		OwnerUIDLabelSelector:       string(instance.UID),
		OwnerNameSpaceLabelSelector: instance.Namespace,
		OwnerNameLabelSelector:      instance.Name,
	}
	for k, v := range common.GetLabels(instance.Name, vmset.AppLabel) {
		labelSelector[k] = v
	}

	// 01 - create root base volume on StorageClass
	manifests, err := bindatautil.RenderDir(filepath.Join(ManifestPath, "cdi"), data)
	if err != nil {
		return err
	}
	objs = append(objs, manifests...)

	// Apply the objects to the cluster
	for _, obj := range objs {
		// Set owner reference on objects in the same namespace as the operator
		if obj.GetNamespace() == instance.Namespace {
			obj.SetOwnerReferences([]metav1.OwnerReference{*oref})
		}
		// merge owner ref label into labels on the objects
		obj.SetLabels(labels.Merge(obj.GetLabels(), labelSelector))

		if err := bindatautil.ApplyObject(context.TODO(), r.Client, obj); err != nil {
			r.Log.Error(err, "Failed to apply objects")
			return err
		}
	}

	return nil
}

func (r *VMSetReconciler) vmCreateInstance(instance *ospdirectorv1beta1.VMSet, envVars map[string]common.EnvSetter, ctl *ospdirectorv1beta1.Host) error {
	data, err := r.getRenderData(instance)
	if err != nil {
		return err
	}

	objs := []*uns.Unstructured{}
	oref := metav1.NewControllerRef(instance, instance.GroupVersionKind())

	// Labels for all objects
	labelSelector := map[string]string{
		OwnerUIDLabelSelector:       string(instance.UID),
		OwnerNameSpaceLabelSelector: instance.Namespace,
		OwnerNameLabelSelector:      instance.Name,
	}
	for k, v := range common.GetLabels(instance.Name, vmset.AppLabel) {
		labelSelector[k] = v
	}

	// Generate the Contoller VM object
	data.Data["DomainName"] = ctl.DomainName
	data.Data["DomainNameUniq"] = ctl.DomainNameUniq
	data.Data["NetworkDataSecret"] = ctl.NetworkDataSecret

	manifests, err := bindatautil.RenderDir(filepath.Join(ManifestPath, "virtualmachine"), data)
	if err != nil {
		r.Log.Error(err, "Failed to render virtualmachine manifests : %v")
		return err
	}
	objs = append(objs, manifests...)

	// Apply the objects to the cluster
	for _, obj := range objs {
		// Set owner reference on objects in the same namespace as the operator
		if obj.GetNamespace() == instance.Namespace {
			obj.SetOwnerReferences([]metav1.OwnerReference{*oref})
		}
		// merge owner ref label into labels on the objects
		obj.SetLabels(labels.Merge(obj.GetLabels(), labelSelector))

		//Todo: mschuppert log vm definition when debug?
		r.Log.Info(fmt.Sprintf("/n%s/n", obj))
		if err := bindatautil.ApplyObject(context.TODO(), r.Client, obj); err != nil {
			r.Log.Error(err, "Failed to apply objects")
			return err
		}
	}

	return nil
}
