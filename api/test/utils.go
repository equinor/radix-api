package test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	log "github.com/sirupsen/logrus"

	"github.com/equinor/radix-api/api/router"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
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
	return tu.ExecuteRequestWithParameters(method, endpoint, nil)
}

func (tu *Utils) ExecuteUnAuthorizedRequest(method, endpoint string) <-chan *httptest.ResponseRecorder {
	var reader io.Reader

	req, _ := http.NewRequest(method, endpoint, reader)

	response := make(chan *httptest.ResponseRecorder)
	go func() {
		rr := httptest.NewRecorder()
		router.NewServer("anyClusterName", NewKubeUtilMock(tu.client, tu.radixclient), tu.controllers...).ServeHTTP(rr, req)
		response <- rr
		close(response)
	}()

	return response
}

// ExecuteRequestWithParameters Helper method to issue a http request with payload
func (tu *Utils) ExecuteRequestWithParameters(method, endpoint string, parameters interface{}) <-chan *httptest.ResponseRecorder {
	var reader io.Reader

	if parameters != nil {
		payload, _ := json.Marshal(parameters)
		reader = bytes.NewReader(payload)
	}

	req, _ := http.NewRequest(method, endpoint, reader)
	req.Header.Add("Authorization", "bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6IkJCOENlRlZxeWFHckdOdWVoSklpTDRkZmp6dyIsImtpZCI6IkJCOENlRlZxeWFHckdOdWVoSklpTDRkZmp6dyJ9.eyJhdWQiOiIxMjM0NTY3OC0xMjM0LTEyMzQtMTIzNC0xMjM0MjQ1YTJlYzEiLCJpc3MiOiJodHRwczovL3N0cy53aW5kb3dzLm5ldC8xMjM0NTY3OC03NTY1LTIzNDItMjM0Mi0xMjM0MDViNDU5YjAvIiwiaWF0IjoxNTc1MzU1NTA4LCJuYmYiOjE1NzUzNTU1MDgsImV4cCI6MTU3NTM1OTQwOCwiYWNyIjoiMSIsImFpbyI6IjQyYXNkYXMiLCJhbXIiOlsicHdkIl0sImFwcGlkIjoiMTIzNDU2NzgtMTIzNC0xMjM0LTEyMzQtMTIzNDc5MDM5YTkwIiwiYXBwaWRhY3IiOiIwIiwiZmFtaWx5X25hbWUiOiJSYWRpeCIsImdpdmVuX25hbWUiOiJBIFJhZGl4IFVzZXIiLCJoYXNncm91cHMiOiJ0cnVlIiwiaXBhZGRyIjoiMTQzLjk3LjIuMTI5IiwibmFtZSI6IkEgUmFkaXggVXNlciIsIm9pZCI6IjEyMzQ1Njc4LTEyMzQtMTIzNC0xMjM0LTEyMzRmYzhmYTBlYSIsIm9ucHJlbV9zaWQiOiJTLTEtNS0yMS0xMjM0NTY3ODktMTIzNDU2OTc4MC0xMjM0NTY3ODktMTIzNDU2NyIsInNjcCI6InVzZXJfaW1wZXJzb25hdGlvbiIsInN1YiI6IjBoa2JpbEo3MTIzNHpSU3h6eHZiSW1hc2RmZ3N4amI2YXNkZmVOR2FzZGYiLCJ0aWQiOiIxMjM0NTY3OC0xMjM0LTEyMzQtMTIzNC0xMjM0MDViNDU5YjAiLCJ1bmlxdWVfbmFtZSI6IlJBRElYQGVxdWlub3IuY29tIiwidXBuIjoiUkFESVhAZXF1aW5vci5jb20iLCJ1dGkiOiJCUzEyYXNHZHVFeXJlRWNEY3ZoMkFHIiwidmVyIjoiMS4wIn0=.inP8fD7")
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
	log.Infof(string(body))

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

// GetOutClusterKubernetesClient Gets a kubefake client
func (ku *kubeUtilMock) GetOutClusterKubernetesClientWithImpersonation(token string, impersonation models.Impersonation) (kubernetes.Interface, radixclient.Interface) {
	return ku.kubeFake, ku.radixFake
}

// GetInClusterKubernetesClient Gets a kubefake client using the config of the running pod
func (ku *kubeUtilMock) GetInClusterKubernetesClient() (kubernetes.Interface, radixclient.Interface) {
	return ku.kubeFake, ku.radixFake
}
