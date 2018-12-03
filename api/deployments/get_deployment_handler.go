package deployments

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/statoil/radix-api/api/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	deploymentModels "github.com/statoil/radix-api/api/deployments/models"
	"github.com/statoil/radix-api/api/pods"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployHandler Instance variables
type DeployHandler struct {
	kubeClient  kubernetes.Interface
	radixClient radixclient.Interface
}

// Init Constructor
func Init(kubeClient kubernetes.Interface, radixClient radixclient.Interface) DeployHandler {
	return DeployHandler{
		kubeClient:  kubeClient,
		radixClient: radixClient,
	}
}

// GetLogs handler for GetLogs
func (deploy DeployHandler) GetLogs(appName, podName string) (string, error) {
	ns := crdUtils.GetAppNamespace(appName)
	// TODO! rewrite to use deploymentId to find pod (rd.Env -> namespace -> pod)
	ra, err := deploy.radixClient.RadixV1().RadixApplications(ns).Get(appName, metav1.GetOptions{})
	if err != nil {
		return "", deploymentModels.NonExistingApplication(err, appName)
	}
	for _, env := range ra.Spec.Environments {
		podHandler := pods.Init(deploy.kubeClient)
		log, err := podHandler.HandleGetEnvironmentPodLog(appName, env.Name, podName, "")
		if errors.IsNotFound(err) {
			continue
		} else if err != nil {
			return "", err
		}

		return log, nil
	}
	return "", deploymentModels.NonExistingPod(appName, podName)
}

// GetDeploymentsForApplication Lists deployments accross environments
func (deploy DeployHandler) GetDeploymentsForApplication(appName string, latest bool) ([]*deploymentModels.DeploymentSummary, error) {
	namespace := corev1.NamespaceAll
	return deploy.getDeployments(namespace, appName, "", latest)
}

// GetDeploymentsForApplicationEnvironment Lists deployments inside environment
func (deploy DeployHandler) GetDeploymentsForApplicationEnvironment(appName, environment string, latest bool) ([]*deploymentModels.DeploymentSummary, error) {
	var namespace = corev1.NamespaceAll
	if strings.TrimSpace(environment) != "" {
		namespace = crdUtils.GetEnvironmentNamespace(appName, environment)
	}

	return deploy.getDeployments(namespace, appName, "", latest)
}

// GetDeploymentsForJob Lists deployments for job name
func (deploy DeployHandler) GetDeploymentsForJob(appName, jobName string) ([]*deploymentModels.DeploymentSummary, error) {
	namespace := corev1.NamespaceAll
	return deploy.getDeployments(namespace, appName, jobName, false)
}

// GetDeploymentWithName Handler for GetDeploymentWithName
func (deploy DeployHandler) GetDeploymentWithName(appName, deploymentName string) (*deploymentModels.Deployment, error) {
	// Need to list all deployments to find active to of deployment
	allDeployments, err := deploy.GetDeploymentsForApplication(appName, false)
	if err != nil {
		return nil, err
	}

	// Find the deployment summary
	var theDeployment *deploymentModels.DeploymentSummary
	for _, deployment := range allDeployments {
		if strings.EqualFold(deployment.Name, deploymentName) {
			theDeployment = deployment
			break
		}
	}

	if theDeployment == nil {
		return nil, deploymentModels.NonExistingDeployment(nil, deploymentName)
	}

	namespace := crdUtils.GetEnvironmentNamespace(appName, theDeployment.Environment)
	rd, err := deploy.radixClient.RadixV1().RadixDeployments(namespace).Get(deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var activeTo time.Time
	if !strings.EqualFold(theDeployment.ActiveTo, "") {
		activeTo, err = utils.ParseTimestamp(theDeployment.ActiveTo)
		if err != nil {
			return nil, err
		}
	}

	components, err := deploy.GetComponentsForDeployment(appName, theDeployment)
	if err != nil {
		return nil, err
	}

	return deploymentModels.NewDeploymentBuilder().
		WithRadixDeployment(*rd).
		WithActiveTo(activeTo).
		WithComponents(components).
		BuildDeployment(), nil
}

func (deploy DeployHandler) getDeployments(namespace, appName, jobName string, latest bool) ([]*deploymentModels.DeploymentSummary, error) {
	var listOptions metav1.ListOptions
	labelSelector := fmt.Sprintf("radixApp=%s, radix-app=%s", appName, appName)

	if strings.TrimSpace(jobName) != "" {
		labelSelector = fmt.Sprintf(labelSelector+", radix-job-name=%s", jobName)
	}

	listOptions.LabelSelector = labelSelector
	radixDeploymentList, err := deploy.radixClient.RadixV1().RadixDeployments(namespace).List(listOptions)

	if err != nil {
		return nil, err
	}

	rds := sortRdsByCreationTimestampDesc(radixDeploymentList.Items)
	envsLastIndexMap := getRdEnvironments(rds)

	radixDeployments := make([]*deploymentModels.DeploymentSummary, 0)
	for i, rd := range rds {
		envName := rd.Spec.Environment

		builder := deploymentModels.NewDeploymentBuilder().WithRadixDeployment(rd)

		lastIndex := envsLastIndexMap[envName]
		if lastIndex >= 0 {
			builder.WithActiveTo(rds[lastIndex].CreationTimestamp.Time)
		}
		envsLastIndexMap[envName] = i

		radixDeployments = append(radixDeployments, builder.BuildDeploymentSummary())
	}

	return postFiltering(radixDeployments, latest), nil
}

func getRdEnvironments(rds []v1.RadixDeployment) map[string]int {
	envs := make(map[string]int)
	for _, rd := range rds {
		envName := rd.Spec.Environment
		if _, exists := envs[envName]; !exists {
			envs[envName] = -1
		}
	}
	return envs
}

func sortRdsByCreationTimestampDesc(rds []v1.RadixDeployment) []v1.RadixDeployment {
	sort.Slice(rds, func(i, j int) bool {
		return rds[j].CreationTimestamp.Before(&rds[i].CreationTimestamp)
	})
	return rds
}

func postFiltering(all []*deploymentModels.DeploymentSummary, latest bool) []*deploymentModels.DeploymentSummary {
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

func isLatest(theOne *deploymentModels.DeploymentSummary, all []*deploymentModels.DeploymentSummary) bool {
	theOneActiveFrom, err := utils.ParseTimestamp(theOne.ActiveFrom)
	if err != nil {
		return false
	}

	for _, rd := range all {
		rdActiveFrom, err := utils.ParseTimestamp(rd.ActiveFrom)
		if err != nil {
			continue
		}

		if rd.Environment == theOne.Environment &&
			rd.Name != theOne.Name &&
			rdActiveFrom.After(theOneActiveFrom) {
			return false
		}
	}

	return true
}
