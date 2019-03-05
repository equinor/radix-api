package admissioncontrollers

import (
	"bytes"
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Validates a new app registration
func ValidateRegistrationChange(client kubernetes.Interface, radixclient radixclient.Interface, ar v1beta1.AdmissionReview) (bool, error) {
	log.Infof("admitting radix registrations")

	radixRegistration, err := decodeRadixRegistration(ar)
	if err != nil {
		log.Warnf("radix reg decoding failed")
		return false, err
	}
	log.Infof("radix registration decoded")

	isValid, err := radixvalidators.CanRadixRegistrationBeUpdated(radixclient, radixRegistration)
	if isValid {
		log.Infof("radix reg %s was admitted", radixRegistration.Name)
	} else {
		log.Warnf("radix reg %s was rejected", radixRegistration.Name)
	}
	return isValid, err
}

func decodeRadixRegistration(ar v1beta1.AdmissionReview) (*v1.RadixRegistration, error) {
	rrResource := metav1.GroupVersionResource{Group: "radix.equinor.com", Version: "v1", Resource: "radixregistrations"}
	if ar.Request.Resource != rrResource {
		return nil, fmt.Errorf("resource was %s, expect resource to be %s", ar.Request.Resource, rrResource)
	}

	radixRegistration := v1.RadixRegistration{}
	if err := json.NewDecoder(bytes.NewReader(ar.Request.Object.Raw)).Decode(&radixRegistration); err != nil {
		return nil, err
	}
	return &radixRegistration, nil
}
