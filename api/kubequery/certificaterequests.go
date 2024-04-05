package kubequery

import (
	"context"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//

// GetEventsForEnvironment returns all Events for the specified application and environment.
func GetCertificateRequestsForEnvironment(ctx context.Context, client certclient.Interface, appName, envName string) ([]cmv1.CertificateRequest, error) {
	ns := utils.GetEnvironmentNamespace(appName, envName)
	certReqList, err := client.CertmanagerV1().CertificateRequests(ns).List(ctx, v1.ListOptions{LabelSelector: labels.ForApplicationName(appName).AsSelector().String()})
	if err != nil {
		return nil, err
	}
	return certReqList.Items, nil
}
