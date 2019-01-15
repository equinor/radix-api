package admissioncontrollers_test

import (
	"encoding/json"
	"testing"

	"github.com/Sirupsen/logrus"
	. "github.com/equinor/radix-api/api/admissioncontrollers"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_valid_ra_returns_true(t *testing.T) {
	kubeclient, client, validRA := validRASetup()
	admissionReview := admissionReviewMockApp(validRA)
	isValid, err := ValidateRadixConfigurationChange(kubeclient, client, admissionReview)

	assert.True(t, isValid)
	assert.Nil(t, err)
}

func Test_invalid_ra_admission_review(t *testing.T) {
	kubeclient, client, validRA := validRASetup()
	t.Run("invalid resource type", func(t *testing.T) {
		admissionReview := admissionReviewMockApp(validRA)
		admissionReview.Request.Resource = metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "namespaces",
		}
		isValid, err := ValidateRadixConfigurationChange(kubeclient, client, admissionReview)

		assert.False(t, isValid)
		assert.NotNil(t, err)
	})

	t.Run("invalid encoded ra", func(t *testing.T) {
		admissionReview := admissionReviewMockApp(validRA)
		admissionReview.Request.Object = runtime.RawExtension{Raw: []byte("some invalid encoded rr")}
		isValid, err := ValidateRadixConfigurationChange(kubeclient, client, admissionReview)

		assert.False(t, isValid)
		assert.NotNil(t, err)
	})

	t.Run("invalid ra", func(t *testing.T) {
		validRA.Name = "∞§|[][|§∞€INVALID_NAME"
		admissionReview := admissionReviewMockApp(validRA)
		isValid, err := ValidateRadixConfigurationChange(kubeclient, client, admissionReview)

		assert.False(t, isValid)
		assert.NotNil(t, err)
	})
}

func validRASetup() (kubernetes.Interface, radixclient.Interface, *v1.RadixApplication) {
	validRA, _ := utils.GetRadixApplication("testdata/radixconfig.yaml")
	validRR, _ := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	kubeclient := kubefake.NewSimpleClientset()
	client := radixfake.NewSimpleClientset(validRR)

	return kubeclient, client, validRA
}

func admissionReviewMockApp(app *v1.RadixApplication) v1beta1.AdmissionReview {
	obj := encodeRadixApplication(app)
	return v1beta1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind: "AdmissionReview",
		},
		Request: &v1beta1.AdmissionRequest{
			UID: "e911857d-c318-11e8-bbad-025000000001",
			Kind: metav1.GroupVersionKind{
				Kind: "Namespace",
			},
			Operation: "CREATE",
			Resource: metav1.GroupVersionResource{
				Group:    "radix.equinor.com",
				Version:  "v1",
				Resource: "radixapplications",
			},
			Object: runtime.RawExtension{
				Raw: obj,
			},
		},
	}
}

func encodeRadixApplication(app *v1.RadixApplication) []byte {
	ret, err := json.Marshal(app)
	if err != nil {
		logrus.Errorln(err)
	}
	return ret
}
