package main

import (
	"context"
	"fmt"

	admissioncontrol "github.com/elithrar/admission-control"
	log "github.com/go-kit/kit/log"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	admission "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
)

func ValidateBaremetalSet(client dynamic.NamespaceableResourceInterface, logger log.Logger) admissioncontrol.AdmitFunc {
	// Return a function of type AdmitFunc
	return func(admissionReview *admission.AdmissionReview) (*admission.AdmissionResponse, error) {
		// Create an *admission.AdmissionResponse that denies by default.
		resp := &admission.AdmissionResponse{
			Allowed: false,
			Result:  &metav1.Status{},
		}

		// Create an object to deserialize our requests' object into
		bmSet := ospdirectorv1beta1.BaremetalSet{}
		deserializer := serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
		if _, _, err := deserializer.Decode(admissionReview.Request.Object.Raw, nil, &bmSet); err != nil {
			return nil, err
		}

		list, err := client.Namespace(bmSet.Namespace).List(context.Background(), metav1.ListOptions{})

		if err != nil {
			resp.Result.Message = err.Error()
			return resp, err
		}

		found := ""

		for _, item := range list.Items {
			spec := item.Object["spec"].(map[string]interface{})

			if item.GetName() != bmSet.Name && spec["role"] == bmSet.Spec.Role {
				found = item.GetName()
				break
			}
		}

		// If the role name is already in use, this BaremetalSet is invalid
		if found != "" {
			resp.Result.Message = fmt.Sprintf("Role \"%s\" is already in use by BaremetalSet %s", bmSet.Spec.Role, found)
		} else {
			resp.Allowed = true
		}

		return resp, nil
	}
}
