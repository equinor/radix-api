package alerting

import (
	"context"
	"testing"
	"time"

	alertModels "github.com/equinor/radix-api/api/alerting/models"
	"github.com/equinor/radix-api/models"
	radixmodels "github.com/equinor/radix-common/models"
	operatoralert "github.com/equinor/radix-operator/pkg/apis/alert"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

type HandlerTestSuite struct {
	suite.Suite
	accounts models.Accounts
}

func (s *HandlerTestSuite) SetupTest() {
	inKubeClient, outKubeClient := kubefake.NewSimpleClientset(), kubefake.NewSimpleClientset()
	inRadixClient, outRadixClient := radixfake.NewSimpleClientset(), radixfake.NewSimpleClientset()
	s.accounts = models.NewAccounts(inKubeClient, inRadixClient, outKubeClient, outRadixClient, "", radixmodels.Impersonation{})
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) Test_GetAlertingConfig_MissingRadixAlert() {
	ns1, ns2, appName := "ns1", "ns2", "the-app"
	ral := radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "any-alert", Labels: map[string]string{kube.RadixAppLabel: appName}}}
	s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(ns1).Create(context.Background(), &ral, metav1.CreateOptions{})

	s.Run("RadixAlert exists in namespace, but not labelled correctly", func() {
		sut := handler{accounts: s.accounts, namespace: ns1, appName: "other-app"}
		config, err := sut.GetAlertingConfig()
		s.Nil(err)
		s.False(config.Enabled)
		s.False(config.Ready)
	})
	s.Run("no RadixAlert exists in namespace", func() {
		sut := handler{accounts: s.accounts, namespace: ns2, appName: "any-app"}
		config, err := sut.GetAlertingConfig()
		s.Nil(err)
		s.False(config.Enabled)
		s.False(config.Ready)
	})
}

func (s *HandlerTestSuite) Test_GetAlertingConfig_MultipleRadixAlerts() {
	namespace, appName := "myapp-app", "myapp"
	ral1 := radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: "alert1", Labels: map[string]string{kube.RadixAppLabel: appName}},
	}
	ral2 := radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: "alert2", Labels: map[string]string{kube.RadixAppLabel: appName}},
	}
	s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), &ral1, metav1.CreateOptions{})
	s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), &ral2, metav1.CreateOptions{})

	sut := handler{accounts: s.accounts, namespace: namespace, appName: appName}
	config, err := sut.GetAlertingConfig()
	s.Error(err, MultipleAlertingConfigurationsError())
	s.Nil(config)
}

func (s *HandlerTestSuite) Test_GetAlertingConfig_RadixAlertExists_NotReconciled() {
	namespace, appName := "myapp-app", "myapp"
	ral := radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: "alert1", Labels: map[string]string{kube.RadixAppLabel: appName}},
	}
	s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), &ral, metav1.CreateOptions{})

	sut := handler{accounts: s.accounts, namespace: namespace, appName: appName}
	config, _ := sut.GetAlertingConfig()
	s.True(config.Enabled)
	s.False(config.Ready)
}

func (s *HandlerTestSuite) Test_GetAlertingConfig_RadixAlertExists_Reconciled() {
	namespace, appName, alertName := "myapp-app", "myapp", "alert"
	ral := radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Labels: map[string]string{kube.RadixAppLabel: appName}},
		Status:     radixv1.RadixAlertStatus{Reconciled: &metav1.Time{Time: time.Now()}},
	}
	s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), &ral, metav1.CreateOptions{})

	sut := handler{accounts: s.accounts, namespace: namespace, appName: appName}
	config, _ := sut.GetAlertingConfig()
	s.True(config.Enabled)
	s.True(config.Ready)
}

func (s *HandlerTestSuite) Test_GetAlertingConfig() {
	namespace, appName, alertName, alertNames := "myapp-app", "myapp", "alert", []string{"alert1", "alert2", "alert3"}
	ral := radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Labels: map[string]string{kube.RadixAppLabel: appName}},
		Spec: radixv1.RadixAlertSpec{
			Receivers: radixv1.ReceiverMap{
				"receiver1": radixv1.Receiver{
					SlackConfig: radixv1.SlackConfig{Enabled: true},
				},
				"receiver2": radixv1.Receiver{
					SlackConfig: radixv1.SlackConfig{Enabled: false},
				},
			},
			Alerts: []radixv1.Alert{
				{Receiver: "receiver1", Alert: "alert1"},
				{Receiver: "receiver2", Alert: "alert2"},
			},
		},
		Status: radixv1.RadixAlertStatus{Reconciled: &metav1.Time{Time: time.Now()}},
	}
	s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), &ral, metav1.CreateOptions{})
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: operatoralert.GetAlertSecretName(alertName)},
		Data: map[string][]byte{
			operatoralert.GetSlackConfigSecretKeyName("receiver2"): []byte("data"),
		},
	}
	s.accounts.UserAccount.Client.CoreV1().Secrets(namespace).Create(context.Background(), &secret, metav1.CreateOptions{})

	sut := handler{accounts: s.accounts, namespace: namespace, appName: appName, validAlertNames: alertNames}
	config, _ := sut.GetAlertingConfig()
	s.NotNil(config)
	s.ElementsMatch(alertNames, config.AlertNames)
	s.ElementsMatch(alertModels.AlertConfigList{{Alert: "alert1", Receiver: "receiver1"}, {Alert: "alert2", Receiver: "receiver2"}}, config.Alerts)
	s.Len(config.Receivers, 2)
	s.Equal(config.Receivers["receiver1"], alertModels.ReceiverConfig{SlackConfig: &alertModels.SlackConfig{Enabled: true}})
	s.Equal(config.Receivers["receiver2"], alertModels.ReceiverConfig{SlackConfig: &alertModels.SlackConfig{Enabled: false}})
	s.Len(config.ReceiverSecretStatus, 2)
	s.Equal(config.ReceiverSecretStatus["receiver1"], alertModels.ReceiverConfigSecretStatus{SlackConfig: &alertModels.SlackConfigSecretStatus{WebhookURLConfigured: false}})
	s.Equal(config.ReceiverSecretStatus["receiver2"], alertModels.ReceiverConfigSecretStatus{SlackConfig: &alertModels.SlackConfigSecretStatus{WebhookURLConfigured: true}})
}
