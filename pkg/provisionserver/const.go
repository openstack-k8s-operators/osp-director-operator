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

package provisionserver

const (
	// AppLabel -
	AppLabel = "osp-provisionserver"

	// DefaultPort - The starting default port for the OpenStackProvisionServer's Apache container
	DefaultPort = 6190

	// FinalizerName -
	FinalizerName = "provisionservers.osp-director.openstack.org"

	// HttpdConfPath -
	HttpdConfPath = "/usr/local/apache2/conf/httpd.conf"

	// OwnerUIDLabelSelector -
	OwnerUIDLabelSelector = "provisionservers.osp-director.openstack.org/uid"
	// OwnerNameSpaceLabelSelector -
	OwnerNameSpaceLabelSelector = "provisionservers.osp-director.openstack.org/namespace"
	// OwnerNameLabelSelector -
	OwnerNameLabelSelector = "provisionservers.osp-director.openstack.org/name"

	// ServiceAccount -
	ServiceAccount = "osp-director-operator-openstackprovisionserver"
)
