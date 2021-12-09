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
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/diff"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstackclient "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackclient"
	openstackipset "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackipset"
	openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
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
func (r *OpenStackClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, true)

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("OpenStackClient %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	envVars := make(map[string]common.EnvSetter)

	// check for required secrets
	if instance.Spec.DeploymentSSHSecret != "" {
		_, _, err = common.GetSecret(r, instance.Spec.DeploymentSSHSecret, instance.Namespace)
		if err != nil && k8s_errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Second * 20}, fmt.Errorf("DeploymentSSHSecret secret does not exist: %v", err)
		} else if err != nil {
			return ctrl.Result{}, err
		}
	}
	if instance.Spec.IdmSecret != "" {
		_, _, err = common.GetSecret(r, instance.Spec.IdmSecret, instance.Namespace)
		if err != nil && k8s_errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Second * 20}, fmt.Errorf("IdmSecret secret does not exist: %v", err)
		} else if err != nil {
			return ctrl.Result{}, err
		}
	}
	if instance.Status.OpenStackClientNetStatus == nil {
		instance.Status.OpenStackClientNetStatus = map[string]ospdirectorv1beta1.HostStatus{}
	}

	//
	// create hostname for the openstackclient
	//
	hostnameRef := instance.GetHostnames()
	hostnameDetails := common.Hostname{}
	if len(hostnameRef) == 0 {
		hostnameDetails = common.Hostname{
			Basename: instance.Name,
			Hostname: "",
			VIP:      false,
		}

		err = common.CreateOrGetHostname(instance, &hostnameDetails)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		for hostname := range hostnameRef {

			hostnameDetails = common.Hostname{
				Basename: instance.Name,
				Hostname: hostname,
				VIP:      false,
			}
		}
	}

	if _, ok := instance.Status.OpenStackClientNetStatus[hostnameDetails.Hostname]; !ok {
		instance.Status.OpenStackClientNetStatus[hostnameDetails.Hostname] = ospdirectorv1beta1.HostStatus{
			Hostname: hostnameDetails.Hostname,
			HostRef:  strings.ToLower(instance.Name),
		}
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			r.Log.Error(err, "Failed to update CR status %v")
			return ctrl.Result{}, err
		}
	}

	hostnameRefs := instance.GetHostnames()

	ipsetDetails := common.IPSet{
		Networks:            instance.Spec.Networks,
		Role:                fmt.Sprintf("%s%s", openstackclient.Role, instance.Name),
		HostCount:           openstackclient.Count,
		AddToPredictableIPs: false,
		HostNameRefs:        hostnameRefs,
	}
	ipset, op, err := openstackipset.OvercloudipsetCreateOrUpdate(r, instance, ipsetDetails)
	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("IPSet for %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	if len(ipset.Status.HostIPs) != openstackclient.Count {
		r.Log.Info(fmt.Sprintf("IPSet has not yet reached the required replicas %d", openstackclient.Count))
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	// get a copy of the current CR status
	currentStatus := instance.Status.DeepCopy()

	// update network status
	for _, netName := range instance.Spec.Networks {
		r.setNetStatus(instance, &hostnameDetails, netName, ipset.Status.HostIPs[hostnameDetails.Hostname].IPAddresses[netName])
	}

	// update the IPs for IPSet if status got updated
	actualStatus := instance.Status
	if !reflect.DeepEqual(currentStatus, &actualStatus) {
		r.Log.Info(fmt.Sprintf("OpenStackClient network status for Hostname: %s - %s", instance.Status.OpenStackClientNetStatus[hostnameDetails.Hostname].Hostname, diff.ObjectReflectDiff(currentStatus, &actualStatus)))

		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			r.Log.Error(err, "Failed to update CR status %v")
			return ctrl.Result{}, err
		}
	}
	// verify that NetworkAttachmentDefinition for each configured network exist
	nadMap, err := common.GetAllNetworkAttachmentDefinitions(r, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, netNameLower := range instance.Spec.Networks {
		// get network with name_lower label
		network, err := openstacknet.GetOpenStackNetWithLabel(
			r,
			instance.Namespace,
			map[string]string{
				openstacknet.SubNetNameLabelSelector: netNameLower,
			},
		)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				r.Log.Info(fmt.Sprintf("OpenStackNet with NameLower %s not found!", netNameLower))
				continue
			}
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}

		if _, ok := nadMap[network.Name]; !ok {
			r.Log.Error(err, fmt.Sprintf("NetworkAttachmentDefinition for network %s does not yet exist.  Reconciling again in 10 seconds", netNameLower))
			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}
	}

	cmLabels := common.GetLabels(instance, openstackclient.AppLabel, map[string]string{})

	// create cm holding deployment script and render deployment script.
	cms := []common.Template{
		// ScriptsConfigMap
		{
			Name:               "openstackclient-sh",
			Namespace:          instance.Namespace,
			Type:               common.TemplateTypeScripts,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{},
			ConfigOptions:      make(map[string]interface{}),
			Labels:             cmLabels,
		},
	}
	err = common.EnsureConfigMaps(r, instance, cms, &envVars)
	if err != nil && k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// PVCs
	// volume to presistent store /etc/hosts where entries get added by tripleo deploy
	pvcDetails := common.Pvc{
		Name:         fmt.Sprintf("%s-hosts", instance.Name),
		Namespace:    instance.Namespace,
		Size:         openstackclient.HostsPersistentStorageSize,
		Labels:       common.GetLabels(instance, openstackclient.AppLabel, map[string]string{}),
		StorageClass: instance.Spec.StorageClass,
		AccessMode: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},
	}

	pvc, op, err := common.CreateOrUpdatePvc(r, instance, &pvcDetails)
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Openstackclient /etc/hosts PVC %s Updated", pvc.Name))
	}

	pvcDetails = common.Pvc{
		Name:         fmt.Sprintf("%s-cloud-admin", instance.Name),
		Namespace:    instance.Namespace,
		Size:         openstackclient.CloudAdminPersistentStorageSize,
		Labels:       common.GetLabels(instance, openstackclient.AppLabel, map[string]string{}),
		StorageClass: instance.Spec.StorageClass,
		AccessMode: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},
	}

	pvc, op, err = common.CreateOrUpdatePvc(r, instance, &pvcDetails)
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Openstackclient PVC /home/cloud-admin %s Updated", pvc.Name))
	}

	pvcDetails = common.Pvc{
		Name:         fmt.Sprintf("%s-kolla-src", instance.Name),
		Namespace:    instance.Namespace,
		Size:         openstackclient.KollaSrcPersistentStorageSize,
		Labels:       common.GetLabels(instance, openstackclient.AppLabel, map[string]string{}),
		StorageClass: instance.Spec.StorageClass,
		AccessMode: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},
	}

	pvc, op, err = common.CreateOrUpdatePvc(r, instance, &pvcDetails)
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Openstackclient kolla-src PVC %s Updated", pvc.Name))
	}

	// Create or update the pod object
	op, err = r.podCreateOrUpdate(instance, &hostnameDetails, envVars)
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Pod %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}
	return ctrl.Result{}, nil
}

// SetupWithManager -
func (r *OpenStackClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackClient{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *OpenStackClientReconciler) setNetStatus(instance *ospdirectorv1beta1.OpenStackClient, hostnameDetails *common.Hostname, netName string, ipaddress string) {

	// If OpenStackClient status map is nil, create it
	if instance.Status.OpenStackClientNetStatus == nil {
		instance.Status.OpenStackClientNetStatus = map[string]ospdirectorv1beta1.HostStatus{}
	}

	// Set network information status
	if instance.Status.OpenStackClientNetStatus[hostnameDetails.Hostname].IPAddresses == nil {
		instance.Status.OpenStackClientNetStatus[hostnameDetails.Hostname] = ospdirectorv1beta1.HostStatus{
			Hostname: hostnameDetails.Hostname,
			HostRef:  strings.ToLower(instance.Name),
			IPAddresses: map[string]string{
				netName: ipaddress,
			},
		}
	} else {
		status := instance.Status.OpenStackClientNetStatus[hostnameDetails.Hostname]
		status.HostRef = strings.ToLower(instance.Name)
		status.IPAddresses[netName] = ipaddress
		instance.Status.OpenStackClientNetStatus[hostnameDetails.Hostname] = status
	}
}

func (r *OpenStackClientReconciler) podCreateOrUpdate(instance *ospdirectorv1beta1.OpenStackClient, hostnameDetails *common.Hostname, envVars map[string]common.EnvSetter) (controllerutil.OperationResult, error) {
	var terminationGracePeriodSeconds int64 = 0

	runAsUser := int64(instance.Spec.RunUID)
	runAsGroup := int64(instance.Spec.RunGID)

	// Get volumes
	initVolumeMounts := openstackclient.GetInitVolumeMounts(instance)
	volumeMounts := openstackclient.GetVolumeMounts(instance)
	volumes := openstackclient.GetVolumes(instance)

	envVars["KOLLA_CONFIG_STRATEGY"] = common.EnvValue("COPY_ALWAYS")

	if instance.Spec.CloudName != "" {
		envVars["OS_CLOUD"] = common.EnvValue(instance.Spec.CloudName)
	}

	if instance.Spec.DomainName != "" {
		envVars["FQDN"] = common.EnvValue(instance.Name + "." + instance.Spec.DomainName)

	}

	// create k8s.v1.cni.cncf.io/networks network annotation to attach OpenStackClient to networks set in instance.Spec.Networks
	annotation := "["
	for id, netName := range instance.Spec.Networks {
		// get network with name_lower label
		network, err := openstacknet.GetOpenStackNetWithLabel(
			r,
			instance.Namespace,
			map[string]string{
				openstacknet.SubNetNameLabelSelector: netName,
			},
		)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				r.Log.Info(fmt.Sprintf("OpenStackNet with NameLower %s not found!", netName))
				continue
			}
			// Error reading the object - requeue the request.
			return "", err
		}

		nad := fmt.Sprintf("%s-static", network.Name)
		annotation += fmt.Sprintf("{\"name\": \"%s\", \"namespace\": \"%s\", \"ips\": [\"%s\"]}", nad, instance.Namespace, instance.Status.OpenStackClientNetStatus[hostnameDetails.Hostname].IPAddresses[netName])
		if id < len(instance.Spec.Networks)-1 {
			annotation += ", "
		}
	}
	annotation += "]"

	env := common.MergeEnvs([]corev1.EnvVar{}, envVars)

	if instance.Spec.GitSecret != "" {
		env = common.MergeEnvs([]corev1.EnvVar{
			{
				Name: "GIT_URL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: instance.Spec.GitSecret,
						},
						Key: "git_url",
					},
				},
			},
		}, envVars)
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Annotations: map[string]string{
				"k8s.v1.cni.cncf.io/networks": annotation,
			},
		},
	}
	pod.Spec = corev1.PodSpec{
		ServiceAccountName: openstackclient.ServiceAccount,
		SecurityContext: &corev1.PodSecurityContext{
			RunAsUser:  &runAsUser,
			RunAsGroup: &runAsGroup,
			FSGroup:    &runAsGroup,
		},
		TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
		Volumes:                       volumes,
		Containers: []corev1.Container{
			{
				Name:            "openstackclient",
				Image:           instance.Spec.ImageURL,
				ImagePullPolicy: corev1.PullAlways,
				Env:             env,
				VolumeMounts:    volumeMounts,
			},
		},
	}

	if len(instance.Spec.DNSServers) != 0 {
		pod.Spec.DNSPolicy = corev1.DNSNone
		pod.Spec.DNSConfig = &corev1.PodDNSConfig{
			Nameservers: instance.Spec.DNSServers,
		}
		if len(instance.Spec.DNSSearchDomains) != 0 {
			pod.Spec.DNSConfig.Searches = instance.Spec.DNSSearchDomains
		}
	}

	initEnv := env
	if instance.Spec.IdmSecret != "" {
		idmEnv := []corev1.EnvVar{
			{
				Name: "IPA_SERVER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: instance.Spec.IdmSecret,
						},
						Key: "IdMServer",
					},
				},
			},
			{
				Name: "IPA_SERVER_USER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: instance.Spec.IdmSecret,
						},
						Key: "IdMAdminUser",
					},
				},
			},
			{
				Name: "IPA_SERVER_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: instance.Spec.IdmSecret,
						},
						Key: "IdMAdminPassword",
					},
				},
			},
			{
				Name: "IPA_REALM",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: instance.Spec.IdmSecret,
						},
						Key: "IdMRealm",
					},
				},
			},
			{
				Name:  "IPA_DOMAIN",
				Value: instance.Spec.DomainName,
			},
		}
		initEnv = append(initEnv, idmEnv...)
	}

	initContainerDetails := []openstackclient.InitContainer{
		{
			ContainerImage: instance.Spec.ImageURL,
			Env:            initEnv,
			Privileged:     false,
			VolumeMounts:   initVolumeMounts,
		},
	}

	pod.Spec.InitContainers = openstackclient.GetInitContainers(initContainerDetails)

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, pod, func() error {
		pod.Spec.Containers[0].Env = common.MergeEnvs(pod.Spec.Containers[0].Env, envVars)

		// labels
		common.InitMap(&pod.Labels)
		for k, v := range common.GetLabels(instance, openstackclient.AppLabel, map[string]string{}) {
			pod.Labels[k] = v
		}

		err := controllerutil.SetControllerReference(instance, pod, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil && k8s_errors.IsInvalid(err) {
		// Delete pod when an unsupported change was requested, like
		// e.g. additional controller VM got up. We just re-create the
		// openstackclient pod
		r.Log.Info(fmt.Sprintf("openstackclient pod deleted due to spec change %v", err))
		if err := r.Client.Delete(context.TODO(), pod); err != nil {
			return op, err
		}

		r.Log.Info("openstackclient pod deleted due to spec change")
		return controllerutil.OperationResultUpdated, nil
	}
	return op, err
}
