package deployments

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/equinor/radix-api/api/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/pods"
	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployHandler Instance variables
type DeployHandler struct {
	kubeClient  kubernetes.Interface
	radixClient radixclient.Interface
}

// Init Constructor
func Init(accounts models.Accounts) DeployHandler {
	return DeployHandler{
		kubeClient:  accounts.UserAccount.Client,
		radixClient: accounts.UserAccount.RadixClient,
	}
}

// GetLogs handler for GetLogs
func (deploy DeployHandler) GetLogs(appName, podName string, sinceTime *time.Time) (string, error) {
	ns := crdUtils.GetAppNamespace(appName)
	// TODO! rewrite to use deploymentId to find pod (rd.Env -> namespace -> pod)
	ra, err := deploy.radixClient.RadixV1().RadixApplications(ns).Get(appName, metav1.GetOptions{})
	if err != nil {
		return "", deploymentModels.NonExistingApplication(err, appName)
	}
	for _, env := range ra.Spec.Environments {
		podHandler := pods.Init(deploy.kubeClient)
		log, err := podHandler.HandleGetEnvironmentPodLog(appName, env.Name, podName, "", sinceTime)
		if errors.IsNotFound(err) {
			continue
		} else if err != nil {
			return "", err
		}

		return log, nil
	}
	return "", deploymentModels.NonExistingPod(appName, podName)
}

// GetLogs handler for GetScheduledJobLogs
func (deploy DeployHandler) GetScheduledJobLogs(appName, jobName string, sinceTime *time.Time) (string, error) {
	ns := crdUtils.GetAppNamespace(appName)
	// TODO! rewrite to use deploymentId to find pod (rd.Env -> namespace -> pod)
	ra, err := deploy.radixClient.RadixV1().RadixApplications(ns).Get(appName, metav1.GetOptions{})
	if err != nil {
		return "", deploymentModels.NonExistingApplication(err, appName)
	}
	for _, env := range ra.Spec.Environments {
		handler := pods.Init(deploy.kubeClient)
		log, err := handler.HandleGetEnvironmentScheduledJobLog(appName, env.Name, jobName, "", sinceTime)
		if errors.IsNotFound(err) {
			continue
		} else if err != nil {
			return "", err
		}

		return log, nil
	}
	return "", deploymentModels.NonExistingPod(appName, jobName)
}

// GetDeploymentsForApplication Lists deployments accross environments
func (deploy DeployHandler) GetDeploymentsForApplication(appName string, latest bool) ([]*deploymentModels.DeploymentSummary, error) {
	namespace := corev1.NamespaceAll
	return deploy.getDeployments(namespace, appName, "", latest)
}

// GetLatestDeploymentForApplicationEnvironment Gets latest, active, deployment in environment
func (deploy DeployHandler) GetLatestDeploymentForApplicationEnvironment(appName, environment string) (*deploymentModels.DeploymentSummary, error) {
	if strings.TrimSpace(environment) == "" {
		return nil, deploymentModels.IllegalEmptyEnvironment()
	}

	namespace := crdUtils.GetEnvironmentNamespace(appName, environment)
	deploymentSummaries, err := deploy.getDeployments(namespace, appName, "", true)
	if err == nil && len(deploymentSummaries) == 1 {
		return deploymentSummaries[0], nil
	}

	return nil, deploymentModels.NoActiveDeploymentFoundInEnvironment(appName, environment)
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
	labelSelector := fmt.Sprintf("radix-app=%s", appName)

	if strings.TrimSpace(jobName) != "" {
		labelSelector = fmt.Sprintf(labelSelector+", %s=%s", kube.RadixJobNameLabel, jobName)
	}

	listOptions.LabelSelector = labelSelector
	radixDeploymentList, err := deploy.radixClient.RadixV1().RadixDeployments(namespace).List(listOptions)

	if err != nil {
		return nil, err
	}

	rds := sortRdsByActiveFromDesc(radixDeploymentList.Items)
	radixDeployments := make([]*deploymentModels.DeploymentSummary, 0)
	for _, rd := range rds {
		if latest && rd.Status.Condition == v1.DeploymentInactive {
			continue
		}

		builder := deploymentModels.NewDeploymentBuilder().WithRadixDeployment(rd)
		radixDeployments = append(radixDeployments, builder.BuildDeploymentSummary())
	}

	return radixDeployments, nil
}

func sortRdsByActiveFromDesc(rds []v1.RadixDeployment) []v1.RadixDeployment {
	sort.Slice(rds, func(i, j int) bool {
		if rds[j].Status.ActiveFrom.IsZero() {
			return true
		}

		if rds[i].Status.ActiveFrom.IsZero() {
			return false
		}
		return rds[j].Status.ActiveFrom.Before(&rds[i].Status.ActiveFrom)
	})
	return rds
}
