package secrets

import (
	"context"
	"strings"

	"github.com/equinor/radix-api/api/deployments"
	"github.com/equinor/radix-api/api/secrets/models"
	"github.com/equinor/radix-api/api/secrets/suffix"
	"github.com/equinor/radix-api/api/utils/labelselector"
	sortUtils "github.com/equinor/radix-api/api/utils/sort"
	apiModels "github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

const (
	secretDefaultData          = "xx"
	secretStoreCsiManagedLabel = "secrets-store.csi.k8s.io/managed"
	k8sJobNameLabel            = "job-name" // A label that k8s automatically adds to a Pod created by a Job
)

type podNameToSecretVersionMap map[string]string
type secretIdToPodNameToSecretVersionMap map[string]podNameToSecretVersionMap

// SecretHandlerOptions defines a configuration function
type SecretHandlerOptions func(*SecretHandler)

// WithAccounts configures all SecretHandler fields
func WithAccounts(accounts apiModels.Accounts) SecretHandlerOptions {
	return func(eh *SecretHandler) {
		eh.userAccount = accounts.UserAccount
		eh.serviceAccount = accounts.ServiceAccount
		eh.deployHandler = deployments.Init(accounts)
	}
}

// SecretHandler Instance variables
type SecretHandler struct {
	userAccount    apiModels.Account
	serviceAccount apiModels.Account
	deployHandler  deployments.DeployHandler
}

// Init Constructor.
// Use the WithAccounts configuration function to configure a 'ready to use' SecretHandler.
// SecretHandlerOptions are processed in the sequence they are passed to this function.
func Init(opts ...SecretHandlerOptions) SecretHandler {
	eh := SecretHandler{}

	for _, opt := range opts {
		opt(&eh)
	}

	return eh
}

// ChangeComponentSecret handler for HandleChangeComponentSecret
func (eh SecretHandler) ChangeComponentSecret(ctx context.Context, appName, envName, componentName, secretName string, componentSecret models.SecretParameters) error {
	newSecretValue := componentSecret.SecretValue
	if strings.TrimSpace(newSecretValue) == "" {
		return radixhttp.ValidationError("Secret", "New secret value is empty")
	}

	ns := operatorutils.GetEnvironmentNamespace(appName, envName)

	var secretObjName, partName string

	if strings.HasSuffix(secretName, suffix.ExternalDNSTLSCert) {
		// This is the cert part of the TLS secret
		secretObjName = strings.TrimSuffix(secretName, suffix.ExternalDNSTLSCert)
		partName = corev1.TLSCertKey

	} else if strings.HasSuffix(secretName, suffix.ExternalDNSTLSKey) {
		// This is the key part of the TLS secret
		secretObjName = strings.TrimSuffix(secretName, suffix.ExternalDNSTLSKey)
		partName = corev1.TLSPrivateKeyKey

	} else if strings.HasSuffix(secretName, defaults.BlobFuseCredsAccountKeyPartSuffix) {
		// This is the account key part of the blobfuse cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.BlobFuseCredsAccountKeyPartSuffix)
		partName = defaults.BlobFuseCredsAccountKeyPart

	} else if strings.HasSuffix(secretName, defaults.BlobFuseCredsAccountNamePartSuffix) {
		// This is the account name part of the blobfuse cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.BlobFuseCredsAccountNamePartSuffix)
		partName = defaults.BlobFuseCredsAccountNamePart

	} else if strings.HasSuffix(secretName, defaults.CsiAzureCredsAccountKeyPartSuffix) {
		// This is the account key part of the Csi Azure volume cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.CsiAzureCredsAccountKeyPartSuffix)
		partName = defaults.CsiAzureCredsAccountKeyPart

	} else if strings.HasSuffix(secretName, defaults.CsiAzureCredsAccountNamePartSuffix) {
		// This is the account name part of the Csi Azure volume cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.CsiAzureCredsAccountNamePartSuffix)
		partName = defaults.CsiAzureCredsAccountNamePart

	} else if strings.HasSuffix(secretName, defaults.CsiAzureKeyVaultCredsClientIdSuffix) {
		// This is the client-id part of the Csi Azure KeyVault cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.CsiAzureKeyVaultCredsClientIdSuffix)
		partName = defaults.CsiAzureKeyVaultCredsClientIdPart

	} else if strings.HasSuffix(secretName, defaults.CsiAzureKeyVaultCredsClientSecretSuffix) {
		// This is the client secret part of the Csi Azure KeyVault cred secret
		secretObjName = strings.TrimSuffix(secretName, defaults.CsiAzureKeyVaultCredsClientSecretSuffix)
		partName = defaults.CsiAzureKeyVaultCredsClientSecretPart

	} else if strings.HasSuffix(secretName, suffix.ClientCertificate) {
		// This is the account name part of the client certificate secret
		secretObjName = secretName
		partName = "ca.crt"

	} else if strings.HasSuffix(secretName, suffix.OAuth2ClientSecret) {
		secretObjName = operatorutils.GetAuxiliaryComponentSecretName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
		partName = defaults.OAuthClientSecretKeyName
	} else if strings.HasSuffix(secretName, suffix.OAuth2CookieSecret) {
		secretObjName = operatorutils.GetAuxiliaryComponentSecretName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
		partName = defaults.OAuthCookieSecretKeyName
	} else if strings.HasSuffix(secretName, suffix.OAuth2RedisPassword) {
		secretObjName = operatorutils.GetAuxiliaryComponentSecretName(componentName, defaults.OAuthProxyAuxiliaryComponentSuffix)
		partName = defaults.OAuthRedisPasswordKeyName
	} else {
		// This is a regular secret
		secretObjName = operatorutils.GetComponentSecretName(componentName)
		partName = secretName

	}

	secretObject, err := eh.userAccount.Client.CoreV1().Secrets(ns).Get(ctx, secretObjName, metav1.GetOptions{})
	if err != nil && k8sErrors.IsNotFound(err) {
		return radixhttp.TypeMissingError("Secret object does not exist", err)
	}
	if err != nil {
		return radixhttp.UnexpectedError("Failed getting secret object", err)
	}

	if secretObject.Data == nil {
		secretObject.Data = make(map[string][]byte)
	}

	secretObject.Data[partName] = []byte(newSecretValue)

	_, err = eh.userAccount.Client.CoreV1().Secrets(ns).Update(ctx, secretObject, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (eh SecretHandler) getAzureKeyVaultSecretVersionsMap(appName, envNamespace, componentName, azureKeyVaultName string) (secretIdToPodNameToSecretVersionMap, error) {
	secretProviderClassMap, err := eh.getAzureKeyVaultSecretProviderClassMapForAppComponentStorage(appName, envNamespace, componentName, azureKeyVaultName)
	if err != nil {
		return nil, err
	}
	secretsInPodStatusList, err := eh.serviceAccount.SecretProviderClient.SecretsstoreV1().SecretProviderClassPodStatuses(envNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	secretStatusMap := make(secretIdToPodNameToSecretVersionMap)
	for _, secretsInPod := range secretsInPodStatusList.Items {
		if _, ok := secretProviderClassMap[secretsInPod.Status.SecretProviderClassName]; !ok {
			continue
		}
		for _, secretVersion := range secretsInPod.Status.Objects {
			if _, ok := secretStatusMap[secretVersion.ID]; !ok {
				secretStatusMap[secretVersion.ID] = make(podNameToSecretVersionMap)
			}
			secretStatusMap[secretVersion.ID][secretsInPod.Status.PodName] = secretVersion.Version
		}
	}
	return secretStatusMap, nil // map[secretType/secretName][podName]secretVersion
}

func (eh SecretHandler) getAzureKeyVaultSecretProviderClassMapForAppComponentStorage(appName, envNamespace, componentName, azureKeyVaultName string) (map[string]secretsstorev1.SecretProviderClass, error) {
	labelSelector := getAzureKeyVaultSecretRefSecretProviderClassLabels(appName, componentName, azureKeyVaultName).String()
	return eh.getSecretProviderClassMapForLabelSelector(envNamespace, labelSelector)
}

func getAzureKeyVaultSecretRefSecretProviderClassLabels(appName string, componentName string, azureKeyVaultName string) labels.Set {
	return labels.Set{
		kube.RadixAppLabel:           appName,
		kube.RadixComponentLabel:     componentName,
		kube.RadixSecretRefNameLabel: strings.ToLower(azureKeyVaultName),
		kube.RadixSecretRefTypeLabel: string(radixv1.RadixSecretRefTypeAzureKeyVault),
	}
}

func (eh SecretHandler) getSecretProviderClassMapForLabelSelector(envNamespace, labelSelector string) (map[string]secretsstorev1.SecretProviderClass, error) {
	secretProviderClassList, err := eh.serviceAccount.SecretProviderClient.SecretsstoreV1().SecretProviderClasses(envNamespace).
		List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}
	secretProviderClassMap := make(map[string]secretsstorev1.SecretProviderClass)
	for _, secretProviderClass := range secretProviderClassList.Items {
		secretProviderClass := secretProviderClass
		secretProviderClassMap[secretProviderClass.GetName()] = secretProviderClass
	}
	return secretProviderClassMap, nil
}

// GetAzureKeyVaultSecretVersions Gets list of Azure Key vault secret versions for the storage in the component
func (eh SecretHandler) GetAzureKeyVaultSecretVersions(appName, envName, componentName, azureKeyVaultName, secretId string) ([]models.AzureKeyVaultSecretVersion, error) {
	var envNamespace = operatorutils.GetEnvironmentNamespace(appName, envName)
	azureKeyVaultSecretMap, err := eh.getAzureKeyVaultSecretVersionsMap(appName, envNamespace, componentName, azureKeyVaultName)
	if err != nil {
		return nil, err
	}
	podList, err := eh.userAccount.Client.CoreV1().Pods(envNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelselector.ForComponent(appName, componentName).String()})
	if err != nil {
		return nil, err
	}
	sortUtils.Pods(podList.Items, sortUtils.ByPodCreationTimestamp, sortUtils.Descending)
	return eh.getAzKeyVaultSecretVersions(appName, envNamespace, componentName, podList.Items, azureKeyVaultSecretMap[secretId])
}

func (eh SecretHandler) getAzKeyVaultSecretVersions(appName string, envNamespace string, componentName string, pods []corev1.Pod, podSecretVersionMap podNameToSecretVersionMap) ([]models.AzureKeyVaultSecretVersion, error) {
	jobMap, err := eh.getJobMap(appName, envNamespace, componentName)
	if err != nil {
		return nil, err
	}
	var azKeyVaultSecretVersions []models.AzureKeyVaultSecretVersion
	for _, pod := range pods {
		secretVersion, ok := podSecretVersionMap[pod.GetName()]
		if !ok {
			continue
		}
		podCreated := pod.GetCreationTimestamp()
		azureKeyVaultSecretVersion := models.AzureKeyVaultSecretVersion{
			ReplicaName:    pod.GetName(),
			ReplicaCreated: radixutils.FormatTime(&podCreated),
			Version:        secretVersion,
		}
		if _, ok := pod.ObjectMeta.Labels[kube.RadixPodIsJobAuxObjectLabel]; ok {
			azureKeyVaultSecretVersion.ReplicaName = "New jobs"
			azKeyVaultSecretVersions = append(azKeyVaultSecretVersions, azureKeyVaultSecretVersion)
			continue
		}
		if !strings.EqualFold(pod.ObjectMeta.Labels[kube.RadixJobTypeLabel], kube.RadixJobTypeJobSchedule) {
			azKeyVaultSecretVersions = append(azKeyVaultSecretVersions, azureKeyVaultSecretVersion)
			continue
		}
		jobName := pod.ObjectMeta.Labels[k8sJobNameLabel]
		job, ok := jobMap[jobName]
		if !ok {
			continue
		}
		azureKeyVaultSecretVersion.JobName = jobName
		jobCreated := job.GetCreationTimestamp()
		azureKeyVaultSecretVersion.JobCreated = radixutils.FormatTime(&jobCreated)
		if batchName, ok := pod.ObjectMeta.Labels[kube.RadixBatchNameLabel]; ok {
			if batch, ok := jobMap[batchName]; ok {
				azureKeyVaultSecretVersion.BatchName = batchName
				batchCreated := batch.GetCreationTimestamp()
				azureKeyVaultSecretVersion.BatchCreated = radixutils.FormatTime(&batchCreated)
			}
		}
		azKeyVaultSecretVersions = append(azKeyVaultSecretVersions, azureKeyVaultSecretVersion)
	}
	return azKeyVaultSecretVersions, nil
}

func (eh SecretHandler) getJobMap(appName, namespace, componentName string) (map[string]batchv1.Job, error) {
	jobMap := make(map[string]batchv1.Job)
	jobList, err := eh.userAccount.Client.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelselector.JobAndBatchJobsForComponent(appName, componentName)})
	if err != nil {
		return nil, err
	}
	for _, job := range jobList.Items {
		job := job
		jobMap[job.GetName()] = job
	}
	return jobMap, nil
}
