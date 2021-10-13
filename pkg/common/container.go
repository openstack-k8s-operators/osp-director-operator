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
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GetOSPVersion -
func GetOSPVersion(parsedVersion string) (ospdirectorv1beta1.OSPVersion, error) {
	log.Log.Info(fmt.Sprintf("Parsed OSP Version %s", parsedVersion))
	switch parsedVersion {
	case string(ospdirectorv1beta1.TemplateVersionTrain):
		return ospdirectorv1beta1.TemplateVersion16_2, nil

	case string(ospdirectorv1beta1.TemplateVersion16_2):
		return ospdirectorv1beta1.TemplateVersion16_2, nil

	case string(ospdirectorv1beta1.TemplateVersionWallaby):
		return ospdirectorv1beta1.TemplateVersion17_0, nil

	case string(ospdirectorv1beta1.TemplateVersion17_0):
		return ospdirectorv1beta1.TemplateVersion17_0, nil
	default:
		err := fmt.Errorf("not a supported OSP version: %v", parsedVersion)
		return "", err

	}
}

// GetVersionFromImageURL -
func GetVersionFromImageURL(r ReconcilerCommon, imageURL string) (ospdirectorv1beta1.OSPVersion, error) {

	parts := strings.SplitN(imageURL, ":", 2)
	if len(parts) != 2 {
		return "", errors.Errorf(`Invalid image name URL "%s", expected url:tag`, imageURL)
	}

	url := parts[0]
	tag := parts[1]

	// check for downstream version in image URL tags
	// get the first major.minor version string
	re := regexp.MustCompile(`(\d+\.\d+)?`)
	match := re.FindStringSubmatch(tag)

	// if no match in checking image tag, check for upstream image version in URL
	if match == nil || match[1] == "" {
		// Until all patches backported to upstream wallaby there are two possibilities:
		// -quay.io/tripleowallaby/openstack-tripleoclient:current-tripleo
		// -quay.io/openstack-k8s-operators/tripleowallaby-openstack-tripleoclient:current-tripleo_patched
		re = regexp.MustCompile(`.*\/tripleo(.*)[-\/](openstack-tripleoclient).*`)
		match = re.FindStringSubmatch(url)
	}

	if len(match) > 0 && match[1] != "" {
		// verify parsed OSP version
		ospVersion, err := GetOSPVersion(match[1])
		r.GetLogger().Info(fmt.Sprintf("OSP version %s", ospVersion))
		return ospVersion, err
	}

	return "", fmt.Errorf("no OSP version detected from imageURL: %s", imageURL)
}
