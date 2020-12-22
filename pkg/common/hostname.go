/*

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
	"sort"
	"strconv"
	"strings"
)

// HostnameStore -
type HostnameStore interface {
	GetHostnames() map[string]string
}

// CreateOrGetHostname -
func CreateOrGetHostname(hostStore HostnameStore, idkey string, basename string) (string, error) {
	if found, ok := hostStore.GetHostnames()[idkey]; ok {
		return found, nil
	}

	// Get all numbers currently in use
	foundNumbers := []int{}

	//for _, bmh := range instance.Status.BaremetalHosts {
	for _, hostname := range hostStore.GetHostnames() {
		pieces := strings.Split(hostname, "-")
		num, err := strconv.Atoi(pieces[len(pieces)-1])

		if err != nil {
			// This should never happen, as we control the generated hostnames
			// and always use the "<instance.Name>-<number>" format
			return "", err
		}

		foundNumbers = append(foundNumbers, num)
	}

	// Sort the existing numbers in ascending order
	sort.Ints(foundNumbers)

	//
	// Approach: choose the lowest unused number in the sequence
	//

	chosenNumber := -1

	if len(foundNumbers) > 0 && foundNumbers[0] != 0 {
		// Unique case where first number is not 0, so we choose 0
		chosenNumber = 0
	} else {
		for i := 1; i < len(foundNumbers); i++ {
			// If there is a gap of at least 1 number between this foundNumber and the last,
			// we use the number equal to foundNumbers[i-1]+1
			if foundNumbers[i-1] < foundNumbers[i]-1 {
				chosenNumber = foundNumbers[i-1] + 1
				break
			}
		}
	}

	if chosenNumber == -1 {
		// No gaps were found that we could reuse, so just chose the next number
		// in the sequence
		chosenNumber = len(foundNumbers)
	}

	return fmt.Sprintf("%s-%d", strings.ToLower(basename), chosenNumber), nil
}
