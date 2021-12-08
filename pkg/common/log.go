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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func logObjectParams(object metav1.Object) []interface{} {
	return []interface{}{
		"ObjectType", fmt.Sprintf("%T", object),
		"ObjectNamespace", object.GetNamespace(),
		"ObjectName", object.GetName()}
}

// LogForObject -
func LogForObject(r ReconcilerCommon,
	msg string, object metav1.Object, params ...interface{}) {

	params = append(params, logObjectParams(object)...)

	r.GetLogger().Info(msg, params...)
}

// WrapErrorForObject -
func WrapErrorForObject(msg string, object client.Object, err error) error {
	key := client.ObjectKeyFromObject(object)

	return fmt.Errorf("%s %T %v: %w",
		msg, object, key, err)
}

// LogErrorForObject -
func LogErrorForObject(r ReconcilerCommon,
	err error, msg string, object metav1.Object, params ...interface{}) {

	params = append(params, logObjectParams(object)...)
	r.GetLogger().Error(err, msg, params...)
}
