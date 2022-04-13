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

import "fmt"

// OSPVersion - OSP template version
type OSPVersion string

const (
	//
	// OSPVersion
	//

	// TemplateVersionTrain - upstream train template version
	TemplateVersionTrain OSPVersion = "train"
	// TemplateVersion16_2 - OSP 16.2 template version
	TemplateVersion16_2 OSPVersion = "16.2"
	// TemplateVersionWallaby - upstream wallaby template version
	TemplateVersionWallaby OSPVersion = "wallaby"
	// TemplateVersion17_0 - OSP 17.0 template version
	TemplateVersion17_0 OSPVersion = "17.0"
)

// GetOSPVersion - returns unified ospdirectorv1beta1.OSPVersion for upstream/downstream version
//  - TemplateVersion16_2 for eitner 16.2 or upstream train
//  - TemplateVersion17_0 for eitner 17.0 or upstream wallaby
func GetOSPVersion(parsedVersion string) (OSPVersion, error) {
	switch parsedVersion {
	case string(TemplateVersionTrain):
		return TemplateVersion16_2, nil

	case string(TemplateVersion16_2):
		return TemplateVersion16_2, nil

	case string(TemplateVersionWallaby):
		return TemplateVersion17_0, nil

	case string(TemplateVersion17_0):
		return TemplateVersion17_0, nil
	default:
		err := fmt.Errorf("not a supported OSP version: %v", parsedVersion)
		return "", err

	}
}
