package admissioncontrollers

import (
	"encoding/json"
	"net/http"

	"github.com/equinor/radix-api/api/utils"

	"github.com/equinor/radix-api/models"
	log "github.com/sirupsen/logrus"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"

	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

const RootPath = "/admissioncontrollers"

type admissionController struct {
}

// NewAdmissionController Constructor
func NewAdmissionController() models.Controller {
	return &admissionController{}
}

// GetRoutes List the supported routes of this handler
func (ac *admissionController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:   RootPath + "/registrations",
			Method: "POST",
			HandlerFunc: func(clients models.Clients, w http.ResponseWriter, r *http.Request) {
				serve(clients.InClusterClient, clients.InClusterRadixClient, w, r, ValidateRegistrationChange)
			},
		},
		models.Route{
			Path:   RootPath + "/applications",
			Method: "POST",
			HandlerFunc: func(clients models.Clients, w http.ResponseWriter, r *http.Request) {
				serve(clients.InClusterClient, clients.InClusterRadixClient, w, r, ValidateRadixConfigurationChange)
			},
		},
	}

	return routes
}

// GetSubscriptions Lists subscriptions this handler offers
func (ac *admissionController) GetSubscriptions() models.Subscriptions {
	subscriptions := models.Subscriptions{}

	return subscriptions
}

type admitFunc func(client kubernetes.Interface, radixclient radixclient.Interface, ar v1beta1.AdmissionReview) (bool, error)

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
