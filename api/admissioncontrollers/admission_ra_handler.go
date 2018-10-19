package admissioncontrollers

import (
	"bytes"
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
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

	isValid, err := CanRadixApplicationBeInserted(radixclient, radixApplication)
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

func CanRadixApplicationBeInserted(client radixclient.Interface, app *radixv1.RadixApplication) (bool, error) {
	errs := []error{}
	err := validateAppName(app.Name)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateExistEnvForComponentVariables(app)
	if err != nil {
		errs = append(errs, err)
	}

	err = validateDoesRRExist(client, app.Name)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) <= 0 {
		return true, nil
	}
	return false, concatErrors(errs)
}

func validateDoesRRExist(client radixclient.Interface, appName string) error {
	rr, err := client.RadixV1().RadixRegistrations("default").Get(appName, metav1.GetOptions{})
	if rr == nil {
		return fmt.Errorf("No app registered with that name %s", appName)
	}
	if err != nil {
		return fmt.Errorf("Could not get app registration obj %s", appName)
	}
	return nil
}

func validateExistEnvForComponentVariables(app *radixv1.RadixApplication) error {
	for _, component := range app.Spec.Components {
		for _, variable := range component.EnvironmentVariables {
			if !doesEnvExist(app, variable.Environment) {
				return fmt.Errorf("Env %s refered to by component variable %s is not defined", variable.Environment, component.Name)
			}
		}
	}

	return nil
}

func doesEnvExist(app *radixv1.RadixApplication, name string) bool {
	for _, env := range app.Spec.Environments {
		if env.Name == name {
			return true
		}
	}
	return false
}
