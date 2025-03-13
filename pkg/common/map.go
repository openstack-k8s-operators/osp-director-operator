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

package common //revive:disable:var-naming

import (
	"fmt"
	"reflect"
	"sort"
)

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

// IsInterfaceMap - check if type interface{} is a map
func IsInterfaceMap(v interface{}) bool {
	return reflect.TypeOf(v).Kind() == reflect.Map
}

// RecursiveMergeMaps recursively merges the src into dst maps.
func RecursiveMergeMaps(
	dst, src map[string]interface{},
	maxDepth int) (map[string]interface{},
	error,
) {
	return recursiveMerge(dst, src, 0, maxDepth)
}

func recursiveMerge(
	dst, src map[string]interface{},
	depth int,
	maxDepth int,
) (map[string]interface{}, error) {
	if depth > maxDepth {
		return dst, fmt.Errorf("reached max depth of %d", maxDepth)
	}
	for key, srcVal := range src {
		if dstVal, ok := dst[key]; !ok {
			srcMap, srcMapOk := convertMap(srcVal)
			dstMap, _ := convertMap(dstVal)

			if srcMapOk {
				var err error
				srcVal, err = recursiveMerge(dstMap, srcMap, depth+1, maxDepth)
				if err != nil {
					return dst, err
				}
			}
		}
		dst[key] = srcVal
	}
	return dst, nil
}

// if i is of type Map, converts to map[string]interface{}
// else returns an empty map[string]interface{} and returns
// the map and true of it was a conversion job.
func convertMap(i interface{}) (map[string]interface{}, bool) {
	value := reflect.ValueOf(i)
	if value.Kind() == reflect.Map {
		// convert map to map[string]interface{}
		m := map[string]interface{}{}
		for _, k := range value.MapKeys() {
			m[fmt.Sprintf("%v", k)] = value.MapIndex(k).Interface()
		}

		return m, true
	}

	return map[string]interface{}{}, false
}
