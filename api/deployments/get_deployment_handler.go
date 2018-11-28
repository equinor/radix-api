package deployments

import (
	"fmt"
	"sort"
	"strings"

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

// HandleGetLogs handler for GetLogs
func (deploy DeployHandler) HandleGetLogs(appName, podName string) (string, error) {
	ns := crdUtils.GetAppNamespace(appName)
	// TODO! rewrite to use deploymentId to find pod (rd.Env -> namespace -> pod)
	ra, err := deploy.radixClient.RadixV1().RadixApplications(ns).Get(appName, metav1.GetOptions{})
	if err != nil {
		return "", nonExistingApplication(err, appName)
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
	return "", nonExistingPod(appName, podName)
}

// HandleGetDeployments handler for GetDeployments
func (deploy DeployHandler) HandleGetDeployments(appName, environment string, latest bool) ([]*deploymentModels.DeploymentSummary, error) {
	var listOptions metav1.ListOptions
	if strings.TrimSpace(appName) != "" {
		listOptions.LabelSelector = fmt.Sprintf("radixApp=%s", appName)
	}

	var namespace = corev1.NamespaceAll
	if strings.TrimSpace(appName) != "" && strings.TrimSpace(environment) != "" {
		namespace = crdUtils.GetEnvironmentNamespace(appName, environment)
	}

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

// HandleGetDeployment Handler for GetDeployment
func (deploy DeployHandler) HandleGetDeployment(appName, deploymentName string) (*deploymentModels.Deployment, error) {
	return nil, nil
}

// GetDeploymentsForJob Lists deployments for job name
func (deploy DeployHandler) GetDeploymentsForJob(appName, jobName string) ([]*deploymentModels.DeploymentSummary, error) {
	deployments, err := deploy.HandleGetDeployments(appName, "", false)
	if err != nil {
		return nil, err
	}

	deploymentsForJob := []*deploymentModels.DeploymentSummary{}
	for _, deployment := range deployments {
		if deployment.CreatedByJob == jobName {
			deploymentsForJob = append(deploymentsForJob, deployment)
		}
	}

	return deploymentsForJob, nil
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
