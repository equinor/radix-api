package test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	log "github.com/Sirupsen/logrus"

	"github.com/statoil/radix-api/api/router"
	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-api/models"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	kubernetes "k8s.io/client-go/kubernetes"
)

// Utils Instance variables
type Utils struct {
	client      kubernetes.Interface
	radixclient radixclient.Interface
	controllers []models.Controller
}

// NewTestUtils Constructor
func NewTestUtils(client kubernetes.Interface, radixclient radixclient.Interface, controllers ...models.Controller) Utils {
	return Utils{
		client,
		radixclient,
		controllers,
	}
}

// ExecuteRequest Helper method to issue a http request
func (tu *Utils) ExecuteRequest(method, endpoint string) <-chan *httptest.ResponseRecorder {
	return tu.ExecuteRequestWithPayload(method, endpoint, nil)
}

// ExecuteRequestWithPayload Helper method to issue a http request with payload
func (tu *Utils) ExecuteRequestWithPayload(method, endpoint string, payload []byte) <-chan *httptest.ResponseRecorder {
	var reader io.Reader

	if payload != nil {
		reader = bytes.NewReader(payload)
	}

	req, _ := http.NewRequest(method, endpoint, reader)
	req.Header.Add("Authorization", "bearer xyz")
	req.Header.Add("Accept", "application/json")

	response := make(chan *httptest.ResponseRecorder)
	go func() {
		rr := httptest.NewRecorder()
		router.NewServer("anyClusterName", NewKubeUtilMock(tu.client, tu.radixclient), tu.controllers...).ServeHTTP(rr, req)
		response <- rr
		close(response)
	}()

	return response

}

// GetErrorResponse Gets error repsonse
func GetErrorResponse(response *httptest.ResponseRecorder) (*utils.Error, error) {
	errorResponse := &utils.Error{}
	err := GetResponseBody(response, errorResponse)
	if err != nil {
		log.Infof("%v", err)
		return nil, err
	}

	return errorResponse, nil
}

// GetResponseBody Gets response payload as type
func GetResponseBody(response *httptest.ResponseRecorder, target interface{}) error {
	body, _ := ioutil.ReadAll(response.Body)
	return json.Unmarshal(body, target)
}

type kubeUtilMock struct {
	kubeFake  kubernetes.Interface
	radixFake radixclient.Interface
}

// NewKubeUtilMock Constructor
func NewKubeUtilMock(client kubernetes.Interface, radixclient radixclient.Interface) utils.KubeUtil {
	return &kubeUtilMock{
		client,
		radixclient,
	}
}

// GetOutClusterKubernetesClient Gets a kubefake client using the bearer token from the radix api client
func (ku *kubeUtilMock) GetOutClusterKubernetesClient(token string) (kubernetes.Interface, radixclient.Interface) {
	return ku.kubeFake, ku.radixFake
}

// GetInClusterKubernetesClient Gets a kubefake client using the config of the running pod
func (ku *kubeUtilMock) GetInClusterKubernetesClient() (kubernetes.Interface, radixclient.Interface) {
	return ku.kubeFake, ku.radixFake
}
