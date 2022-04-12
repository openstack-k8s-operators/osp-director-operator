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

package shared

//
// Common
//

const (
	//
	// condition types
	//

	// CommonCondTypeEmpty - special state for 0 requested resources and 0 already provisioned
	CommonCondTypeEmpty ConditionType = "Empty"
	// CommonCondTypeWaiting - something is causing the CR to wait
	CommonCondTypeWaiting ConditionType = "Waiting"
	// CommonCondTypeProvisioning - one or more resoources are provisioning
	CommonCondTypeProvisioning ConditionType = "Provisioning"
	// CommonCondTypeProvisioned - the requested resource count has been satisfied
	CommonCondTypeProvisioned ConditionType = "Provisioned"
	// CommonCondTypeDeprovisioning - one or more resources are deprovisioning
	CommonCondTypeDeprovisioning ConditionType = "Deprovisioning"
	// CommonCondTypeError - general catch-all for actual errors
	CommonCondTypeError ConditionType = "Error"
	// CommonCondTypeCreated - general resource created
	CommonCondTypeCreated ConditionType = "Created"

	//
	// condition reasones
	//

	// CommonCondReasonInit - new resource set to reason Init
	CommonCondReasonInit ConditionReason = "CommonInit"
	// CommonCondReasonDeploymentSecretMissing - deployment secret does not exist
	CommonCondReasonDeploymentSecretMissing ConditionReason = "DeploymentSecretMissing"
	// CommonCondReasonDeploymentSecretError - deployment secret error
	CommonCondReasonDeploymentSecretError ConditionReason = "DeploymentSecretError"
	// CommonCondReasonSecretMissing - secret does not exist
	CommonCondReasonSecretMissing ConditionReason = "SecretMissing"
	// CommonCondReasonSecretError - secret error
	CommonCondReasonSecretError ConditionReason = "SecretError"
	// CommonCondReasonSecretDeleteError - secret deletion error
	CommonCondReasonSecretDeleteError ConditionReason = "SecretDeleteError"
	// CommonCondReasonConfigMapMissing - config map does not exist
	CommonCondReasonConfigMapMissing ConditionReason = "ConfigMapMissing"
	// CommonCondReasonConfigMapError - config map error
	CommonCondReasonConfigMapError ConditionReason = "ConfigMapError"
	// CommonCondReasonIdmSecretMissing - idm secret does not exist
	CommonCondReasonIdmSecretMissing ConditionReason = "IdmSecretMissing"
	// CommonCondReasonIdmSecretError - idm secret error
	CommonCondReasonIdmSecretError ConditionReason = "IdmSecretError"
	// CommonCondReasonCAConfigMapMissing - ca config map does not exist
	CommonCondReasonCAConfigMapMissing ConditionReason = "CAConfigMapMissing"
	// CommonCondReasonCAConfigMapError - ca config map error
	CommonCondReasonCAConfigMapError ConditionReason = "CAConfigMapError"
	// CommonCondReasonNewHostnameError - error creating new hostname
	CommonCondReasonNewHostnameError ConditionReason = "NewHostnameError"
	// CommonCondReasonCRStatusUpdateError - error updating CR status
	CommonCondReasonCRStatusUpdateError ConditionReason = "CRStatusUpdateError"
	// CommonCondReasonNNCPError - NNCP error
	CommonCondReasonNNCPError ConditionReason = "NNCPError"
	// CommonCondReasonOSNetNotFound - openstack network not found
	CommonCondReasonOSNetNotFound ConditionReason = "OSNetNotFound"
	// CommonCondReasonOSNetError - openstack network error
	CommonCondReasonOSNetError ConditionReason = "OSNetError"
	// CommonCondReasonOSNetWaiting - openstack network waiting
	CommonCondReasonOSNetWaiting ConditionReason = "OSNetWaiting"
	// CommonCondReasonOSNetAvailable - openstack networks available
	CommonCondReasonOSNetAvailable ConditionReason = "OSNetAvailable"
	// CommonCondReasonControllerReferenceError - error set controller reference on object
	CommonCondReasonControllerReferenceError ConditionReason = "ControllerReferenceError"
	// CommonCondReasonOwnerRefLabeledObjectsDeleteError - error deleting object using OwnerRef label
	CommonCondReasonOwnerRefLabeledObjectsDeleteError ConditionReason = "OwnerRefLabeledObjectsDeleteError"
	// CommonCondReasonRemoveFinalizerError - error removing finalizer from object
	CommonCondReasonRemoveFinalizerError ConditionReason = "RemoveFinalizerError"
	// CommonCondReasonAddRefLabelError - error adding reference label
	CommonCondReasonAddRefLabelError ConditionReason = "AddRefLabelError"
	// CommonCondReasonAddOSNetLabelError - error adding osnet labels
	CommonCondReasonAddOSNetLabelError ConditionReason = "AddOSNetLabelError"
	// CommonCondReasonCIDRParseError - could not parse CIDR
	CommonCondReasonCIDRParseError ConditionReason = "CIDRParseError"
	// CommonCondReasonServiceNotFound - service not found
	CommonCondReasonServiceNotFound ConditionReason = "ServiceNotFound"
)

//
// Client
//
const (
	//
	// condition reasones
	//

	// OsClientCondReasonError - error creating openstackclient
	OsClientCondReasonError ConditionReason = "OpenStackClientError"
	// OsClientCondReasonProvisioned - pod created
	OsClientCondReasonProvisioned ConditionReason = "OpenStackClientProvisioned"
	// OsClientCondReasonCreated - created openstackclient
	OsClientCondReasonCreated ConditionReason = "OpenStackClientCreated"
	// OsClientCondReasonPVCError - error creating pvc
	OsClientCondReasonPVCError ConditionReason = "PVCError"
	// OsClientCondReasonPVCProvisioned - pvcs provisioned
	OsClientCondReasonPVCProvisioned ConditionReason = "PVCProvisioned"
	// OsClientCondReasonPodError - error creating pod
	OsClientCondReasonPodError ConditionReason = "PodError"
	// OsClientCondReasonPodProvisioned - pod created
	OsClientCondReasonPodProvisioned ConditionReason = "OpenStackClientPodProvisioned"
	// OsClientCondReasonPodDeleted - pod deleted
	OsClientCondReasonPodDeleted ConditionReason = "OpenStackClientPodDeleted"
	// OsClientCondReasonPodDeleteError - pod delete error
	OsClientCondReasonPodDeleteError ConditionReason = "PodDeleteError"
	// OsClientCondReasonPodMissing - openstackclient pod missing
	OsClientCondReasonPodMissing ConditionReason = "OpenStackClientPodMissing"
)

//
// ConfigGenerator
//
const (
	//
	// condition types
	//

	// ConfigGeneratorCondTypeWaiting - the config generator is blocked by prerequisite objects
	ConfigGeneratorCondTypeWaiting ConditionType = "Waiting"
	// ConfigGeneratorCondTypeInitializing - the config generator is preparing to execute
	ConfigGeneratorCondTypeInitializing ConditionType = "Initializing"
	// ConfigGeneratorCondTypeGenerating - the config generator is executing
	ConfigGeneratorCondTypeGenerating ConditionType = "Generating"
	// ConfigGeneratorCondTypeFinished - the config generation has finished executing
	ConfigGeneratorCondTypeFinished ConditionType = "Finished"
	// ConfigGeneratorCondTypeError - the config generation hit a generic error
	ConfigGeneratorCondTypeError ConditionType = "Error"

	//
	// condition reasones
	//

	// ConfigGeneratorCondReasonCMUpdated - configmap updated
	ConfigGeneratorCondReasonCMUpdated ConditionReason = "ConfigMapUpdated"
	// ConfigGeneratorCondReasonCMNotFound - configmap not found
	ConfigGeneratorCondReasonCMNotFound ConditionReason = "ConfigMapNotFound"
	// ConfigGeneratorCondReasonCMCreateError - error creating/update CM
	ConfigGeneratorCondReasonCMCreateError ConditionReason = "ConfigMapCreateError"
	// ConfigGeneratorCondReasonCMHashError - error creating/update CM
	ConfigGeneratorCondReasonCMHashError ConditionReason = "ConfigMapHashError"
	// ConfigGeneratorCondReasonCMHashChanged - cm hash changed
	ConfigGeneratorCondReasonCMHashChanged ConditionReason = "ConfigMapHashChanged"
	// ConfigGeneratorCondReasonCustomRolesNotFound - custom roles file not found
	ConfigGeneratorCondReasonCustomRolesNotFound ConditionReason = "CustomRolesNotFound"
	// ConfigGeneratorCondReasonVMInstanceList - VM instance list error
	ConfigGeneratorCondReasonVMInstanceList ConditionReason = "VMInstanceListError"
	// ConfigGeneratorCondReasonFencingTemplateError - Error creating fencing config parameters
	ConfigGeneratorCondReasonFencingTemplateError ConditionReason = "FencingTemplateError"
	// ConfigGeneratorCondReasonEphemeralHeatUpdated - Ephemeral heat created/updated
	ConfigGeneratorCondReasonEphemeralHeatUpdated ConditionReason = "EphemeralHeatUpdated"
	// ConfigGeneratorCondReasonEphemeralHeatLaunch - Ephemeral heat to launch
	ConfigGeneratorCondReasonEphemeralHeatLaunch ConditionReason = "EphemeralHeatLaunch"
	// ConfigGeneratorCondReasonEphemeralHeatDelete - Ephemeral heat delete
	ConfigGeneratorCondReasonEphemeralHeatDelete ConditionReason = "EphemeralHeatDelete"
	// ConfigGeneratorCondReasonExportFailed - Export of Ctlplane Heat Parameters Failed
	ConfigGeneratorCondReasonExportFailed ConditionReason = "CtlplaneExportFailed"
	// ConfigGeneratorCondReasonJobCreated - created job
	ConfigGeneratorCondReasonJobCreated ConditionReason = "JobCreated"
	// ConfigGeneratorCondReasonJobDelete - delete job
	ConfigGeneratorCondReasonJobDelete ConditionReason = "JobDelete"
	// ConfigGeneratorCondReasonJobFailed - failed job
	ConfigGeneratorCondReasonJobFailed ConditionReason = "JobFailed"
	// ConfigGeneratorCondReasonJobFinished - job finished
	ConfigGeneratorCondReasonJobFinished ConditionReason = "JobFinished"
	// ConfigGeneratorCondReasonConfigCreate - config create in progress
	ConfigGeneratorCondReasonConfigCreate ConditionReason = "ConfigCreate"
	// ConfigGeneratorCondReasonWaitingNodesProvisioned - waiting on nodes be provisioned
	ConfigGeneratorCondReasonWaitingNodesProvisioned ConditionReason = "WaitingNodesProvisioned"
	// ConfigGeneratorCondReasonInputLabelError - error adding/update ConfigGeneratorInputLabel
	ConfigGeneratorCondReasonInputLabelError ConditionReason = "ConfigGeneratorInputLabelError"
	// ConfigGeneratorCondReasonRenderEnvFilesError - error rendering environmane file
	ConfigGeneratorCondReasonRenderEnvFilesError ConditionReason = "RenderEnvFilesError"
	// ConfigGeneratorCondReasonClusterServiceIPError - error rendering environmane file
	ConfigGeneratorCondReasonClusterServiceIPError ConditionReason = "ClusterServiceIPError"
)

//
// BaremetalSet
//
const (
	// BaremetalSetCondTypeEmpty - special state for 0 requested BaremetalHosts and 0 already provisioned
	BaremetalSetCondTypeEmpty ConditionType = "Empty"
	// BaremetalSetCondTypeWaiting - something other than BaremetalHost availability is causing the OpenStackBaremetalSet to wait
	BaremetalSetCondTypeWaiting ConditionType = "Waiting"
	// BaremetalSetCondTypeProvisioning - one or more BaremetalHosts are provisioning
	BaremetalSetCondTypeProvisioning ConditionType = "Provisioning"
	// BaremetalSetCondTypeProvisioned - the requested BaremetalHosts count has been satisfied
	BaremetalSetCondTypeProvisioned ConditionType = "Provisioned"
	// BaremetalSetCondTypeDeprovisioning - one or more BaremetalHosts are deprovisioning
	BaremetalSetCondTypeDeprovisioning ConditionType = "Deprovisioning"
	// BaremetalSetCondTypeInsufficient - one or more BaremetalHosts not found (either for scale-up or scale-down) to satisfy count request
	BaremetalSetCondTypeInsufficient ConditionType = "Insufficient availability"
	// BaremetalSetCondTypeError - general catch-all for actual errors
	BaremetalSetCondTypeError ConditionType = "Error"

	//
	// condition reasones
	//

	//
	// metal3
	//

	// BaremetalHostCondReasonListError - metal3 bmh list objects error
	BaremetalHostCondReasonListError ConditionReason = "BaremetalHostListError"
	// BaremetalHostCondReasonGetError - error get metal3 bmh object
	BaremetalHostCondReasonGetError ConditionReason = "BaremetalHostGetError"
	// BaremetalHostCondReasonUpdateError - error updating metal3 bmh object
	BaremetalHostCondReasonUpdateError ConditionReason = "BaremetalHostUpdateError"
	// BaremetalHostCondReasonCloudInitSecretError - error creating cloud-init secrets for metal3
	BaremetalHostCondReasonCloudInitSecretError ConditionReason = "BaremetalHostCloudInitSecretError"

	//
	// osbms
	//

	// BaremetalSetCondReasonError - General error getting the OSBms object
	BaremetalSetCondReasonError ConditionReason = "BaremetalSetError"
	// BaremetalSetCondReasonBaremetalHostStatusNotFound - bare metal host status not found
	BaremetalSetCondReasonBaremetalHostStatusNotFound ConditionReason = "BaremetalHostStatusNotFound"
	// BaremetalSetCondReasonUserDataSecretDeleteError - error deleting user data secret
	BaremetalSetCondReasonUserDataSecretDeleteError ConditionReason = "BaremetalSetUserDataSecretDeleteError"
	// BaremetalSetCondReasonNetworkDataSecretDeleteError - error deleting network data secret
	BaremetalSetCondReasonNetworkDataSecretDeleteError ConditionReason = "BaremetalSetNetworkDataSecretDeleteError"
	// BaremetalSetCondReasonScaleDownInsufficientAnnotatedHosts - not enough nodes annotated for deletion
	BaremetalSetCondReasonScaleDownInsufficientAnnotatedHosts ConditionReason = "BaremetalSetScaleDownInsufficientAnnotatedHosts"
	// BaremetalSetCondReasonScaleUpInsufficientHosts - not enough nodes for requested host count
	BaremetalSetCondReasonScaleUpInsufficientHosts ConditionReason = "BaremetalSetScaleUpInsufficientHosts"
	// BaremetalSetCondReasonProvisioningErrors - errors during bmh provisioning
	BaremetalSetCondReasonProvisioningErrors ConditionReason = "BaremetalSetProvisioningErrors"
	// BaremetalSetCondReasonVirtualMachineProvisioning - bmh provisioning in progress
	BaremetalSetCondReasonVirtualMachineProvisioning ConditionReason = "BaremetalHostProvisioning"
	// BaremetalSetCondReasonVirtualMachineDeprovisioning - bmh deprovisioning in progress
	BaremetalSetCondReasonVirtualMachineDeprovisioning ConditionReason = "BaremetalHostDeprovisioning"
	// BaremetalSetCondReasonVirtualMachineProvisioned - bmh provisioned
	BaremetalSetCondReasonVirtualMachineProvisioned ConditionReason = "BaremetalHostProvisioned"
	// BaremetalSetCondReasonVirtualMachineCountZero - no bmh requested
	BaremetalSetCondReasonVirtualMachineCountZero ConditionReason = "BaremetalHostCountZero"
)

//
// ControlPlane
//
const (
	// ControlPlaneEmpty - special state for 0 requested VMs and 0 already provisioned
	ControlPlaneEmpty ConditionType = "Empty"
	// ControlPlaneWaiting - something is causing the OpenStackBaremetalSet to wait
	ControlPlaneWaiting ConditionType = "Waiting"
	// ControlPlaneProvisioning - one or more VMs are provisioning
	ControlPlaneProvisioning ConditionType = "Provisioning"
	// ControlPlaneProvisioned - the requested VM count for all roles has been satisfied
	ControlPlaneProvisioned ConditionType = "Provisioned"
	// ControlPlaneDeprovisioning - one or more VMs are deprovisioning
	ControlPlaneDeprovisioning ConditionType = "Deprovisioning"
	// ControlPlaneError - general catch-all for actual errors
	ControlPlaneError ConditionType = "Error"

	//
	// condition reasones
	//

	// ControlPlaneReasonNetNotFound - osctlplane not found
	ControlPlaneReasonNetNotFound ConditionReason = "CtlPlaneNotFound"
	// ControlPlaneReasonNotSupportedVersion - osctlplane not found
	ControlPlaneReasonNotSupportedVersion ConditionReason = "CtlPlaneNotSupportedVersion"
	// ControlPlaneReasonTripleoPasswordsSecretError - Tripleo password secret error
	ControlPlaneReasonTripleoPasswordsSecretError ConditionReason = "TripleoPasswordsSecretError"
	// ControlPlaneReasonTripleoPasswordsSecretNotFound - Tripleo password secret not found
	ControlPlaneReasonTripleoPasswordsSecretNotFound ConditionReason = "TripleoPasswordsSecretNotFound"
	// ControlPlaneReasonTripleoPasswordsSecretCreateError - Tripleo password secret create error
	ControlPlaneReasonTripleoPasswordsSecretCreateError ConditionReason = "TripleoPasswordsSecretCreateError"
	// ControlPlaneReasonDeploymentSSHKeysSecretError - Deployment SSH Keys Secret Error
	ControlPlaneReasonDeploymentSSHKeysSecretError ConditionReason = "DeploymentSSHKeysSecretError"
	// ControlPlaneReasonDeploymentSSHKeysGenError - Deployment SSH Keys generation Error
	ControlPlaneReasonDeploymentSSHKeysGenError ConditionReason = "DeploymentSSHKeysGenError"
	// ControlPlaneReasonDeploymentSSHKeysSecretCreateOrUpdateError - Deployment SSH Keys Secret Crate or Update Error
	ControlPlaneReasonDeploymentSSHKeysSecretCreateOrUpdateError ConditionReason = "DeploymentSSHKeysSecretCreateOrUpdateError"
)

//
// Deploy
//
const (
	//
	// condition types
	//

	// DeployCondTypeWaiting - the deployment is waiting
	DeployCondTypeWaiting ConditionType = "Waiting"
	// DeployCondTypeInitializing - the deployment is waiting
	DeployCondTypeInitializing ConditionType = "Initializing"
	// DeployCondTypeRunning - the deployment is running
	DeployCondTypeRunning ConditionType = "Running"
	// DeployCondTypeFinished - the deploy has finished executing
	DeployCondTypeFinished ConditionType = "Finished"
	// DeployCondTypeError - the deployment hit a generic error
	DeployCondTypeError ConditionType = "Error"

	// DeployCondReasonJobCreated - job created
	DeployCondReasonJobCreated ConditionReason = "JobCreated"
	// DeployCondReasonJobCreateFailed - job create failed
	DeployCondReasonJobCreateFailed ConditionReason = "JobCreated"
	// DeployCondReasonJobDelete - job deleted
	DeployCondReasonJobDelete ConditionReason = "JobDeleted"
	// DeployCondReasonJobFinished - job deleted
	DeployCondReasonJobFinished ConditionReason = "JobFinished"
	// DeployCondReasonCVUpdated - error creating/update ConfigVersion
	DeployCondReasonCVUpdated ConditionReason = "ConfigVersionUpdated"
	// DeployCondReasonConfigVersionNotFound - error finding ConfigVersion
	DeployCondReasonConfigVersionNotFound ConditionReason = "ConfigVersionNotFound"
	// DeployCondReasonJobFailed - error creating/update CM
	DeployCondReasonJobFailed ConditionReason = "JobFailed"
	// DeployCondReasonConfigCreate - error creating/update CM
	DeployCondReasonConfigCreate ConditionReason = "ConfigCreate"
)

//
// EphemeralHeat
//
const (
	//
	// condition reasons
	//

	// EphemeralHeatCondGenPassSecretError - error generating password secret
	EphemeralHeatCondGenPassSecretError ConditionReason = "EphemeralHeatCondGenPassSecretError"
	// EphemeralHeatCondWaitOnPassSecret - waiting for the generated password secret to populate
	EphemeralHeatCondWaitOnPassSecret ConditionReason = "EphemeralHeatCondWaitOnPassSecret"
	// EphemeralHeatCondMariaDBError - error creating/updating MariaDB pod or service
	EphemeralHeatCondMariaDBError ConditionReason = "EphemeralHeatCondMariaDBError"
	// EphemeralHeatCondWaitOnMariaDB - waiting for MariaDB pod to be ready
	EphemeralHeatCondWaitOnMariaDB ConditionReason = "EphemeralHeatCondWaitOnMariaDB"
	// EphemeralHeatCondRabbitMQError - error creating/updating RabbitMQ pod or service
	EphemeralHeatCondRabbitMQError ConditionReason = "EphemeralHeatCondRabbitMQError"
	// EphemeralHeatCondWaitOnRabbitMQ - waiting for RabbitMQ pod to be ready
	EphemeralHeatCondWaitOnRabbitMQ ConditionReason = "EphemeralHeatCondWaitOnRabbitMQ"
	// EphemeralHeatCondHeatError - error creating/updating Heat pod or service
	EphemeralHeatCondHeatError ConditionReason = "EphemeralHeatCondHeatError"
	// EphemeralHeatCondWaitOnHeat - waiting for Heat pod to be ready
	EphemeralHeatCondWaitOnHeat ConditionReason = "EphemeralHeatCondWaitOnHeat"
	// EphemeralHeatReady - Heat is available for use
	EphemeralHeatReady ConditionReason = "EphemeralHeatReady"
)

//
// IPSet
//
const (
	//
	// condition reasones
	//

	// IPSetCondReasonError - General error getting the OSIPset object
	IPSetCondReasonError ConditionReason = "OpenStackIPSetError"
	// IPSetCondReasonProvisioned - ipset provisioned
	IPSetCondReasonProvisioned ConditionReason = "OpenStackIPSetProvisioned"
	// IPSetCondReasonWaitingOnHosts - ipset Waiting on hosts to be created on IPSet
	IPSetCondReasonWaitingOnHosts ConditionReason = "OpenStackIPSetWaitingOnHostsCreated"
	// IPSetCondReasonCreated - created ipset
	IPSetCondReasonCreated ConditionReason = "OpenStackIPSetCreated"
)

//
// MACAddress
//
const (
	//
	// condition types
	//

	// MACCondTypeWaiting - the mac creation is blocked by prerequisite objects
	MACCondTypeWaiting ConditionType = "Waiting"
	// MACCondTypeCreating - we are waiting for mac addresses to be created
	MACCondTypeCreating ConditionType = "Creating"
	// MACCondTypeConfigured - the MAC addresses have been created
	MACCondTypeConfigured ConditionType = "Created"
	// MACCondTypeError - the mac creation hit a error
	MACCondTypeError ConditionType = "Error"

	//
	// condition reasones
	//

	// MACCondReasonRemovedIPs - the removed MAC reservations
	MACCondReasonRemovedIPs ConditionReason = "RemovedIPs"
	// MACCondReasonNetNotFound - osnet not found
	MACCondReasonNetNotFound ConditionReason = "NetNotFound"
	// MACCondReasonCreateMACError - error creating MAC address
	MACCondReasonCreateMACError ConditionReason = "CreateMACError"
	// MACCondReasonAllMACAddressesCreated - all MAC addresses created
	MACCondReasonAllMACAddressesCreated ConditionReason = "MACAddressesCreated"
	// MACCondReasonError - General error getting the OSMACaddr object
	MACCondReasonError ConditionReason = "MACError"
	// MACCondReasonMACNotFound - osmacaddr object not found
	MACCondReasonMACNotFound ConditionReason = "OpenStackMACNotFound"
)

//
// Net
//
const (
	//
	// condition types
	//

	// NetWaiting - the network configuration is blocked by prerequisite objects
	NetWaiting ConditionType = "Waiting"
	// NetInitializing - we are waiting for underlying OCP network resource(s) to appear
	NetInitializing ConditionType = "Initializing"
	// NetConfiguring - the underlying network resources are configuring the nodes
	NetConfiguring ConditionType = "Configuring"
	// NetConfigured - the nodes have been configured by the underlying network resources
	NetConfigured ConditionType = "Configured"
	// NetError - the network configuration hit a generic error
	NetError ConditionType = "Error"

	//
	// condition reasones
	//

	// NetCondReasonCreated - osnet created
	NetCondReasonCreated ConditionReason = "OpenStackNetCreated"
	// NetCondReasonCreateError - error creating osnet object
	NetCondReasonCreateError ConditionReason = "OpenStackNetCreateError"
	// NetCondReasonNetNotFound - error osnet not found
	NetCondReasonNetNotFound ConditionReason = "OpenStackNetNotFound"
)

//
// NetAttach
//
const (
	//
	// condition types
	//

	// NetAttachWaiting - the network configuration is blocked by prerequisite objects
	NetAttachWaiting ConditionType = "Waiting"
	// NetAttachInitializing - we are waiting for underlying OCP network resource(s) to appear
	NetAttachInitializing ConditionType = "Initializing"
	// NetAttachConfiguring - the underlying network resources are configuring the nodes
	NetAttachConfiguring ConditionType = "Configuring"
	// NetAttachConfigured - the nodes have been configured by the underlying network resources
	NetAttachConfigured ConditionType = "Configured"
	// NetAttachError - the network configuration hit a generic error
	NetAttachError ConditionType = "Error"

	//
	// condition reasones
	//

	// NetAttachCondReasonCreated - osnetattachment created
	NetAttachCondReasonCreated ConditionReason = "OpenStackNetAttachCreated"
	// NetAttachCondReasonCreateError - error creating osnetatt object
	NetAttachCondReasonCreateError ConditionReason = "OpenStackNetAttachCreateError"
)

//
// NetConfig
//
const (
	// NetConfigWaiting - the network configuration is blocked by prerequisite objects
	NetConfigWaiting ConditionType = "Waiting"
	// NetConfigInitializing - we are waiting for underlying OCP network resource(s) to appear
	NetConfigInitializing ConditionType = "Initializing"
	// NetConfigConfiguring - the underlying network resources are configuring the nodes
	NetConfigConfiguring ConditionType = "Configuring"
	// NetConfigConfigured - the nodes have been configured by the underlying network resources
	NetConfigConfigured ConditionType = "Configured"
	// NetConfigError - the network configuration hit a generic error
	NetConfigError ConditionType = "Error"

	// NetConfigCondReasonError - osnetcfg error
	NetConfigCondReasonError ConditionReason = "OpenStackNetConfigError"
	// NetConfigCondReasonWaitingOnIPsForHost - waiting on IPs for all configured networks to be created
	NetConfigCondReasonWaitingOnIPsForHost ConditionReason = "WaitingOnIPsForHost"
	// NetConfigCondReasonWaitingOnHost - waiting on host to be added to osnetcfg
	NetConfigCondReasonWaitingOnHost ConditionReason = "WaitingOnHost"
	// NetConfigCondReasonIPReservationError - Failed to do ip reservation
	NetConfigCondReasonIPReservationError ConditionReason = "IPReservationError"
	// NetConfigCondReasonIPReservation - ip reservation created
	NetConfigCondReasonIPReservation ConditionReason = "IPReservationCreated"
)

//
// ProvisionServer
//
const (
	// ProvisionServerCondTypeWaiting - something else is causing the OpenStackProvisionServer to wait
	ProvisionServerCondTypeWaiting ConditionType = "Waiting"
	// ProvisionServerCondTypeProvisioning - the provision server pod is provisioning
	ProvisionServerCondTypeProvisioning ConditionType = "Provisioning"
	// ProvisionServerCondTypeProvisioned - the provision server pod is ready
	ProvisionServerCondTypeProvisioned ConditionType = "Provisioned"
	// ProvisionServerCondTypeError - general catch-all for actual errors
	ProvisionServerCondTypeError ConditionType = "Error"

	//
	// condition reasons
	//

	// OpenStackProvisionServerCondReasonListError - osprovserver list objects error
	OpenStackProvisionServerCondReasonListError ConditionReason = "OpenStackProvisionServerListError"
	// OpenStackProvisionServerCondReasonGetError - osprovserver list objects error
	OpenStackProvisionServerCondReasonGetError ConditionReason = "OpenStackProvisionServerCondReasonGetError"
	// OpenStackProvisionServerCondReasonNotFound - osprovserver object not found
	OpenStackProvisionServerCondReasonNotFound ConditionReason = "OpenStackProvisionServerNotFound"
	// OpenStackProvisionServerCondReasonInterfaceAcquireError - osprovserver hit an error while finding provisioning interface name
	OpenStackProvisionServerCondReasonInterfaceAcquireError ConditionReason = "OpenStackProvisionServerCondReasonInterfaceAcquireError"
	// OpenStackProvisionServerCondReasonInterfaceNotFound - osprovserver unable to find provisioning interface name
	OpenStackProvisionServerCondReasonInterfaceNotFound ConditionReason = "OpenStackProvisionServerCondReasonInterfaceNotFound"
	// OpenStackProvisionServerCondReasonDeploymentError - osprovserver associated deployment failed to create/update
	OpenStackProvisionServerCondReasonDeploymentError ConditionReason = "OpenStackProvisionServerCondReasonDeploymentError"
	// OpenStackProvisionServerCondReasonDeploymentCreated - osprovserver associated deployment has been created/update
	OpenStackProvisionServerCondReasonDeploymentCreated ConditionReason = "OpenStackProvisionServerCondReasonDeploymentCreated"
	// OpenStackProvisionServerCondReasonProvisioning - osprovserver associated pod is provisioning
	OpenStackProvisionServerCondReasonProvisioning ConditionReason = "OpenStackProvisionServerCondReasonProvisioning"
	// OpenStackProvisionServerCondReasonLocalImageURLParseError - osprovserver was unable to parse its received local image URL
	OpenStackProvisionServerCondReasonLocalImageURLParseError ConditionReason = "OpenStackProvisionServerCondReasonLocalImageURLParseError"
	// OpenStackProvisionServerCondReasonProvisioned - osprovserver associated pod is provisioned
	OpenStackProvisionServerCondReasonProvisioned ConditionReason = "OpenStackProvisionServerCondReasonProvisioned"
	// OpenStackProvisionServerCondReasonCreateError - error creating osprov server object
	OpenStackProvisionServerCondReasonCreateError ConditionReason = "OpenStackProvisionServerCreateError"
	// OpenStackProvisionServerCondReasonCreated - osprov server object created
	OpenStackProvisionServerCondReasonCreated ConditionReason = "OpenStackProvisionServerCreated"
)

//
// VMSet
//
const (
	//
	// condition types
	//

	// VMSetCondTypeEmpty - special state for 0 requested VMs and 0 already provisioned
	VMSetCondTypeEmpty ConditionType = "Empty"
	// VMSetCondTypeWaiting - something is causing the OpenStackVmSet to wait
	VMSetCondTypeWaiting ConditionType = "Waiting"
	// VMSetCondTypeProvisioning - one or more VMs are provisioning
	VMSetCondTypeProvisioning ConditionType = "Provisioning"
	// VMSetCondTypeProvisioned - the requested VM count has been satisfied
	VMSetCondTypeProvisioned ConditionType = "Provisioned"
	// VMSetCondTypeDeprovisioning - one or more VMs are deprovisioning
	VMSetCondTypeDeprovisioning ConditionType = "Deprovisioning"
	// VMSetCondTypeError - general catch-all for actual errors
	VMSetCondTypeError ConditionType = "Error"

	//
	// condition reasones
	//

	// VMSetCondReasonError - error creating osvmset
	VMSetCondReasonError ConditionReason = "OpenStackVMSetError"
	// VMSetCondReasonInitialize - vmset initialize
	VMSetCondReasonInitialize ConditionReason = "OpenStackVMSetInitialize"
	// VMSetCondReasonProvisioning - vmset provisioning
	VMSetCondReasonProvisioning ConditionReason = "OpenStackVMSetProvisioning"
	// VMSetCondReasonDeprovisioning - vmset deprovisioning
	VMSetCondReasonDeprovisioning ConditionReason = "OpenStackVMSetDeprovisioning"
	// VMSetCondReasonProvisioned - vmset provisioned
	VMSetCondReasonProvisioned ConditionReason = "OpenStackVMSetProvisioned"
	// VMSetCondReasonCreated - vmset created
	VMSetCondReasonCreated ConditionReason = "OpenStackVMSetCreated"

	// VMSetCondReasonNamespaceFencingDataError - error creating the namespace fencing data
	VMSetCondReasonNamespaceFencingDataError ConditionReason = "NamespaceFencingDataError"
	// VMSetCondReasonKubevirtFencingServiceAccountError - error creating/reading the KubevirtFencingServiceAccount secret
	VMSetCondReasonKubevirtFencingServiceAccountError ConditionReason = "KubevirtFencingServiceAccountError"
	// VMSetCondReasonKubeConfigError - error getting the KubeConfig used by the operator
	VMSetCondReasonKubeConfigError ConditionReason = "KubeConfigError"
	// VMSetCondReasonCloudInitSecretError - error creating the CloudInitSecret
	VMSetCondReasonCloudInitSecretError ConditionReason = "CloudInitSecretError"
	// VMSetCondReasonDeploymentSecretMissing - deployment secret does not exist
	VMSetCondReasonDeploymentSecretMissing ConditionReason = "DeploymentSecretMissing"
	// VMSetCondReasonDeploymentSecretError - deployment secret error
	VMSetCondReasonDeploymentSecretError ConditionReason = "DeploymentSecretError"
	// VMSetCondReasonPasswordSecretMissing - password secret does not exist
	VMSetCondReasonPasswordSecretMissing ConditionReason = "PasswordSecretMissing"
	// VMSetCondReasonPasswordSecretError - password secret error
	VMSetCondReasonPasswordSecretError ConditionReason = "PasswordSecretError"

	// VMSetCondReasonVirtualMachineGetError - failed to get virtual machine
	VMSetCondReasonVirtualMachineGetError ConditionReason = "VirtualMachineGetError"
	// VMSetCondReasonVirtualMachineAnnotationMissmatch - Unable to find sufficient amount of VirtualMachine replicas annotated for scale-down
	VMSetCondReasonVirtualMachineAnnotationMissmatch ConditionReason = "VirtualMachineAnnotationMissmatch"
	// VMSetCondReasonVirtualMachineNetworkDataError - Error creating VM NetworkData
	VMSetCondReasonVirtualMachineNetworkDataError ConditionReason = "VMSetCondReasonVirtualMachineNetworkDataError"
	// VMSetCondReasonVirtualMachineProvisioning - virtual machine provisioning in progress
	VMSetCondReasonVirtualMachineProvisioning ConditionReason = "VirtualMachineProvisioning"
	// VMSetCondReasonVirtualMachineDeprovisioning - virtual machine deprovisioning in progress
	VMSetCondReasonVirtualMachineDeprovisioning ConditionReason = "VirtualMachineDeprovisioning"
	// VMSetCondReasonVirtualMachineProvisioned - virtual machines provisioned
	VMSetCondReasonVirtualMachineProvisioned ConditionReason = "VirtualMachineProvisioned"
	// VMSetCondReasonVirtualMachineCountZero - no virtual machines requested
	VMSetCondReasonVirtualMachineCountZero ConditionReason = "VirtualMachineCountZero"

	// VMSetCondReasonPersitentVolumeClaimNotFound - Persitent Volume Claim Not Found
	VMSetCondReasonPersitentVolumeClaimNotFound ConditionReason = "PersitentVolumeClaimNotFound"
	// VMSetCondReasonPersitentVolumeClaimError - Persitent Volume Claim error
	VMSetCondReasonPersitentVolumeClaimError ConditionReason = "PersitentVolumeClaimError"
	// VMSetCondReasonPersitentVolumeClaimCreating - Persitent Volume Claim create in progress
	VMSetCondReasonPersitentVolumeClaimCreating ConditionReason = "PersitentVolumeClaimCreating"
	// VMSetCondReasonBaseImageNotReady - VM base image not ready
	VMSetCondReasonBaseImageNotReady ConditionReason = "BaseImageNotReady"
)
