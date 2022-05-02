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

const (
	//
	// LabelSelectors
	//

	// NetworkNameLabelSelector -
	NetworkNameLabelSelector = "ooo-netname"
	// NetworkNameLowerLabelSelector -
	NetworkNameLowerLabelSelector = "ooo-netname-lower"
	// SubNetNameLabelSelector -
	SubNetNameLabelSelector = "ooo-subnetname"
	// ControlPlaneNetworkLabelSelector - is the network a ctlplane network?
	ControlPlaneNetworkLabelSelector = "ooo-ctlplane-network"
	// OpenStackNetConfigReconcileLabel - label set on objects on which change
	// trigger a reconcile of the osnetconfig
	OpenStackNetConfigReconcileLabel = "osnetconfig-ref"
)
