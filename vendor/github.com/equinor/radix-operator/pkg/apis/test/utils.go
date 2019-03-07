package test

import (
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Utils Instance variables
type Utils struct {
	client      kubernetes.Interface
	radixclient radixclient.Interface
}

// NewTestUtils Constructor
func NewTestUtils(client kubernetes.Interface, radixclient radixclient.Interface) Utils {
	return Utils{
		client:      client,
		radixclient: radixclient,
	}
}

// ApplyRegistration Will help persist an application registration
func (tu *Utils) ApplyRegistration(registrationBuilder builders.RegistrationBuilder) error {
	rr := registrationBuilder.BuildRR()

	_, err := tu.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(rr)
	if err != nil {
		return err
	}

	return nil
}

// ApplyApplication Will help persist an application
func (tu *Utils) ApplyApplication(applicationBuilder builders.ApplicationBuilder) error {
	if applicationBuilder.GetRegistrationBuilder() != nil {
		tu.ApplyRegistration(applicationBuilder.GetRegistrationBuilder())
	}

	ra := applicationBuilder.BuildRA()
	appNamespace := CreateAppNamespace(tu.client, ra.GetName())
	_, err := tu.radixclient.RadixV1().RadixApplications(appNamespace).Create(ra)
	if err != nil {
		return err
	}

	return nil
}

// ApplyApplicationUpdate Will help update an application
func (tu *Utils) ApplyApplicationUpdate(applicationBuilder builders.ApplicationBuilder) error {
	ra := applicationBuilder.BuildRA()
	appNamespace := builders.GetAppNamespace(ra.GetName())

	_, err := tu.radixclient.RadixV1().RadixApplications(appNamespace).Update(ra)
	if err != nil {
		return err
	}

	return nil
}

// ApplyDeployment Will help persist a deployment
func (tu *Utils) ApplyDeployment(deploymentBuilder builders.DeploymentBuilder) (*v1.RadixDeployment, error) {
	if deploymentBuilder.GetApplicationBuilder() != nil {
		tu.ApplyApplication(deploymentBuilder.GetApplicationBuilder())
	}

	rd := deploymentBuilder.BuildRD()
	log.Debugf("%s", rd.GetObjectMeta().GetCreationTimestamp())

	envNamespace := CreateEnvNamespace(tu.client, rd.Spec.AppName, rd.Spec.Environment)
	newRd, err := tu.radixclient.RadixV1().RadixDeployments(envNamespace).Create(rd)
	if err != nil {
		return nil, err
	}

	return newRd, nil
}

// ApplyDeploymentUpdate Will help update a deployment
func (tu *Utils) ApplyDeploymentUpdate(deploymentBuilder builders.DeploymentBuilder) error {
	rd := deploymentBuilder.BuildRD()
	envNamespace := builders.GetEnvironmentNamespace(rd.Spec.AppName, rd.Spec.Environment)

	_, err := tu.radixclient.RadixV1().RadixDeployments(envNamespace).Update(rd)
	if err != nil {
		return err
	}

	return nil
}

// ApplyRegistrationWithSync Will help persist an application registration
func (tu *Utils) ApplyRegistrationWithSync(registrationBuilder utils.RegistrationBuilder) error {
	err := tu.ApplyRegistration(registrationBuilder)

	rr := registrationBuilder.BuildRR()
	application, _ := application.NewApplication(tu.client, tu.radixclient, rr)
	err = application.OnSync()
	if err != nil {
		return err
	}

	return nil
}

// ApplyApplicationWithSync Will help persist an application
func (tu *Utils) ApplyApplicationWithSync(applicationBuilder utils.ApplicationBuilder) error {
	if applicationBuilder.GetRegistrationBuilder() != nil {
		tu.ApplyRegistrationWithSync(applicationBuilder.GetRegistrationBuilder())
	}

	err := tu.ApplyApplication(applicationBuilder)
	if err != nil {
		return err
	}

	ra := applicationBuilder.BuildRA()
	radixRegistration, err := tu.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(ra.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	applicationconfig, err := applicationconfig.NewApplicationConfig(tu.client, tu.radixclient, radixRegistration, ra)

	err = applicationconfig.OnSync()
	if err != nil {
		return err
	}

	return nil
}

// ApplyDeploymentWithSync Will help persist a deployment
func (tu *Utils) ApplyDeploymentWithSync(deploymentBuilder utils.DeploymentBuilder) (*v1.RadixDeployment, error) {

	if deploymentBuilder.GetApplicationBuilder() != nil {
		tu.ApplyApplicationWithSync(deploymentBuilder.GetApplicationBuilder())
	}

	rd, err := tu.ApplyDeployment(deploymentBuilder)
	if err != nil {
		return nil, err
	}

	radixRegistration, err := tu.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	deployment, err := deployment.NewDeployment(tu.client, tu.radixclient, nil, radixRegistration, rd)

	err = deployment.OnSync()
	if err != nil {
		return nil, err
	}

	return rd, nil
}

// ApplyDeploymentUpdateWithSync Will help update a deployment
func (tu *Utils) ApplyDeploymentUpdateWithSync(deploymentBuilder utils.DeploymentBuilder) error {
	rd := deploymentBuilder.BuildRD()

	err := tu.ApplyDeploymentUpdate(deploymentBuilder)
	if err != nil {
		return err
	}

	radixRegistration, err := tu.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment, err := deployment.NewDeployment(tu.client, tu.radixclient, nil, radixRegistration, rd)

	err = deployment.OnSync()
	if err != nil {
		return err
	}

	return nil
}

// CreateClusterPrerequisites Will do the needed setup which is part of radix boot
func (tu *Utils) CreateClusterPrerequisites(clustername, containerRegistry string) {
	tu.client.CoreV1().Secrets(corev1.NamespaceDefault).Create(&corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "radix-docker",
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string][]byte{
			"config.json": []byte("abcd"),
		},
	})

	tu.client.CoreV1().Secrets(corev1.NamespaceDefault).Create(&corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "radix-known-hosts-git",
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string][]byte{
			"known_hosts": []byte("abcd"),
		},
	})

	tu.client.CoreV1().ConfigMaps(corev1.NamespaceDefault).Create(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "radix-config",
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string]string{
			"clustername":       clustername,
			"containerRegistry": containerRegistry,
		},
	})
}

// CreateAppNamespace Helper method to creat app namespace
func CreateAppNamespace(kubeclient kubernetes.Interface, appName string) string {
	ns := builders.GetAppNamespace(appName)
	createNamespace(kubeclient, appName, "app", ns)
	return ns
}

// CreateEnvNamespace Helper method to creat env namespace
func CreateEnvNamespace(kubeclient kubernetes.Interface, appName, environment string) string {
	ns := builders.GetEnvironmentNamespace(appName, environment)
	createNamespace(kubeclient, appName, environment, ns)
	return ns
}

func createNamespace(kubeclient kubernetes.Interface, appName, envName, ns string) {
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
			Labels: map[string]string{
				"radixApp":         appName, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel: appName,
				kube.RadixEnvLabel: envName,
			},
		},
	}

	kubeclient.CoreV1().Namespaces().Create(&namespace)
}
