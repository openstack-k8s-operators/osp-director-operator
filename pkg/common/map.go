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

import "sort"

// MergeMaps - merge two or more maps
func MergeMaps(baseMap map[string]interface{}, extraMaps ...map[string]interface{}) map[string]interface{} {
	for _, extraMap := range extraMaps {
		for key, value := range extraMap {
			baseMap[key] = value
		}
	}

	return baseMap
}

// Pair -
type Pair struct {
	Key   string
	Value string
}

// List -
type List []Pair

func (p List) Len() int           { return len(p) }
func (p List) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p List) Less(i, j int) bool { return p[i].Key < p[j].Key }

// SortMapByValue -
func SortMapByValue(in map[string]string) List {

	sorted := make(List, len(in))

	i := 0
	for k, v := range in {
		sorted[i] = Pair{k, v}
		i++
	}

	sort.Sort(sorted)

	return sorted
}
