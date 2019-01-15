package admissioncontrollers_test

import (
	"encoding/json"
	"testing"

	kubefake "k8s.io/client-go/kubernetes/fake"

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
)

func Test_valid_rr_returns_true(t *testing.T) {
	kubeclient, client, validRR := validRRSetup()
	admissionReview := admissionReviewMock(validRR)
	isValid, err := ValidateRegistrationChange(kubeclient, client, admissionReview)

	assert.True(t, isValid)
	assert.Nil(t, err)
}

func Test_invalid_rr_admission_review(t *testing.T) {
	kubeclient, client, validRR := validRRSetup()
	t.Run("invalid resource type", func(t *testing.T) {
		admissionReview := admissionReviewMock(validRR)
		admissionReview.Request.Resource = metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "namespaces",
		}
		isValid, err := ValidateRegistrationChange(kubeclient, client, admissionReview)

		assert.False(t, isValid)
		assert.NotNil(t, err)
	})

	t.Run("invalid encoded rr", func(t *testing.T) {
		admissionReview := admissionReviewMock(validRR)
		admissionReview.Request.Object = runtime.RawExtension{Raw: []byte("some invalid encoded rr")}
		isValid, err := ValidateRegistrationChange(kubeclient, client, admissionReview)

		assert.False(t, isValid)
		assert.NotNil(t, err)
	})

	t.Run("invalid rr", func(t *testing.T) {
		validRR.Name = "|[[]§∞§|INVALID_CHARACTERS"
		admissionReview := admissionReviewMock(validRR)
		isValid, err := ValidateRegistrationChange(kubeclient, client, admissionReview)

		assert.False(t, isValid)
		assert.NotNil(t, err)
	})
}

func validRRSetup() (kubernetes.Interface, radixclient.Interface, *v1.RadixRegistration) {
	validRR, _ := utils.GetRadixRegistrationFromFile("testdata/sampleregistration.yaml")
	kubeclient := kubefake.NewSimpleClientset()
	client := radixfake.NewSimpleClientset()

	return kubeclient, client, validRR
}

func admissionReviewMock(reg *v1.RadixRegistration) v1beta1.AdmissionReview {
	obj := encodeRadixRegistration(reg)
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
				Resource: "radixregistrations",
			},
			Object: runtime.RawExtension{
				Raw: obj,
			},
		},
	}
}

func encodeRadixRegistration(reg *v1.RadixRegistration) []byte {
	ret, err := json.Marshal(reg)
	if err != nil {
		logrus.Errorln(err)
	}
	return ret
}
