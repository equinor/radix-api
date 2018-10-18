package admissioncontrollers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/statoil/radix-api/api/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-api/models"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"

	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

const RootPath = "/admissioncontrollers"

// GetRoutes List the supported routes of this handler
func GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:   RootPath + "/registrations",
			Method: "POST",
			HandlerFunc: func(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
				serve(client, radixclient, w, r, ValidateRegistrationCreation)
			},
		},
	}

	return routes
}

type admitFunc func(client kubernetes.Interface, radixclient radixclient.Interface, ar v1beta1.AdmissionReview) (bool, error)

// Validates a new app registration
func ValidateRegistrationCreation(client kubernetes.Interface, radixclient radixclient.Interface, ar v1beta1.AdmissionReview) (bool, error) {
	log.Infof("admitting radix registrations")

	radixRegistration, err := decodeRadixRegistration(ar)
	if err != nil {
		log.Warnf("radix reg decoding failed")
		return false, err
	}
	log.Infof("radix registration decoded")

	isValid, err := CanRadixRegistrationBeUpdated(radixclient, radixRegistration)
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

func serve(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request, admit admitFunc) {
	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		log.Errorf("contentType=%s, expect application/json", contentType)
		return
	}

	var reviewResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if err := json.NewDecoder(r.Body).Decode(&ar); err != nil {
		reviewResponse = toAdmissionResponse(err)
	} else {
		isValid, err := admit(client, radixclient, ar)
		if isValid {
			reviewResponse = &v1beta1.AdmissionResponse{
				Allowed: true,
			}
		} else {
			reviewResponse = toAdmissionResponse(err)
		}
	}

	response := v1beta1.AdmissionReview{}
	if reviewResponse != nil {
		response.Response = reviewResponse
		response.Response.UID = ar.Request.UID
	}
	// reset the Object and OldObject, they are not needed in a response.
	ar.Request.Object = runtime.RawExtension{}
	ar.Request.OldObject = runtime.RawExtension{}

	utils.JSONResponse(w, r, response)
}

func toAdmissionResponse(err error) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}
