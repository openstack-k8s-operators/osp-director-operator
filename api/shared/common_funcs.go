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

// MergeStringMaps - merge two or more string->map maps
func MergeStringMaps(baseMap map[string]string, extraMaps ...map[string]string) map[string]string {
	InitMap(&baseMap)

	for _, extraMap := range extraMaps {
		for key, value := range extraMap {
			baseMap[key] = value
		}
	}

	return baseMap
}

// InitMap - Inititialise a map to an empty map if it is nil.
func InitMap(m *map[string]string) {
	if *m == nil {
		*m = make(map[string]string)
	}
}
