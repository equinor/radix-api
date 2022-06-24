package deployments

import (
	"context"
	"io"
	"sort"
	"strings"
	"time"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/pods"
	"github.com/equinor/radix-api/models"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

type DeployHandler interface {
	GetLogs(appName, podName string, sinceTime *time.Time) (io.ReadCloser, error)
	GetDeploymentWithName(appName, deploymentName string) (*deploymentModels.Deployment, error)
	GetDeploymentsForApplicationEnvironment(appName, environment string, latest bool) ([]*deploymentModels.DeploymentSummary, error)
	GetComponentsForDeploymentName(appName, deploymentID string) ([]*deploymentModels.Component, error)
	GetComponentsForDeployment(appName string, deployment *deploymentModels.DeploymentSummary) ([]*deploymentModels.Component, error)
	GetLatestDeploymentForApplicationEnvironment(appName, environment string) (*deploymentModels.DeploymentSummary, error)
	GetDeploymentsForJob(appName, jobName string) ([]*deploymentModels.DeploymentSummary, error)
}

// DeployHandler Instance variables
type deployHandler struct {
	kubeClient  kubernetes.Interface
	radixClient radixclient.Interface
}

// Init Constructor
func Init(accounts models.Accounts) DeployHandler {
	return &deployHandler{
		kubeClient:  accounts.UserAccount.Client,
		radixClient: accounts.UserAccount.RadixClient,
	}
}

// GetLogs handler for GetLogs
func (deploy *deployHandler) GetLogs(appName, podName string, sinceTime *time.Time) (io.ReadCloser, error) {
	ns := operatorUtils.GetAppNamespace(appName)
	// TODO! rewrite to use deploymentId to find pod (rd.Env -> namespace -> pod)
	ra, err := deploy.radixClient.RadixV1().RadixApplications(ns).Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return nil, deploymentModels.NonExistingApplication(err, appName)
	}
	for _, env := range ra.Spec.Environments {
		podHandler := pods.Init(deploy.kubeClient)
		log, err := podHandler.HandleGetEnvironmentPodLog(appName, env.Name, podName, "", sinceTime, nil)
		if errors.IsNotFound(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		return log, nil
	}
	return nil, deploymentModels.NonExistingPod(appName, podName)
}

// GetDeploymentsForApplication Lists deployments across environments
func (deploy *deployHandler) GetDeploymentsForApplication(appName string, latest bool) ([]*deploymentModels.DeploymentSummary, error) {
	namespace := corev1.NamespaceAll
	return deploy.getDeployments(namespace, appName, "", latest)
}

// GetLatestDeploymentForApplicationEnvironment Gets latest, active, deployment in environment
func (deploy *deployHandler) GetLatestDeploymentForApplicationEnvironment(appName, environment string) (*deploymentModels.DeploymentSummary, error) {
	if strings.TrimSpace(environment) == "" {
		return nil, deploymentModels.IllegalEmptyEnvironment()
	}

	namespace := operatorUtils.GetEnvironmentNamespace(appName, environment)
	deploymentSummaries, err := deploy.getDeployments(namespace, appName, "", true)
	if err == nil && len(deploymentSummaries) == 1 {
		return deploymentSummaries[0], nil
	}

	return nil, deploymentModels.NoActiveDeploymentFoundInEnvironment(appName, environment)
}

// GetDeploymentsForApplicationEnvironment Lists deployments inside environment
func (deploy *deployHandler) GetDeploymentsForApplicationEnvironment(appName, environment string, latest bool) ([]*deploymentModels.DeploymentSummary, error) {
	var namespace = corev1.NamespaceAll
	if strings.TrimSpace(environment) != "" {
		namespace = operatorUtils.GetEnvironmentNamespace(appName, environment)
	}

	deployments, err := deploy.getDeployments(namespace, appName, "", latest)
	return deployments, err
}

// GetDeploymentsForJob Lists deployments for job name
func (deploy *deployHandler) GetDeploymentsForJob(appName, jobName string) ([]*deploymentModels.DeploymentSummary, error) {
	namespace := corev1.NamespaceAll
	return deploy.getDeployments(namespace, appName, jobName, false)
}

// GetDeploymentWithName Handler for GetDeploymentWithName
func (deploy *deployHandler) GetDeploymentWithName(appName, deploymentName string) (*deploymentModels.Deployment, error) {
	// Need to list all deployments to find active to of deployment
	allDeployments, err := deploy.GetDeploymentsForApplication(appName, false)
	if err != nil {
		return nil, err
	}

	// Find the deployment summary
	var deploymentSummary *deploymentModels.DeploymentSummary
	for _, deployment := range allDeployments {
		if strings.EqualFold(deployment.Name, deploymentName) {
			deploymentSummary = deployment
			break
		}
	}

	if deploymentSummary == nil {
		return nil, deploymentModels.NonExistingDeployment(nil, deploymentName)
	}

	namespace := operatorUtils.GetEnvironmentNamespace(appName, deploymentSummary.Environment)
	rd, err := deploy.radixClient.RadixV1().RadixDeployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var activeTo time.Time
	if !strings.EqualFold(deploymentSummary.ActiveTo, "") {
		activeTo, err = radixutils.ParseTimestamp(deploymentSummary.ActiveTo)
		if err != nil {
			return nil, err
		}
	}

	components, err := deploy.GetComponentsForDeployment(appName, deploymentSummary)
	if err != nil {
		return nil, err
	}

	return deploymentModels.NewDeploymentBuilder().
		WithRadixDeployment(*rd).
		WithActiveTo(activeTo).
		WithComponents(components).
		BuildDeployment()

}

func (deploy *deployHandler) getEnvironmentNamespaces(appName string) (*corev1.NamespaceList, error) {
	appNameLabel, _ := labels.NewRequirement(kube.RadixAppLabel, selection.Equals, []string{appName})
	envLabel, _ := labels.NewRequirement(kube.RadixEnvLabel, selection.Exists, nil)

	labelSelector := labels.NewSelector().Add(*appNameLabel).Add(*envLabel)

	return deploy.kubeClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector.String()})
}

func (deploy *deployHandler) getDeployments(namespace, appName, jobName string, latest bool) ([]*deploymentModels.DeploymentSummary, error) {
	appNameLabel, err := labels.NewRequirement(kube.RadixAppLabel, selection.Equals, []string{appName})
	if err != nil {
		return nil, err
	}

	rdLabelSelector := labels.NewSelector().Add(*appNameLabel)
	if jobName != "" {
		jobNameLabel, err := labels.NewRequirement(kube.RadixJobNameLabel, selection.Equals, []string{jobName})
		if err != nil {
			return nil, err
		}
		rdLabelSelector = rdLabelSelector.Add(*jobNameLabel)
	}

	var environmentNamespaces []string

	if namespace == corev1.NamespaceAll {
		namespaceList, err := deploy.getEnvironmentNamespaces(appName)
		if err != nil {
			return nil, err
		}
		for _, ns := range namespaceList.Items {
			environmentNamespaces = append(environmentNamespaces, ns.Name)
		}
	} else {
		environmentNamespaces = append(environmentNamespaces, namespace)
	}
	var radixDeploymentList []v1.RadixDeployment

	for _, ns := range environmentNamespaces {
		rdlist, err := deploy.radixClient.RadixV1().RadixDeployments(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: rdLabelSelector.String()})
		if err != nil {
			return nil, err
		}
		radixDeploymentList = append(radixDeploymentList, rdlist.Items...)
	}

	appNamespace := operatorUtils.GetAppNamespace(appName)
	radixJobMap := make(map[string]*v1.RadixJob)
	if jobName != "" {
		radixJob, err := deploy.radixClient.RadixV1().RadixJobs(appNamespace).Get(context.TODO(), jobName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		radixJobMap[radixJob.Name] = radixJob
	} else {
		radixJobList, err := deploy.radixClient.RadixV1().RadixJobs(appNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: appNameLabel.String()})
		if err != nil {
			return nil, err
		}
		for _, rj := range radixJobList.Items {
			rj := rj
			radixJobMap[rj.Name] = &rj
		}
	}

	rds := sortRdsByActiveFromDesc(radixDeploymentList)
	radixDeployments := make([]*deploymentModels.DeploymentSummary, 0)
	for _, rd := range rds {
		if latest && rd.Status.Condition == v1.DeploymentInactive {
			continue
		}

		deploySummary, err := deploymentModels.
			NewDeploymentBuilder().
			WithRadixDeployment(rd).
			WithPipelineJob(radixJobMap[rd.Labels[kube.RadixJobNameLabel]]).
			BuildDeploymentSummary()
		if err != nil {
			return nil, err
		}

		radixDeployments = append(radixDeployments, deploySummary)
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
