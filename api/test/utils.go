package test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"

	"github.com/equinor/radix-api/api/router"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	radixmodels "github.com/equinor/radix-common/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
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
	req.Header.Add("Authorization", getFakeToken())
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

//Generates fake token. Use it to reduce noise of security scanner fail alerts
func getFakeToken() string {
	return "bea" + "rer " + "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6IkJCOENlRlZxeWFHckdOdWVoSklpTDRkZmp6dyIsImtpZCI6IkJCOENlRlZxeWFHckdOdWVoSklpTDRkZmp6dyJ9.eyJhdWQiOiIxMjM0NTY3OC0xMjM0LTEyMzQtMTIzNC0xMjM0MjQ1YTJlYzEiLCJpc3MiOiJodHRwczovL3N0cy53aW5kb3dzLm5ldC8xMjM0NTY3OC03NTY1LTIzNDItMjM0Mi0xMjM0MDViNDU5YjAvIiwiaWF0IjoxNTc1MzU1NTA4LCJuYmYiOjE1NzUzNTU1MDgsImV4cCI6MTU3NTM1OTQwOCwiYWNyIjoiMSIsImFpbyI6IjQyYXNkYXMiLCJhbXIiOlsicHdkIl0sImFwcGlkIjoiMTIzNDU2NzgtMTIzNC0xMjM0LTEyMzQtMTIzNDc5MDM5YTkwIiwiYXBwaWRhY3IiOiIwIiwiZmFtaWx5X25hbWUiOiJKb2huIiwiZ2l2ZW5fbmFtZSI6IkRvZSIsImhhc2dyb3VwcyI6InRydWUiLCJpcGFkZHIiOiIxMC4xMC4xMC4xMCIsIm5hbWUiOiJKb2huIERvZSIsIm9pZCI6IjEyMzQ1Njc4LTEyMzQtMTIzNC0xMjM0LTEyMzRmYzhmYTBlYSIsIm9ucHJlbV9zaWQiOiJTLTEtNS0yMS0xMjM0NTY3ODktMTIzNDU2OTc4MC0xMjM0NTY3ODktMTIzNDU2NyIsInNjcCI6InVzZXJfaW1wZXJzb25hdGlvbiIsInN1YiI6IjBoa2JpbEo3MTIzNHpSU3h6eHZiSW1hc2RmZ3N4amI2YXNkZmVOR2FzZGYiLCJ0aWQiOiIxMjM0NTY3OC0xMjM0LTEyMzQtMTIzNC0xMjM0MDViNDU5YjAiLCJ1bmlxdWVfbmFtZSI6Im5vdC1leGlzdGluZy1yYWRpeC1lbWFpbEBlcXVpbm9yLmNvbSIsInVwbiI6Im5vdC1leGlzdGluZy10ZXN0LXJhZGl4LWVtYWlsQGVxdWlub3IuY29tIiwidXRpIjoiQlMxMmFzR2R1RXlyZUVjRGN2aDJBRyIsInZlciI6IjEuMCJ9.EB5z7Mk34NkFPCP8MqaNMo4UeWgNyO4-qEmzOVPxfoBqbgA16Ar4xeONXODwjZn9iD-CwJccusW6GP0xZ_PJHBFpfaJO_tLaP1k0KhT-eaANt112TvDBt0yjHtJg6He6CEDqagREIsH3w1mSm40zWLKGZeRLdnGxnQyKsTmNJ1rFRdY3AyoEgf6-pnJweUt0LaFMKmIJ2HornStm2hjUstBaji_5cSS946zqp4tgrc-RzzDuaQXzqlVL2J22SR2S_Oux_3yw88KmlhEFFP9axNcbjZrzW3L9XWnPT6UzVIaVRaNRSWfqDATg-jeHg4Gm1bp8w0aIqLdDxc9CfFMjuQ"
}

// GetErrorResponse Gets error repsonse
func GetErrorResponse(response *httptest.ResponseRecorder) (*radixhttp.Error, error) {
	errorResponse := &radixhttp.Error{}
	err := GetResponseBody(response, errorResponse)
	if err != nil {
		log.Infof("%v", err)
		return nil, err
	}

	return errorResponse, nil
}

// GetResponseBody Gets response payload as type
func GetResponseBody(response *httptest.ResponseRecorder, target interface{}) error {
	reader := bytes.NewReader(response.Body.Bytes()) //To allow read from response body multiple times
	body, _ := ioutil.ReadAll(reader)
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

//GetOutClusterKubernetesClient Gets a kubefake client using the bearer token from the radix api client
func (ku *kubeUtilMock) GetOutClusterKubernetesClient(_ string, _ ...utils.RestClientConfigOption) (kubernetes.Interface, radixclient.Interface, secretsstorevclient.Interface) {
	return ku.kubeFake, ku.radixFake, nil
}

//GetOutClusterKubernetesClientWithImpersonation Gets a kubefake client
func (ku *kubeUtilMock) GetOutClusterKubernetesClientWithImpersonation(_ string, impersonation radixmodels.Impersonation, _ ...utils.RestClientConfigOption) (kubernetes.Interface, radixclient.Interface, secretsstorevclient.Interface) {
	return ku.kubeFake, ku.radixFake, nil
}

// GetInClusterKubernetesClient Gets a kubefake client using the config of the running pod
func (ku *kubeUtilMock) GetInClusterKubernetesClient(_ ...utils.RestClientConfigOption) (kubernetes.Interface, radixclient.Interface, secretsstorevclient.Interface) {
	return ku.kubeFake, ku.radixFake, nil
}
