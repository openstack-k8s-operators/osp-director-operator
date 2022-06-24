/*
Copyright 2021 Red Hat

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

package openstackconfiggenerator

const (
	// AppLabel -
	AppLabel = "osp-openstackconfiggenerator"

	// NicTemplateTrain - nic template file for train/16.2
	NicTemplateTrain = "net-config-multi-nic.role.j2.yaml"

	// RenderedNicFileTrain - suffix of the renered train/16.2 file
	RenderedNicFileTrain = "nic-template.role.j2.yaml"

	// NicTemplateWallaby - nic template file for wallaby/17.0 and later
	NicTemplateWallaby = "multiple_nics_dvr.j2"

	// RenderedNicFileWallaby - suffix of the renered wallaby/17.0 file
	RenderedNicFileWallaby = "nic-template.j2"

	// NetworkDataFile - tripleo network data file name
	NetworkDataFile = "network_data.yaml"

	// ConfigGeneratorInputLabel - label set on objects which are input for the configgenerator
	// but not owned by the configgenerator
	ConfigGeneratorInputLabel = "playbook-generator-input"
)
