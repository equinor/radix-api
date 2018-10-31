package applications

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	controllertest "github.com/statoil/radix-api/api/test"
	commontest "github.com/statoil/radix-operator/pkg/apis/test"
	builders "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func setupTest() (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := fake.NewSimpleClientset()

	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient)
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, NewApplicationController())

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient
}

func TestIsDeployKeyValid(t *testing.T) {
	commonTestUtils, controllerTestUtils, kubeclient, _ := setupTest()
	commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("some-app"))

	t.Run("missing rr", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-nonexisting-app"))
		response := <-responseChannel

		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("job succeeded", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-app"))
		setStatusOfJob(kubeclient, "some-app-app", true)

		response := <-responseChannel
		assert.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("job failed", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/deploykey-valid", "some-app"))
		setStatusOfJob(kubeclient, "some-app-app", false)

		response := <-responseChannel
		assert.Equal(t, http.StatusUnprocessableEntity, response.Code)

		errorResponse, _ := controllertest.GetErrorResponse(response)
		assert.Equal(t, "Deploy key was invalid", errorResponse.Message)
	})
}

func setStatusOfJob(kubeclient kubernetes.Interface, appNamespace string, succeededStatus bool) {
	time.Sleep(500 * time.Millisecond)
	jobs, _ := kubeclient.BatchV1().Jobs(appNamespace).List(metav1.ListOptions{})
	job := jobs.Items[0]

	if succeededStatus {
		job.Status.Succeeded = int32(1)
	} else {
		job.Status.Failed = int32(1)
	}

	kubeclient.BatchV1().Jobs(appNamespace).Update(&job)
}
