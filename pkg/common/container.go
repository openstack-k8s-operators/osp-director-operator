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

package common

import (
	"fmt"
	"regexp"
	"strings"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	"github.com/pkg/errors"
)

func getOSPVersion(parsedVersion string) (ospdirectorv1beta1.OSPVersion, error) {
	switch parsedVersion {
	case string(ospdirectorv1beta1.TemplateVersionTrain):
		return ospdirectorv1beta1.TemplateVersionTrain, nil

	case string(ospdirectorv1beta1.TemplateVersionWallaby):
		return ospdirectorv1beta1.TemplateVersionWallaby, nil

	default:
		err := fmt.Errorf("not a supported OSP version: %v", parsedVersion)
		return "", err

	}
}

// GetVersionFromImageURL -
func GetVersionFromImageURL(imageURL string) (ospdirectorv1beta1.OSPVersion, error) {

	parts := strings.SplitN(imageURL, ":", 2)
	if len(parts) != 2 {
		return "", errors.Errorf(`Invalid image name URL "%s", expected url:tag`, imageURL)
	}

	tag := parts[1]

	// get the first major.minor version string
	re := regexp.MustCompile(`(\d+\.\d+)?`)
	match := re.FindStringSubmatch(tag)

	// verify parsed OSP version
	ospVersionm, err := getOSPVersion(match[1])

	return ospVersionm, err
}
