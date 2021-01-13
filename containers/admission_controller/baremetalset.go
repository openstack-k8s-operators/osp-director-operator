package main

import (
	admissioncontrol "github.com/elithrar/admission-control"
	log "github.com/go-kit/kit/log"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	admission "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func ValidateBaremetalSet(logger log.Logger) admissioncontrol.AdmitFunc {
	// Return a function of type AdmitFunc
	return func(admissionReview *admission.AdmissionReview) (*admission.AdmissionResponse, error) {
		//kind := admissionReview.Request.Kind.Kind

		// Create an *admission.AdmissionResponse that denies by default.
		resp := &admission.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "You cannot do that!",
			},
		}

		// Create an object to deserialize our requests' object into
		bmSet := ospdirectorv1beta1.BaremetalSet{}
		deserializer := serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
		if _, _, err := deserializer.Decode(admissionReview.Request.Object.Raw, nil, &bmSet); err != nil {
			return nil, err
		}

		// If no existing BaremetalSets with this CR's role value, allow admission
		//resp.Allowed = true

		logger.Log("YO", resp.Allowed)

		if resp.Allowed == false {
			return resp, nil
		}

		return resp, nil
	}
}
