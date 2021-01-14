#!/bin/bash

# CA pairs (k8s API server):
# ca.key, ca.crt
# 
# Webserver pairs (pod):
# webhook-server-tls.key, webhook-server-tls.crt

tmpdir=$(mktemp -d)

# Generate the CA cert and private key
openssl req -nodes -new -x509 -keyout ${tmpdir}/ca.key -out ${tmpdir}/ca.crt -subj "/CN=OSP Director Operator Admission Controller CA"
# Generate the private key for the webhook server
openssl genrsa -out ${tmpdir}/webhook-server-tls.key 2048
# Generate a Certificate Signing Request (CSR) for the private key, and sign it with the private key of the CA.
openssl req -new -key ${tmpdir}/webhook-server-tls.key -subj "/CN=osp-webhook.openstack.svc" \
    | openssl x509 -req -CA ${tmpdir}/ca.crt -CAkey ${tmpdir}/ca.key -CAcreateserial -out ${tmpdir}/webhook-server-tls.crt

oc delete validatingwebhookconfiguration baremetalset-validation-webhook

cat <<EOF | oc apply -f -
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: baremetalset-validation-webhook
webhooks:
  - name: baremetalset-validation-service.osp-director-operator.services
    failurePolicy: Fail
    clientConfig:
      service:
        name: osp-webhook
        namespace: openstack
        path: "/osp-director-operator/validate-baremetalset"
      caBundle: $(openssl base64 -A <"${tmpdir}/ca.crt")
    rules:
      - operations: [ "CREATE","UPDATE" ]
        apiGroups: ["osp-director.openstack.org"]
        apiVersions: ["v1beta1"]
        resources: ["baremetalsets"]
    sideEffects: None
    admissionReviewVersions: 
    - v1beta1
EOF

oc delete secret osp-director-operator-admission-controller-secret

oc create secret tls osp-director-operator-admission-controller-secret --cert="${tmpdir}"/webhook-server-tls.crt --key="${tmpdir}"/webhook-server-tls.key

oc delete -f deployment.yaml --ignore-not-found

oc apply -f deployment.yaml
