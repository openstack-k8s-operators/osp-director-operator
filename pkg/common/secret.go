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
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
)

const (
	// BITSIZE -
	BITSIZE   int    = 4096
	sshConfig string = `Host *
    User cloud-admin
    StrictHostKeyChecking no`
)

// GetSecret -
func GetSecret(r ReconcilerCommon, secretName string, secretNamespace string) (*corev1.Secret, string, error) {
	secret := &corev1.Secret{}

	err := r.GetClient().Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: secretNamespace}, secret)
	if err != nil {
		return nil, "", err
	}

	secretHash, err := ObjectHash(secret)
	if err != nil {
		return nil, "", fmt.Errorf("error calculating configuration hash: %v", err)
	}
	return secret, secretHash, nil
}

// GetSecrets -
func GetSecrets(r ReconcilerCommon, secretNamespace string, labelSelectorMap map[string]string) (*corev1.SecretList, error) {
	var secrets *corev1.SecretList

	secrets, err := r.GetKClient().CoreV1().Secrets(secretNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.FormatLabels(labelSelectorMap),
	})

	if err != nil {
		return secrets, err
	}

	return secrets, nil
}

// CreateOrUpdateSecret -
func CreateOrUpdateSecret(r ReconcilerCommon, obj metav1.Object, secret *corev1.Secret) (string, controllerutil.OperationResult, error) {

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.GetClient(), secret, func() error {

		err := controllerutil.SetControllerReference(obj, secret, r.GetScheme())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return "", op, fmt.Errorf("error create/updating secret: %v", err)
	}

	secretHash, err := ObjectHash(secret)
	if err != nil {
		return "", "", fmt.Errorf("error calculating configuration hash: %v", err)
	}

	return secretHash, op, err
}

// SSHKeySecret - func
func SSHKeySecret(name string, namespace string, labels map[string]string) (*corev1.Secret, error) {

	privateKey, err := GeneratePrivateKey(BITSIZE)
	if err != nil {
		return nil, err
	}

	publicKey, err := GeneratePublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	privateKeyPem := EncodePrivateKeyToPEM(privateKey)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Type: "Opaque",
		StringData: map[string]string{
			"identity":        privateKeyPem,
			"authorized_keys": publicKey,
			"config":          sshConfig,
		},
	}
	return secret, nil
}

// createOrUpdateSecret -
func createOrUpdateSecret(r ReconcilerCommon, obj metav1.Object, st Template) (string, controllerutil.OperationResult, error) {
	data := make(map[string][]byte)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        st.Name,
			Namespace:   st.Namespace,
			Annotations: st.Annotations,
		},
		Data: data,
	}

	if st.SecretType != "" {
		secret.Type = st.SecretType
	}

	// create or update the CM
	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.GetClient(), secret, func() error {
		secret.Labels = st.Labels
		// add data from templates
		dataString := GetTemplateData(st)
		// add provided custom data to dataString
		// Note: this can overwrite data rendered from GetTemplateData() if key is same
		if len(st.CustomData) > 0 {
			for k, v := range st.CustomData {
				dataString[k] = v
			}
		}

		for k, d := range dataString {
			data[k] = []byte(d)
		}
		secret.Data = data

		// Only set controller ref if namespaces are equal, else we hit an error
		if obj.GetNamespace() == secret.Namespace {
			if !st.SkipSetOwner {
				err := controllerutil.SetControllerReference(obj, secret, r.GetScheme())
				if err != nil {
					return err
				}
			}
		} else {
			// Set ownership labels that can be found by the respective controller kind
			ownerLabel := fmt.Sprintf("%s.%s", strings.ToLower(st.InstanceType), GroupLabel)
			labelSelector := map[string]string{
				ownerLabel + "/uid":       string(obj.GetUID()),
				ownerLabel + "/namespace": obj.GetNamespace(),
				ownerLabel + "/name":      obj.GetName(),
			}

			secret.GetObjectMeta().SetLabels(labels.Merge(secret.GetObjectMeta().GetLabels(), labelSelector))
		}

		return nil
	})

	if err != nil {
		return "", op, err
	}

	secretHash, err := ObjectHash(secret)
	if err != nil {
		return "", op, fmt.Errorf("error calculating configuration hash: %v", err)
	}

	return secretHash, op, nil
}

// createOrGetCustomSecret -
func createOrGetCustomSecret(r ReconcilerCommon, obj metav1.Object, st Template) (string, error) {
	// Check if this secret already exists
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        st.Name,
			Namespace:   st.Namespace,
			Labels:      st.Labels,
			Annotations: st.Annotations,
		},
		Data: map[string][]byte{},
	}

	if st.SecretType != "" {
		secret.Type = st.SecretType
	}

	foundSecret := &corev1.Secret{}
	err := r.GetClient().Get(context.TODO(), types.NamespacedName{Name: st.Name, Namespace: st.Namespace}, foundSecret)
	if err != nil && k8s_errors.IsNotFound(err) {
		err := controllerutil.SetControllerReference(obj, secret, r.GetScheme())
		if err != nil {
			return "", err
		}

		r.GetLogger().Info(fmt.Sprintf("Creating a new Secret %s in namespace %s", st.Namespace, st.Name))
		err = r.GetClient().Create(context.TODO(), secret)
		if err != nil {
			return "", err
		}
	} else {
		// use data from already existing custom secret
		secret.Data = foundSecret.Data
	}

	secretHash, err := ObjectHash(secret)
	if err != nil {
		return "", fmt.Errorf("error calculating configuration hash: %v", err)
	}

	return secretHash, nil
}

// EnsureSecrets - get all secrets required, verify they exist and add the hash to env and status
func EnsureSecrets(r ReconcilerCommon, obj metav1.Object, sts []Template, envVars *map[string]EnvSetter) error {
	var err error

	for _, s := range sts {
		var hash string
		var op controllerutil.OperationResult

		if s.Type != TemplateTypeCustom {
			hash, op, err = createOrUpdateSecret(r, obj, s)
		} else {
			hash, err = createOrGetCustomSecret(r, obj, s)
			// set op to OperationResultNone because createOrGetCustomSecret does not return an op
			// and it will add log entries bellow with none operation
			op = controllerutil.OperationResult(controllerutil.OperationResultNone)
		}
		if err != nil {
			return err
		}
		if op != controllerutil.OperationResultNone {
			r.GetLogger().Info(fmt.Sprintf("Secret %s successfully reconciled - operation: %s", s.Name, string(op)))
		}
		if envVars != nil {
			(*envVars)[s.Name] = EnvValue(hash)
		}
	}

	return nil
}

// DeleteSecretsWithLabel - Delete all secrets in namespace of the obj matching label selector
func DeleteSecretsWithLabel(r ReconcilerCommon, obj metav1.Object, labelSelectorMap map[string]string) error {
	err := r.GetClient().DeleteAllOf(
		context.TODO(),
		&corev1.Secret{},
		client.InNamespace(obj.GetNamespace()),
		client.MatchingLabels(labelSelectorMap),
	)
	if err != nil && !k8s_errors.IsNotFound(err) {
		err = fmt.Errorf("Error DeleteAllOf Secret: %v", err)
		return err
	}

	return nil
}

//
// GetDataFromSecret - Get data from Secret
//
// if the secret or data is not found, requeue after requeueTimeout in seconds
func GetDataFromSecret(
	r ReconcilerCommon,
	object client.Object,
	cond *ospdirectorv1beta1.Condition,
	conditionDetails ospdirectorv1beta1.ConditionDetails,
	secretName string,
	requeueTimeout int,
	key string,
) (string, ctrl.Result, error) {

	data := ""

	secret, _, err := GetSecret(r, secretName, object.GetNamespace())
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			cond.Message = fmt.Sprintf("%s secret does not exist: %v", secretName, err)
			cond.Reason = ospdirectorv1beta1.ConditionReason(conditionDetails.ConditionNotFoundReason)
			cond.Type = ospdirectorv1beta1.ConditionType(conditionDetails.ConditionNotFoundType)

			LogForObject(r, cond.Message, object)

			return data, ctrl.Result{RequeueAfter: time.Duration(requeueTimeout) * time.Second}, nil
		}
		cond.Message = fmt.Sprintf("Error getting %s Secret: %v", secretName, err)
		cond.Reason = ospdirectorv1beta1.ConditionReason(conditionDetails.ConditionErrordReason)
		cond.Type = ospdirectorv1beta1.ConditionType(conditionDetails.ConditionErrorType)
		err = WrapErrorForObject(cond.Message, object, err)

		return data, ctrl.Result{}, err
	}

	if key != "" {
		if val, ok := secret.Data[key]; !ok {
			cond.Message = fmt.Sprintf("%s not found in secret %s",
				key,
				secretName)
			cond.Reason = ospdirectorv1beta1.ConditionReason(conditionDetails.ConditionNotFoundReason)
			cond.Type = ospdirectorv1beta1.ConditionType(conditionDetails.ConditionErrorType)

			return data, ctrl.Result{}, fmt.Errorf(cond.Message)
		} else {
			data = strings.TrimSuffix(string(val), "\n")
		}
	}

	return data, ctrl.Result{}, nil
}
