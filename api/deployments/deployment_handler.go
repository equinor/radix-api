package deployments

import (
	"fmt"
	"sort"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-api/api/utils"
	"k8s.io/client-go/kubernetes"

	deploymentModels "github.com/statoil/radix-api/api/deployments/models"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"github.com/statoil/radix-operator/pkg/apis/radixvalidators"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// errors
func nonExistingApplication(underlyingError error, appName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Unable to get application for app %s", appName), underlyingError)
}

func nonExistingFromEnvironment(underlyingError error) error {
	return utils.TypeMissingError("Non existing from environment", underlyingError)
}

func nonExistingToEnvironment(underlyingError error) error {
	return utils.TypeMissingError("Non existing to environment", underlyingError)
}

func nonExistingDeployment(underlyingError error) error {
	return utils.TypeMissingError("Non existing deployment", underlyingError)
}

func nonExistingComponentName(appName, componentName string) error {
	return utils.TypeMissingError(fmt.Sprintf("Unable to get application component %s for app %s", componentName, appName), nil)
}

// HandleGetDeployments handler for GetDeployments
func HandleGetDeployments(radixclient radixclient.Interface, appName, environment string, latest bool) ([]*deploymentModels.ApplicationDeployment, error) {
	var listOptions metav1.ListOptions
	if strings.TrimSpace(appName) != "" {
		listOptions.LabelSelector = fmt.Sprintf("radixApp=%s", appName)
	}

	var namespace = corev1.NamespaceAll
	if strings.TrimSpace(appName) != "" && strings.TrimSpace(environment) != "" {
		namespace = crdUtils.GetEnvironmentNamespace(appName, environment)
	}

	radixDeploymentList, err := radixclient.RadixV1().RadixDeployments(namespace).List(listOptions)

	if err != nil {
		return nil, err
	}

	rds := sortRdsByCreationTimestampDesc(radixDeploymentList.Items)

	radixDeployments := make([]*deploymentModels.ApplicationDeployment, 0)
	for i, rd := range rds {
		builder := NewBuilder().
			withName(rd.GetName()).
			withAppName(rd.Spec.AppName).
			withEnvironment(rd.Spec.Environment).
			withActiveFrom(rd.CreationTimestamp.Time)

		if i > 0 {
			builder.withActiveTo(rds[i-1].CreationTimestamp.Time)
		}
		radixDeployments = append(radixDeployments, builder.buildApplicationDeployment())
	}

	return postFiltering(radixDeployments, latest), nil
}

// HandlePromoteToEnvironment handler for PromoteEnvironment
func HandlePromoteToEnvironment(client kubernetes.Interface, radixclient radixclient.Interface, appName, deploymentName string, promotionParameters deploymentModels.PromotionParameters) (*deploymentModels.ApplicationDeployment, error) {
	if strings.TrimSpace(appName) == "" {
		return nil, utils.ValidationError("Radix Promotion", "App name is required")
	}

	radixConfig, err := radixclient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	if err != nil {
		return nil, nonExistingApplication(err, appName)
	}

	fromNs := crdUtils.GetEnvironmentNamespace(appName, promotionParameters.FromEnvironment)
	toNs := crdUtils.GetEnvironmentNamespace(appName, promotionParameters.ToEnvironment)

	_, err = client.CoreV1().Namespaces().Get(fromNs, metav1.GetOptions{})
	if err != nil {
		return nil, nonExistingFromEnvironment(err)
	}

	_, err = client.CoreV1().Namespaces().Get(toNs, metav1.GetOptions{})
	if err != nil {
		return nil, nonExistingToEnvironment(err)
	}

	log.Infof("Promoting %s from %s to %s", appName, promotionParameters.FromEnvironment, promotionParameters.ToEnvironment)
	var radixDeployment *v1.RadixDeployment

	radixDeployment, err = radixclient.RadixV1().RadixDeployments(fromNs).Get(deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, nonExistingDeployment(err)
	}

	radixDeployment.ResourceVersion = ""
	radixDeployment.Namespace = toNs
	radixDeployment.Spec.Environment = promotionParameters.ToEnvironment

	err = mergeWithRadixApplication(radixConfig, radixDeployment, promotionParameters.ToEnvironment)
	if err != nil {
		return nil, err
	}

	isValid, err := radixvalidators.CanRadixDeploymentBeInserted(radixclient, radixDeployment)
	if !isValid {
		return nil, err
	}

	radixDeployment, err = radixclient.RadixV1().RadixDeployments(toNs).Create(radixDeployment)
	if err != nil {
		return nil, err
	}

	return &deploymentModels.ApplicationDeployment{Name: radixDeployment.Name}, nil
}

func sortRdsByCreationTimestampDesc(rds []v1.RadixDeployment) []v1.RadixDeployment {
	sort.Slice(rds, func(i, j int) bool {
		return rds[j].CreationTimestamp.Before(&rds[i].CreationTimestamp)
	})
	return rds
}

func postFiltering(all []*deploymentModels.ApplicationDeployment, latest bool) []*deploymentModels.ApplicationDeployment {
	if latest {
		filtered := all[:0]
		for _, rd := range all {
			if isLatest(rd, all) {
				filtered = append(filtered, rd)
			}
		}

		return filtered
	}

	return all
}

func isLatest(theOne *deploymentModels.ApplicationDeployment, all []*deploymentModels.ApplicationDeployment) bool {
	theOneActiveFrom, err := utils.ParseTimestamp(theOne.ActiveFrom)
	if err != nil {
		return false
	}

	for _, rd := range all {
		rdActiveFrom, err := utils.ParseTimestamp(rd.ActiveFrom)
		if err != nil {
			continue
		}

		if rd.AppName == theOne.AppName &&
			rd.Environment == theOne.Environment &&
			rd.Name != theOne.Name &&
			rdActiveFrom.After(theOneActiveFrom) {
			return false
		}
	}

	return true
}

func mergeWithRadixApplication(radixConfig *v1.RadixApplication, radixDeployment *v1.RadixDeployment, environment string) error {
	for index, comp := range radixDeployment.Spec.Components {
		raComp := getComponentConfig(radixConfig, comp.Name)
		if raComp == nil {
			return nonExistingComponentName(radixConfig.GetName(), comp.Name)
		}

		environmentVariables := getEnvironmentVariables(raComp, environment)
		radixDeployment.Spec.Components[index].EnvironmentVariables = environmentVariables
	}

	return nil
}

func getComponentConfig(radixConfig *v1.RadixApplication, componentName string) *v1.RadixComponent {
	for _, comp := range radixConfig.Spec.Components {
		if strings.EqualFold(comp.Name, componentName) {
			return &comp
		}
	}

	return nil
}

func getEnvironmentVariables(componentConfig *v1.RadixComponent, environment string) v1.EnvVarsMap {
	for _, environmentVariables := range componentConfig.EnvironmentVariables {
		if strings.EqualFold(environmentVariables.Environment, environment) {
			return environmentVariables.Variables
		}
	}

	return v1.EnvVarsMap{}
}

// Builder Builds DTOs
type Builder interface {
	withName(string) Builder
	withAppName(string) Builder
	withEnvironment(string) Builder
	withActiveFrom(time.Time) Builder
	withActiveTo(time.Time) Builder
	buildApplicationDeployment() *deploymentModels.ApplicationDeployment
}

type builder struct {
	name        string
	appName     string
	environment string
	activeFrom  time.Time
	activeTo    time.Time
}

func (b *builder) withName(name string) Builder {
	b.name = name
	return b
}

func (b *builder) withAppName(appName string) Builder {
	b.appName = appName
	return b
}

func (b *builder) withEnvironment(environment string) Builder {
	b.environment = environment
	return b
}

func (b *builder) withActiveFrom(activeFrom time.Time) Builder {
	b.activeFrom = activeFrom
	return b
}

func (b *builder) withActiveTo(activeTo time.Time) Builder {
	b.activeTo = activeTo
	return b
}

func (b *builder) buildApplicationDeployment() *deploymentModels.ApplicationDeployment {
	return &deploymentModels.ApplicationDeployment{
		Name:        b.name,
		AppName:     b.appName,
		Environment: b.environment,
		ActiveFrom:  utils.FormatTimestamp(b.activeFrom),
		ActiveTo:    utils.FormatTimestamp(b.activeTo),
	}
}

// NewBuilder Constructor for application builder
func NewBuilder() Builder {
	return &builder{}
}
