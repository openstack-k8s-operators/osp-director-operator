#!/bin/bash

# cat <<EOF | cfssl genkey - | cfssljson -bare server
# {
#   "hosts": [
#     "osp-webhook.openstack.svc.cluster.local",
#     "osp-webhook.openstack.svc",
#     "osp-webhook"
#   ],
#   "CN": "osp-webhook.openstack.svc",
#   "key": {
#     "algo": "ecdsa",
#     "size": 256
#   }
# }
# EOF

# service=osp-webhook
# namespace=openstack

# csrName=${service}.${namespace}
# tmpdir=$(mktemp -d)
# echo "creating certs in tmpdir ${tmpdir} "

# cat <<EOF >> "${tmpdir}"/csr.conf
# [req]
# req_extensions = v3_req
# distinguished_name = req_distinguished_name
# [req_distinguished_name]
# [ v3_req ]
# basicConstraints = CA:FALSE
# keyUsage = nonRepudiation, digitalSignature, keyEncipherment
# extendedKeyUsage = serverAuth
# subjectAltName = @alt_names
# [alt_names]
# DNS.1 = ${service}
# DNS.2 = ${service}.${namespace}
# DNS.3 = ${service}.${namespace}.svc
# EOF

# openssl genrsa -out "${tmpdir}"/server-key.pem 2048
# openssl req -new -key "${tmpdir}"/server-key.pem -subj "/CN=${service}.${namespace}.svc" -out "${tmpdir}"/server.csr -config "${tmpdir}"/csr.conf

tmpdir=$(mktemp -d)

# Generate the CA cert and private key
openssl req -nodes -new -x509 -keyout ${tmpdir}/ca.key -out ${tmpdir}/ca.crt -subj "/CN=OSP Director Operator Admission Controller CA"
# Generate the private key for the webhook server
openssl genrsa -out ${tmpdir}/webhook-server-tls.key 2048
# Generate a Certificate Signing Request (CSR) for the private key, and sign it with the private key of the CA.
openssl req -new -key ${tmpdir}/webhook-server-tls.key -subj "/CN=osp-webhook.openstack.svc" \
    | openssl x509 -req -CA ${tmpdir}/ca.crt -CAkey ${tmpdir}/ca.key -CAcreateserial -out ${tmpdir}/webhook-server-tls.crt

# CA pairs (k8s API server):
# ca.key, ca.crt
# 
# Webserver pairs (pod):
# webhook-server-tls.key, webhook-server-tls.crt

# oc delete csr osp-director-operator-admission-controller-service --ignore-not-found

# cat <<EOF | oc apply -f -
# apiVersion: certificates.k8s.io/v1beta1
# kind: CertificateSigningRequest
# metadata:
#   name: osp-director-operator-admission-controller-service
# spec:
#   groups:
#   - system:authenticated
#   request: $(< "${tmpdir}"/server.csr base64 | tr -d '\n')
#   #signerName: kubernetes.io/kubelet-serving
#   usages:
#   - digital signature
#   - key encipherment
#   - server auth
# EOF

# oc adm certificate approve osp-director-operator-admission-controller-service

# oc get csr osp-director-operator-admission-controller-service -o jsonpath='{.status.certificate}' | openssl base64 -d -A > "${tmpdir}"/server.crt

oc delete validatingwebhookconfiguration baremetalset-validation-webhook

cat <<EOF | oc apply -f -
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: baremetalset-validation-webhook
webhooks:
  - name: baremetalset-validation-service.osp-director-operator.services
    # <snip, for brevity>
    failurePolicy: Fail
    clientConfig:
      service:
        # This is the hostname our certificate needs in its Subject Alternative
        # Name array - name.namespace.svc
        # If the certificate does NOT have this name, TLS validation will fail.
        name: osp-webhook
        namespace: openstack
        path: "/osp-director-operator/validate-baremetalset"
      # This will be the CA cert from your k8s cluster, or the CA cert you
      # generated if you took the DIY approach.
      caBundle: $(openssl base64 -A <"${tmpdir}/ca.crt")
      #caBundle: $(oc config view --raw --minify --flatten -o jsonpath='{.clusters[].cluster.certificate-authority-data}')
      #caBundle: $(oc get secrets -o jsonpath="{.items[?(@.metadata.annotations['kubernetes\.io/service-account\.name']=='default')].data.ca\.crt}" | cut -d ' ' -f 1)
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
