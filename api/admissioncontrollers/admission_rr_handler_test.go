package admissioncontrollers_test

import (
	"encoding/json"
	"testing"

	kubefake "github.com/kubernetes/client-go/kubernetes/fake"

	"github.com/Sirupsen/logrus"
	. "github.com/statoil/radix-api/api/admissioncontrollers"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
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

type updateRRFunc func(rr *v1.RadixRegistration)

func Test_create_invalid_rr(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRR updateRRFunc
	}{
		{"to long app name", func(rr *v1.RadixRegistration) {
			rr.Name = "way.toooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo.long-app-name"
		}},
		{"invalid app name", func(rr *v1.RadixRegistration) { rr.Name = "invalid,char.appname" }},
		{"empty app name", func(rr *v1.RadixRegistration) { rr.Name = "" }},
		{"invalid ssh url ending", func(rr *v1.RadixRegistration) { rr.Spec.CloneURL = "git@github.com:auser/go-roman.gitblabla" }},
		{"invalid ssh url start", func(rr *v1.RadixRegistration) { rr.Spec.CloneURL = "asdfasdgit@github.com:auser/go-roman.git" }},
		{"invalid ssh url https", func(rr *v1.RadixRegistration) { rr.Spec.CloneURL = "https://github.com/auser/go-roman" }},
		{"empty ssh url", func(rr *v1.RadixRegistration) { rr.Spec.CloneURL = "" }},
		{"invalid ad group lenght", func(rr *v1.RadixRegistration) { rr.Spec.AdGroups = []string{"7552642f-asdff-fs43-23sf-3ab8f3742c16"} }},
		{"invalid ad group name", func(rr *v1.RadixRegistration) { rr.Spec.AdGroups = []string{"fg_some_group_name"} }},
		{"empty ad group list", func(rr *v1.RadixRegistration) { rr.Spec.AdGroups = []string{} }},
		{"empty ad group", func(rr *v1.RadixRegistration) { rr.Spec.AdGroups = []string{""} }},
	}

	kubeclient, client, validRR := validRRSetup()

	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			testcase.updateRR(validRR)
			admissionReview := admissionReviewMock(validRR)
			isValid, err := ValidateRegistrationChange(kubeclient, client, admissionReview)

			assert.False(t, isValid)
			assert.NotNil(t, err)
		})
	}

	t.Run("name already exist", func(t *testing.T) {
		client = radixfake.NewSimpleClientset(validRR)
		isValid, err := CanRadixRegistrationBeInserted(client, validRR)

		assert.False(t, isValid)
		assert.NotNil(t, err)
	})
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
