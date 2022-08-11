package common

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/tidwall/gjson"
	"sigs.k8s.io/yaml"
)

// GetFencingRoles - roles that normally require fencing
func GetFencingRoles() []string {
	// Can't create an array constant, so this is what we'll do instead
	return []string{
		"CellController",
		"ControllerAllNovaStandalone",
		"ControllerNoCeph",
		"ControllerNovaStandalone",
		"ControllerOpenstack",
		"ControllerSriov",
		"ControllerStorageDashboard",
		"ControllerStorageNfs",
		"Controller",
		"Database",
		"Messaging",
		"Standalone",
		"Telemetry",
	}
}

// GetCustomFencingRoles - return a list of any custom roles included in custom tarball that require fencing support
func GetCustomFencingRoles(customBinaryData map[string][]byte) ([]string, error) {
	customFencingRoles := []string{}

	for _, binaryData := range customBinaryData {
		reader := bytes.NewReader(binaryData)
		uncompressedStream, err := gzip.NewReader(reader)

		if err != nil {
			return nil, err
		}

		tarReader := tar.NewReader(uncompressedStream)

		for {
			header, err := tarReader.Next()

			if err != nil {
				if err == io.EOF {
					break
				}

				return nil, err
			}

			switch header.Typeflag {
			case tar.TypeReg:
				if header.Name == TripleORolesDataFile {
					buf, err := io.ReadAll(tarReader)

					if err != nil {
						return nil, err
					}

					jsonBytes, err := yaml.YAMLToJSON(buf)

					if err != nil {
						return nil, err
					}

					jsonStr := string(jsonBytes)

					if !gjson.Valid(jsonStr) {
						return nil, fmt.Errorf("invalid YAML detected in custom tarball")
					}

					res := gjson.Parse(jsonStr)

					res.ForEach(func(key gjson.Result, value gjson.Result) bool {
						if value.Get(TripleOServicesDefaultKeyName).Exists() {
							res2 := value.Get(TripleOServicesDefaultKeyName)
							res2.ForEach(func(key2 gjson.Result, value2 gjson.Result) bool {
								if value2.String() == TripleOPacemakerServiceName {
									customFencingRoles = append(customFencingRoles, value.Get("name").String())
									return false
								}
								return true
							})
						}
						return true
					})
				}
			}
		}
	}

	return customFencingRoles, nil
}
