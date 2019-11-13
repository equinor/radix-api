package buildsecrets

import (
	"strings"

	buildSecretsModels "github.com/equinor/radix-api/api/buildsecrets/models"
	sharedModels "github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"

	"github.com/equinor/radix-api/api/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

// Handler Instance variables
type Handler struct {
	userAccount    sharedModels.Account
	serviceAccount sharedModels.Account
}

// Init Constructor
func Init(
	client kubernetes.Interface,
	radixClient radixclient.Interface,
	inClusterClient kubernetes.Interface,
	inClusterRadixClient radixclient.Interface) Handler {

	return Handler{
		userAccount: sharedModels.Account{
			Client:      client,
			RadixClient: radixClient,
		},
		serviceAccount: sharedModels.Account{
			Client:      inClusterClient,
			RadixClient: inClusterRadixClient,
		}}
}

// ChangeBuildSecret handler to modify the build secret
func (sh Handler) ChangeBuildSecret(appName, secretName, secretValue string) error {
	if strings.TrimSpace(secretValue) == "" {
		return utils.ValidationError("Secret", "New secret value is empty")
	}

	secretObject, err := sh.userAccount.Client.CoreV1().Secrets(k8sObjectUtils.GetAppNamespace(appName)).Get(defaults.BuildSecretsName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return utils.TypeMissingError("Build secrets object does not exist", err)
	}

	if err != nil {
		return utils.UnexpectedError("Failed getting build secret object", err)
	}

	if secretObject.Data == nil {
		secretObject.Data = make(map[string][]byte)
	}

	secretObject.Data[secretName] = []byte(secretValue)
	_, err = sh.userAccount.Client.CoreV1().Secrets(k8sObjectUtils.GetAppNamespace(appName)).Update(secretObject)
	if err != nil {
		return err
	}

	return nil
}

// GetBuildSecrets Lists build secrets for application
func (sh Handler) GetBuildSecrets(appName string) ([]buildSecretsModels.BuildSecret, error) {
	ra, err := sh.userAccount.RadixClient.RadixV1().RadixApplications(k8sObjectUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	secretObject, err := sh.userAccount.Client.CoreV1().Secrets(k8sObjectUtils.GetAppNamespace(appName)).Get(defaults.BuildSecretsName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil, utils.TypeMissingError("Build secrets object does not exist", err)
	}

	buildSecrets := make([]buildSecretsModels.BuildSecret, 0)
	for _, secretName := range ra.Spec.Build.Secrets {
		secretStatus := buildSecretsModels.Pending.String()
		secretValue := strings.TrimSpace(string(secretObject.Data[secretName]))
		if !strings.EqualFold(secretValue, defaults.BuildSecretDefaultData) {
			secretStatus = buildSecretsModels.Consistent.String()
		}

		buildSecrets = append(buildSecrets, buildSecretsModels.BuildSecret{
			Name:   secretName,
			Status: secretStatus,
		})
	}

	return buildSecrets, nil
}
