package alerting

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	alertModels "github.com/equinor/radix-api/api/alerting/models"
	"github.com/equinor/radix-api/models"
	operatoralert "github.com/equinor/radix-operator/pkg/apis/alert"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	corev1 "k8s.io/api/core/v1"
	kubeErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	defaultReceiverName          = "slack"
	defaultAlertConfigName       = "alerting"
	defaultReconcilePollInterval = 500 * time.Millisecond
	defaultReconcilePollTimeout  = 5 * time.Second
)

var alertConfigNotConfigured = alertModels.AlertingConfig{}

type Handler interface {
	GetAlertingConfig() (*alertModels.AlertingConfig, error)
	EnableAlerting() (*alertModels.AlertingConfig, error)
	DisableAlerting() (*alertModels.AlertingConfig, error)
	UpdateAlertingConfig(config alertModels.UpdateAlertingConfig) (*alertModels.AlertingConfig, error)
}

type handler struct {
	accounts              models.Accounts
	validAlertNames       []string
	namespace             string
	appName               string
	reconcilePollInterval time.Duration
	reconcilePollTimeout  time.Duration
}

func NewEnvironmentHandler(accounts models.Accounts, appName, envName string) Handler {
	return &handler{
		accounts:              accounts,
		appName:               appName,
		namespace:             crdutils.GetEnvironmentNamespace(appName, envName),
		validAlertNames:       getAlertNamesForScope(operatoralert.EnvironmentScope),
		reconcilePollInterval: defaultReconcilePollInterval,
		reconcilePollTimeout:  defaultReconcilePollTimeout,
	}
}

func NewApplicationHandler(accounts models.Accounts, appName string) Handler {
	return &handler{
		accounts:              accounts,
		appName:               appName,
		namespace:             crdutils.GetAppNamespace(appName),
		validAlertNames:       getAlertNamesForScope(operatoralert.ApplicationScope),
		reconcilePollInterval: defaultReconcilePollInterval,
		reconcilePollTimeout:  defaultReconcilePollTimeout,
	}
}

func getAlertNamesForScope(scope operatoralert.AlertScope) []string {
	var alertNames []string
	for alertName, alertConfig := range operatoralert.GetDefaultAlertConfigs() {
		if alertConfig.Scope == scope {
			alertNames = append(alertNames, alertName)
		}
	}
	return alertNames
}

func (h *handler) GetAlertingConfig() (*alertModels.AlertingConfig, error) {
	ral, err := h.getExistingRadixAlerts()
	if err != nil {
		return nil, err
	}

	if len(ral.Items) > 1 {
		return nil, MultipleAlertingConfigurationsError()
	}

	if len(ral.Items) == 0 {
		return &alertConfigNotConfigured, nil
	}

	return h.getAlertingConfigFromRadixAlert(&ral.Items[0])
}

func (h *handler) UpdateAlertingConfig(config alertModels.UpdateAlertingConfig) (*alertModels.AlertingConfig, error) {
	alerts, err := h.getExistingRadixAlerts()
	if err != nil {
		return nil, err
	}

	if len(alerts.Items) > 1 {
		return nil, MultipleAlertingConfigurationsError()
	}

	if len(alerts.Items) == 0 {
		return nil, AlertingNotEnabledError()
	}

	updatedAlert, err := h.updateRadixAlertFromAlertingConfig(alerts.Items[0], config)
	if err != nil {
		return nil, err
	}

	return h.getAlertingConfigFromRadixAlert(updatedAlert)
}

func (h *handler) updateRadixAlertFromAlertingConfig(radixAlert radixv1.RadixAlert, config alertModels.UpdateAlertingConfig) (*radixv1.RadixAlert, error) {
	if err := h.validateUpdateAlertingConfig(&config); err != nil {
		return nil, err
	}

	if len(config.ReceiverSecrets) > 0 {
		configSecret, err := h.getConfigSecret(radixAlert.Name)
		if err != nil {
			return nil, err
		}
		if err := h.updateConfigSecret(*configSecret, &config); err != nil {
			return nil, err
		}
	}

	radixAlert.Spec.Alerts = config.Alerts.AsRadixAlertAlerts()
	radixAlert.Spec.Receivers = config.Receivers.AsRadixAlertReceiverMap()
	return h.applyRadixAlert(&radixAlert)
}

func (h *handler) updateConfigSecret(secret corev1.Secret, config *alertModels.UpdateAlertingConfig) error {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}

	for receiverName, receiverSecret := range config.ReceiverSecrets {
		if receiverSecret.SlackConfig != nil {
			h.setSlackConfigSecret(*receiverSecret.SlackConfig, receiverName, &secret)
		}
	}

	kubeUtil, _ := kube.New(h.accounts.UserAccount.Client, h.accounts.UserAccount.RadixClient)
	_, err := kubeUtil.ApplySecret(h.namespace, &secret)
	return err
}

func (h *handler) setSlackConfigSecret(slackConfig alertModels.UpdateSlackConfigSecrets, receiverName string, secret *corev1.Secret) {
	secretKey := operatoralert.GetSlackConfigSecretKeyName(receiverName)
	if slackConfig.WebhookURL != nil {
		if len(strings.TrimSpace(*slackConfig.WebhookURL)) == 0 {
			delete(secret.Data, secretKey)
		} else {
			secret.Data[secretKey] = []byte(strings.TrimSpace(*slackConfig.WebhookURL))
		}
	}
}

func (h *handler) validateUpdateAlertingConfig(config *alertModels.UpdateAlertingConfig) error {
	for updateReceiverName, updateReceiver := range config.ReceiverSecrets {
		if _, found := config.Receivers[updateReceiverName]; !found {
			return UpdateReceiverSecretNotDefinedError(updateReceiverName)
		}
		if err := h.validateUpdateSlackConfig(updateReceiver.SlackConfig); err != nil {
			return err
		}
	}

	for _, alert := range config.Alerts {
		// Verify receiver exists
		if _, found := config.Receivers[alert.Receiver]; !found {
			return InvalidAlertReceiverError(alert.Alert, alert.Receiver)
		}
		// Verify alert name is valid
		if !slice.ContainsString(h.validAlertNames, alert.Alert) {
			return InvalidAlertError(alert.Alert)
		}
	}

	return nil
}

func (h *handler) validateUpdateSlackConfig(slackConfig *alertModels.UpdateSlackConfigSecrets) error {
	if slackConfig == nil || slackConfig.WebhookURL == nil {
		return nil
	}

	url, err := url.Parse(*slackConfig.WebhookURL)
	if err != nil {
		return InvalidSlackURLError(err)
	}
	if url.Scheme != "https" {
		return InvalidSlackURLError(errors.New("invalid scheme, must be https"))
	}
	return nil
}

func (h *handler) EnableAlerting() (*alertModels.AlertingConfig, error) {
	radixAlert, err := h.createDefaultRadixAlert()
	if err != nil {
		return nil, err
	}
	if reconciledAlert, reconciled := h.waitForRadixAlertReconciled(radixAlert); reconciled {
		radixAlert = reconciledAlert
	}
	return h.getAlertingConfigFromRadixAlert(radixAlert)
}

func (h *handler) DisableAlerting() (*alertModels.AlertingConfig, error) {
	alerts, err := h.getExistingRadixAlerts()
	if err != nil {
		return nil, err
	}

	for _, alert := range alerts.Items {
		if err := h.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(alert.Namespace).Delete(context.TODO(), alert.Name, metav1.DeleteOptions{}); err != nil {
			return nil, err
		}
	}

	return &alertConfigNotConfigured, nil
}

func (h *handler) waitForRadixAlertReconciled(source *radixv1.RadixAlert) (*radixv1.RadixAlert, bool) {
	var reconciledAlert *radixv1.RadixAlert

	hasReconciled := func() (bool, error) {
		radixAlert, err := h.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(source.Namespace).Get(context.TODO(), source.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if radixAlert.Status.Reconciled != nil {
			reconciledAlert = radixAlert
		}
		return radixAlert.Status.Reconciled != nil, nil
	}

	if err := wait.PollImmediate(h.reconcilePollInterval, h.reconcilePollTimeout, hasReconciled); err != nil {
		return nil, false
	}
	return reconciledAlert, true
}

func (h *handler) getExistingRadixAlerts() (*radixv1.RadixAlertList, error) {
	appLabels := labels.Set{kube.RadixAppLabel: h.appName}.String()
	return h.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(h.namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: appLabels})
}

func (h *handler) createDefaultRadixAlert() (*radixv1.RadixAlert, error) {
	ral, err := h.getExistingRadixAlerts()
	if err != nil {
		return nil, err
	}

	if len(ral.Items) > 0 {
		return nil, AlertingAlreadyEnabledError()
	}

	radixAlert := h.buildDefaultRadixAlertSpec()
	return h.applyRadixAlert(radixAlert)
}

func (h *handler) buildDefaultRadixAlertSpec() *radixv1.RadixAlert {
	radixAlert := radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: defaultAlertConfigName, Labels: map[string]string{kube.RadixAppLabel: h.appName}},
		Spec: radixv1.RadixAlertSpec{
			Receivers: h.buildDefaultRadixAlertReceivers(),
			Alerts:    h.buildDefaultRadixAlertAlerts(),
		},
	}

	return &radixAlert
}

func (h *handler) buildDefaultRadixAlertReceivers() radixv1.ReceiverMap {
	return radixv1.ReceiverMap{
		defaultReceiverName: radixv1.Receiver{
			SlackConfig: radixv1.SlackConfig{
				Enabled: true,
			},
		},
	}
}

func (h *handler) buildDefaultRadixAlertAlerts() []radixv1.Alert {
	var alerts []radixv1.Alert

	for _, alertName := range h.validAlertNames {
		alerts = append(alerts, radixv1.Alert{Alert: alertName, Receiver: defaultReceiverName})
	}

	return alerts
}

func (h *handler) applyRadixAlert(radixAlert *radixv1.RadixAlert) (*radixv1.RadixAlert, error) {
	existingAlert, err := h.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(h.namespace).Get(context.TODO(), radixAlert.Name, metav1.GetOptions{})
	if err != nil {
		if !kubeErrors.IsNotFound(err) {
			return nil, err
		}
		return h.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(h.namespace).Create(context.TODO(), radixAlert, metav1.CreateOptions{})
	}

	existingAlert.Labels = radixAlert.Labels
	existingAlert.Spec = radixAlert.Spec
	return h.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(h.namespace).Update(context.TODO(), existingAlert, metav1.UpdateOptions{})
}

func (h *handler) getAlertingConfigFromRadixAlert(ral *radixv1.RadixAlert) (*alertModels.AlertingConfig, error) {
	configSecret, err := h.getConfigSecret(ral.Name)
	if err != nil && !kubeErrors.IsNotFound(err) {
		return nil, err
	}

	alertsConfig := alertModels.AlertingConfig{
		Receivers:            h.getReceiverConfigFromRadixAlert(ral),
		ReceiverSecretStatus: h.getReceiverConfigSecretStatusFromRadixAlert(ral, configSecret),
		Alerts:               h.getAlertConfigFromRadixAlert(ral),
		AlertNames:           h.validAlertNames,
		Enabled:              true,
		Ready:                ral.Status.Reconciled != nil,
	}

	return &alertsConfig, nil
}

func (h *handler) getReceiverConfigFromRadixAlert(radixAlert *radixv1.RadixAlert) map[string]alertModels.ReceiverConfig {
	receiversMap := make(map[string]alertModels.ReceiverConfig)

	for receiverName, receiver := range radixAlert.Spec.Receivers {
		receiversMap[receiverName] = alertModels.ReceiverConfig{
			SlackConfig: &alertModels.SlackConfig{
				Enabled: receiver.SlackConfig.Enabled,
			},
		}
	}

	return receiversMap
}

func (h *handler) getReceiverConfigSecretStatusFromRadixAlert(radixAlert *radixv1.RadixAlert, configSecret *corev1.Secret) map[string]alertModels.ReceiverConfigSecretStatus {
	receiversMap := make(map[string]alertModels.ReceiverConfigSecretStatus)

	for receiverName := range radixAlert.Spec.Receivers {
		receiverStatus := alertModels.ReceiverConfigSecretStatus{
			SlackConfig: &alertModels.SlackConfigSecretStatus{
				WebhookURLConfigured: h.isReceiverSlackURLConfigured(receiverName, configSecret),
			},
		}
		receiversMap[receiverName] = receiverStatus
	}

	return receiversMap
}

func (h *handler) getAlertConfigFromRadixAlert(radixAlert *radixv1.RadixAlert) []alertModels.AlertConfig {
	var alertConfigs []alertModels.AlertConfig

	for _, alert := range radixAlert.Spec.Alerts {
		if _, found := radixAlert.Spec.Receivers[alert.Receiver]; found {
			alertConfigs = append(alertConfigs, alertModels.AlertConfig{
				Receiver: alert.Receiver,
				Alert:    alert.Alert,
			})
		}
	}

	return alertConfigs
}

func (h *handler) isReceiverSlackURLConfigured(receiverName string, configSecret *corev1.Secret) bool {
	url, found := h.getReceiverSlackURLFromSecret(receiverName, configSecret)
	return found && len(url) > 0
}

func (h *handler) getReceiverSlackURLFromSecret(receiverName string, configSecret *corev1.Secret) (string, bool) {
	if configSecret == nil {
		return "", false
	}

	url, found := configSecret.Data[operatoralert.GetSlackConfigSecretKeyName(receiverName)]
	return string(url), found
}

func (h *handler) getConfigSecret(alertName string) (*corev1.Secret, error) {
	return h.accounts.UserAccount.Client.CoreV1().Secrets(h.namespace).
		Get(context.TODO(), operatoralert.GetAlertSecretName(alertName), metav1.GetOptions{})
}
