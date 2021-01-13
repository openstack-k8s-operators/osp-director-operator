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

	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
			Name:      st.Name,
			Namespace: st.Namespace,
		},
		Data: data,
	}

	// create or update the CM
	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.GetClient(), secret, func() error {
		secret.Labels = st.Labels
		// add data from templates
		dataString := getTemplateData(st)
		for k, d := range dataString {
			data[k] = []byte(d)
		}
		secret.Data = data

		// Only set controller ref if namespaces are equal, else we hit an error
		if obj.GetNamespace() == secret.Namespace {
			err := controllerutil.SetControllerReference(obj, secret, r.GetScheme())
			if err != nil {
				return err
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
			Name:      st.Name,
			Namespace: st.Namespace,
			Labels:    st.Labels,
		},
		Data: map[string][]byte{},
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
