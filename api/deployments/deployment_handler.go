package deployments

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/pods"
	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"go.elastic.co/apm"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type DeployHandler interface {
	GetLogs(ctx context.Context, appName, podName string, sinceTime *time.Time, logLines *int64, previousLog bool) (io.ReadCloser, error)
	GetDeploymentWithName(ctx context.Context, appName, deploymentName string) (*deploymentModels.Deployment, error)
	GetDeploymentsForApplicationEnvironment(ctx context.Context, appName, environment string, latest bool) ([]*deploymentModels.DeploymentSummary, error)
	GetComponentsForDeploymentName(ctx context.Context, appName, deploymentID string) ([]*deploymentModels.Component, error)
	GetComponentsForDeployment(ctx context.Context, appName string, deployment *deploymentModels.DeploymentSummary) ([]*deploymentModels.Component, error)
	GetLatestDeploymentForApplicationEnvironment(ctx context.Context, appName, environment string) (*deploymentModels.DeploymentSummary, error)
	GetDeploymentsForJob(ctx context.Context, appName, jobName string) ([]*deploymentModels.DeploymentSummary, error)
}

// DeployHandler Instance variables
type deployHandler struct {
	accounts models.Accounts
}

// Init Constructor
func Init(accounts models.Accounts) DeployHandler {
	return &deployHandler{
		accounts: accounts,
	}
}

// GetLogs handler for GetLogs
func (deploy *deployHandler) GetLogs(ctx context.Context, appName, podName string, sinceTime *time.Time, logLines *int64, previousLog bool) (io.ReadCloser, error) {
	ns := operatorUtils.GetAppNamespace(appName)
	// TODO! rewrite to use deploymentId to find pod (rd.Env -> namespace -> pod)
	ra, err := deploy.accounts.UserAccount.RadixClient.RadixV1().RadixApplications(ns).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, deploymentModels.NonExistingApplication(err, appName)
	}
	for _, env := range ra.Spec.Environments {
		podHandler := pods.Init(deploy.accounts.UserAccount.Client)
		log, err := podHandler.HandleGetEnvironmentPodLog(ctx, appName, env.Name, podName, "", sinceTime, logLines, previousLog)
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
func (deploy *deployHandler) GetDeploymentsForApplication(ctx context.Context, appName string) ([]*deploymentModels.DeploymentSummary, error) {
	environments, err := deploy.getEnvironmentNames(ctx, appName)
	if err != nil {
		return nil, err
	}
	return deploy.getDeployments(ctx, appName, environments, "", false)
}

// GetLatestDeploymentForApplicationEnvironment Gets latest, active, deployment in environment
func (deploy *deployHandler) GetLatestDeploymentForApplicationEnvironment(ctx context.Context, appName, environment string) (*deploymentModels.DeploymentSummary, error) {
	if strings.TrimSpace(environment) == "" {
		return nil, deploymentModels.IllegalEmptyEnvironment()
	}

	deploymentSummaries, err := deploy.getDeployments(ctx, appName, []string{environment}, "", true)
	if err == nil && len(deploymentSummaries) == 1 {
		return deploymentSummaries[0], nil
	}

	return nil, deploymentModels.NoActiveDeploymentFoundInEnvironment(appName, environment)
}

// GetDeploymentsForApplicationEnvironment Lists deployments inside environment
func (deploy *deployHandler) GetDeploymentsForApplicationEnvironment(ctx context.Context, appName, environment string, latest bool) ([]*deploymentModels.DeploymentSummary, error) {
	span, ctx := apm.StartSpan(ctx, fmt.Sprintf("GetDeploymentsForApplicationEnvironment (appName=%s, envName=%s)", appName, environment), "DeployHandler")
	defer span.End()
	var environments []string
	if strings.TrimSpace(environment) != "" {
		environments = append(environments, environment)
	} else {
		envs, err := deploy.getEnvironmentNames(ctx, appName)
		if err != nil {
			return nil, err
		}
		environments = append(environments, envs...)
	}

	deployments, err := deploy.getDeployments(ctx, appName, environments, "", latest)
	return deployments, err
}

// GetDeploymentsForJob Lists deployments for job name
func (deploy *deployHandler) GetDeploymentsForJob(ctx context.Context, appName, jobName string) ([]*deploymentModels.DeploymentSummary, error) {
	environments, err := deploy.getEnvironmentNames(ctx, appName)
	if err != nil {
		return nil, err
	}

	return deploy.getDeployments(ctx, appName, environments, jobName, false)
}

// GetDeploymentWithName Handler for GetDeploymentWithName
func (deploy *deployHandler) GetDeploymentWithName(ctx context.Context, appName, deploymentName string) (*deploymentModels.Deployment, error) {
	// Need to list all deployments to find active to of deployment
	allDeployments, err := deploy.GetDeploymentsForApplication(ctx, appName)
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
	rd, err := deploy.accounts.UserAccount.RadixClient.RadixV1().RadixDeployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	components, err := deploy.GetComponentsForDeployment(ctx, appName, deploymentSummary)
	if err != nil {
		return nil, err
	}

	// getting RadixDeployment's RadixRegistration to fetch git repository url
	rr, err := deploy.accounts.UserAccount.RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	dep, _ := deploymentModels.NewDeploymentBuilder().
		WithRadixDeployment(rd).
		WithComponents(components).
		WithGitCommitHash(rd.Annotations[kube.RadixCommitAnnotation]).
		WithGitTags(rd.Annotations[kube.RadixGitTagsAnnotation]).
		WithRadixRegistration(rr).
		BuildDeployment()

	return dep, nil
}

func (deploy *deployHandler) getEnvironmentNames(ctx context.Context, appName string) ([]string, error) {
	radixlabels.ForApplicationName(appName).AsSelector()
	labelSelector := radixlabels.ForApplicationName(appName).AsSelector()

	reList, err := deploy.accounts.ServiceAccount.RadixClient.RadixV1().RadixEnvironments().List(ctx, metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return nil, err
	}

	return slice.Map(reList.Items, func(re v1.RadixEnvironment) string {
		return re.Spec.EnvName
	}), nil
}

func (deploy *deployHandler) getDeployments(ctx context.Context, appName string, environments []string, jobName string, latest bool) ([]*deploymentModels.DeploymentSummary, error) {
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

	var radixDeploymentList []v1.RadixDeployment
	namespaces := slice.Map(environments, func(env string) string { return operatorUtils.GetEnvironmentNamespace(appName, env) })
	for _, ns := range namespaces {
		rdlist, err := deploy.accounts.UserAccount.RadixClient.RadixV1().RadixDeployments(ns).List(ctx, metav1.ListOptions{LabelSelector: rdLabelSelector.String()})
		if err != nil {
			return nil, err
		}
		radixDeploymentList = append(radixDeploymentList, rdlist.Items...)
	}

	appNamespace := operatorUtils.GetAppNamespace(appName)
	radixJobMap := make(map[string]*v1.RadixJob)

	if jobName != "" {
		radixJob, err := deploy.accounts.UserAccount.RadixClient.RadixV1().RadixJobs(appNamespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		radixJobMap[radixJob.Name] = radixJob
	} else {
		radixJobList, err := deploy.accounts.UserAccount.RadixClient.RadixV1().RadixJobs(appNamespace).List(ctx, metav1.ListOptions{LabelSelector: appNameLabel.String()})
		if err != nil {
			return nil, err
		}
		for _, rj := range radixJobList.Items {
			rj := rj
			radixJobMap[rj.Name] = &rj
		}
	}

	// getting RadixDeployment's RadixRegistration to fetch git repository url
	rr, err := deploy.accounts.UserAccount.RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	rds := sortRdsByActiveFromDesc(radixDeploymentList)
	var deploymentSummaries []*deploymentModels.DeploymentSummary
	for _, rd := range rds {
		if latest && rd.Status.Condition == v1.DeploymentInactive {
			continue
		}

		deploySummary, err := deploymentModels.
			NewDeploymentBuilder().
			WithRadixDeployment(&rd).
			WithPipelineJob(radixJobMap[rd.Labels[kube.RadixJobNameLabel]]).
			WithRadixRegistration(rr).
			BuildDeploymentSummary()
		if err != nil {
			return nil, err
		}

		deploymentSummaries = append(deploymentSummaries, deploySummary)
	}

	return deploymentSummaries, nil
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
