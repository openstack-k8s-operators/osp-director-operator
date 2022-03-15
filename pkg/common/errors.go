/*
Copyright 2022 Red Hat

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
	"errors"
	"fmt"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
)

// ForbiddenPodSpecChangeError - error returned when Pod must be recreate
type ForbiddenPodSpecChangeError struct {
	Field string
}

func (e *ForbiddenPodSpecChangeError) Error() string {
	return fmt.Sprintf("cannot update Pod spec field: %s", e.Field)
}

// IsForbiddenPodSpecChangeError -
func IsForbiddenPodSpecChangeError(err error) bool {
	return errors.Is(err, &ForbiddenPodSpecChangeError{}) || k8s_errors.IsInvalid(err)
}
