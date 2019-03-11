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

func ValidateRadixConfigurationChange(client kubernetes.Interface, radixclient radixclient.Interface, ar v1beta1.AdmissionReview) (bool, error) {
	log.Infof("admitting radix application configuration")

	radixApplication, err := decodeRadixConfiguration(ar)
	if err != nil {
		log.Warnf("radix app decoding failed")
		return false, err
	}
	log.Infof("radix application decoded")

	isValid, err := radixvalidators.CanRadixApplicationBeInserted(radixclient, radixApplication)
	if isValid {
		log.Infof("radix app %s was admitted", radixApplication.Name)
	} else {
		log.Warnf("radix app %s was rejected", radixApplication.Name)
	}
	return isValid, err
}

func decodeRadixConfiguration(ar v1beta1.AdmissionReview) (*v1.RadixApplication, error) {
	rrResource := metav1.GroupVersionResource{Group: "radix.equinor.com", Version: "v1", Resource: "radixapplications"}
	if ar.Request.Resource != rrResource {
		return nil, fmt.Errorf("resource was %s, expect resource to be %s", ar.Request.Resource, rrResource)
	}

	radixApplication := v1.RadixApplication{}
	if err := json.NewDecoder(bytes.NewReader(ar.Request.Object.Raw)).Decode(&radixApplication); err != nil {
		return nil, err
	}
	return &radixApplication, nil
}
