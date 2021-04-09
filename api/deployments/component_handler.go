package deployments

import (
	"fmt"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/utils"
	configUtils "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"sort"
	"strings"
)

const (
	radixEnvVariablePrefix      = "RADIX_"
	defaultTargetCPUUtilization = int32(80)
)

// GetComponentsForDeployment Gets a list of components for a given deployment
func (deploy DeployHandler) GetComponentsForDeployment(appName string, deployment *deploymentModels.DeploymentSummary) ([]*deploymentModels.Component, error) {
	return deploy.getComponents(appName, deployment)
}

// GetComponentsForDeploymentName handler for GetDeployments
func (deploy DeployHandler) GetComponentsForDeploymentName(appName, deploymentID string) ([]*deploymentModels.Component, error) {
	deployments, err := deploy.GetDeploymentsForApplication(appName, false)
	if err != nil {
		return nil, err
	}

	for _, deployment := range deployments {
		if deployment.Name != deploymentID {
			continue
		}
		return deploy.getComponents(appName, deployment)
	}

	return nil, deploymentModels.NonExistingDeployment(nil, deploymentID)
}

func (deploy DeployHandler) getComponents(appName string, deployment *deploymentModels.DeploymentSummary) ([]*deploymentModels.Component, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, deployment.Environment)
	rd, err := deploy.radixClient.RadixV1().RadixDeployments(envNs).Get(deployment.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	ra, _ := deploy.radixClient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	components := []*deploymentModels.Component{}

	for _, component := range rd.Spec.Components {
		componentModel, err := deploy.getComponent(&component, ra, rd, deployment)
		if err != nil {
			return nil, err
		}
		components = append(components, componentModel)
	}

	for _, component := range rd.Spec.Jobs {
		componentModel, err := deploy.getComponent(&component, ra, rd, deployment)
		if err != nil {
			return nil, err
		}
		components = append(components, componentModel)
	}

	return components, nil
}

func (deploy DeployHandler) getComponent(component v1.RadixCommonDeployComponent, ra *v1.RadixApplication, rd *v1.RadixDeployment, deployment *deploymentModels.DeploymentSummary) (*deploymentModels.Component, error) {
	envNs := crdUtils.GetEnvironmentNamespace(ra.Name, deployment.Environment)

	// TODO: Add interface for RA + EnvConfig
	environmentConfig := configUtils.GetComponentEnvironmentConfig(ra, deployment.Environment, component.GetName())

	deploymentComponent, err :=
		GetComponentStateFromSpec(deploy.kubeClient, ra.Name, deployment, rd.Status, environmentConfig, component)
	if err != nil {
		return nil, err
	}

	hpa, err := deploy.kubeClient.AutoscalingV1().HorizontalPodAutoscalers(envNs).Get(component.GetName(), metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if err == nil {
		minReplicas := int32(1)
		if hpa.Spec.MinReplicas != nil {
			minReplicas = *hpa.Spec.MinReplicas
		}
		maxReplicas := hpa.Spec.MaxReplicas
		currentCPUUtil := int32(0)
		if hpa.Status.CurrentCPUUtilizationPercentage != nil {
			currentCPUUtil = *hpa.Status.CurrentCPUUtilizationPercentage
		}
		targetCPUUtil := defaultTargetCPUUtilization
		if hpa.Spec.TargetCPUUtilizationPercentage != nil {
			targetCPUUtil = *hpa.Spec.TargetCPUUtilizationPercentage
		}
		hpaSummary := deploymentModels.HorizontalScalingSummary{
			MinReplicas:                     minReplicas,
			MaxReplicas:                     maxReplicas,
			CurrentCPUUtilizationPercentage: currentCPUUtil,
			TargetCPUUtilizationPercentage:  targetCPUUtil,
		}
		deploymentComponent.HorizontalScalingSummary = &hpaSummary
	}

	return deploymentComponent, nil
}

// GetComponentStateFromSpec Returns a component with the current state
func GetComponentStateFromSpec(
	client kubernetes.Interface,
	appName string,
	deployment *deploymentModels.DeploymentSummary,
	deploymentStatus v1.RadixDeployStatus,
	environmentConfig *v1.RadixEnvironmentConfig,
	component v1.RadixCommonDeployComponent) (*deploymentModels.Component, error) {

	var environmentVariables map[string]string

	envNs := crdUtils.GetEnvironmentNamespace(appName, deployment.Environment)
	componentPodNames := &[]string{}
	componentPods := &[]corev1.Pod{}
	replicaSummaryList := []deploymentModels.ReplicaSummary{}
	scheduledJobSummaryList := []deploymentModels.ScheduledJobSummary{}
	status := deploymentModels.ConsistentComponent

	if deployment.ActiveTo == "" {
		// current active deployment - we get existing pods
		componentPodMap, podsOfScheduledJobsMap, err := getComponentPodsByNamespace(client, envNs, component.GetName())
		if err != nil {
			return nil, err
		}
		componentPodNames, componentPods = slicePodNamesAndPodsFromMap(componentPodMap)
		environmentVariables = getRadixEnvironmentVariables(componentPods)
		replicaSummaryList = getReplicaSummaryList(*componentPods)

		scheduledJobs, err := getComponentJobsByNamespace(client, envNs, component.GetName()) //scheduledJobs
		if err != nil {
			return nil, err
		}
		scheduledJobSummaryList = getScheduledJobSummaryList(scheduledJobs, podsOfScheduledJobsMap)
		status, err = getStatusOfActiveDeployment(component,
			deploymentStatus, environmentConfig, *componentPods)
		if err != nil {
			return nil, err
		}
	}

	i := *componentPodNames
	return deploymentModels.NewComponentBuilder().
		WithComponent(component).
		WithStatus(status).
		WithPodNames(i).
		WithReplicaSummaryList(replicaSummaryList).
		WithScheduledJobSummaryList(scheduledJobSummaryList).
		WithRadixEnvironmentVariables(environmentVariables).
		BuildComponent(), nil
}

func getScheduledJobSummaryList(jobs []batchv1.Job, pods *map[string][]corev1.Pod) []deploymentModels.ScheduledJobSummary {
	var summaries []deploymentModels.ScheduledJobSummary
	for _, job := range jobs {
		creationTimestamp := job.GetCreationTimestamp()
		summary := deploymentModels.ScheduledJobSummary{
			Name:    job.Name,
			Created: utils.FormatTimestamp(creationTimestamp.Time),
			Started: utils.FormatTime(job.Status.StartTime),
			Ended:   utils.FormatTime(job.Status.CompletionTime),
			Status:  models.GetStatusFromJobStatus(job.Status).String(),
		}
		if jobPods, ok := (*pods)[job.Name]; ok {
			summary.ReplicaList = getReplicaSummariesForPods(jobPods)
		}
		summaries = append(summaries, summary)
	}

	// Sort job-summaries descending
	sort.Slice(summaries, func(i, j int) bool {
		return utils.IsBefore(&summaries[j], &summaries[i])
	})
	return summaries
}

func getReplicaSummariesForPods(jobPods []corev1.Pod) []deploymentModels.ReplicaSummary {
	var replicaSummaries []deploymentModels.ReplicaSummary
	for _, pod := range jobPods {
		replicaSummaries = append(replicaSummaries, deploymentModels.GetReplicaSummary(pod))
	}
	return replicaSummaries
}

func slicePodNamesAndPodsFromMap(podMap *map[string]corev1.Pod) (*[]string, *[]corev1.Pod) {
	var names []string
	var pods []corev1.Pod
	for name, pod := range *podMap {
		names = append(names, name)
		pods = append(pods, pod)
	}
	return &names, &pods
}

func getComponentPodsByNamespace(client kubernetes.Interface, envNs, componentName string) (*map[string]corev1.Pod, *map[string][]corev1.Pod, error) {
	pods, err := client.CoreV1().Pods(envNs).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixComponentLabel, componentName),
	})
	if err != nil {
		log.Errorf("error getting pods: %v", err)
		return nil, nil, err
	}
	componentPodMap := map[string]corev1.Pod{}
	scheduledJobPodMap := map[string][]corev1.Pod{}
	for _, pod := range pods.Items {
		if jobType, ok := pod.Labels[kube.RadixJobTypeLabel]; !ok || jobType != kube.RadixJobTypeJobSchedule {
			componentPodMap[pod.GetName()] = pod
			continue
		}
		if jobName, ok := pod.Labels["job-name"]; ok {
			list := scheduledJobPodMap[jobName]
			scheduledJobPodMap[jobName] = append(list, pod)
		}
	}
	return &componentPodMap, &scheduledJobPodMap, nil
}

func getComponentJobsByNamespace(client kubernetes.Interface, envNs, componentName string) ([]batchv1.Job, error) {
	labelSelectors := map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
	}
	jobList, err := client.BatchV1().Jobs(envNs).List(metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelSelectors).String(),
	})
	if err != nil {
		log.Errorf("error getting jobs: %v", err)
		return nil, err
	}

	return jobList.Items, nil
}

func runningReplicaDiffersFromConfig(environmentConfig *v1.RadixEnvironmentConfig, actualPods []corev1.Pod) bool {
	actualPodsLength := len(actualPods)
	if environmentConfig != nil {
		// No HPA config
		if environmentConfig.HorizontalScaling == nil {
			if environmentConfig.Replicas != nil {
				return actualPodsLength != *environmentConfig.Replicas
			}
			return actualPodsLength != deployment.DefaultReplicas
		}
		// With HPA config
		if environmentConfig.Replicas != nil && *environmentConfig.Replicas == 0 {
			return actualPodsLength != *environmentConfig.Replicas
		}
		if environmentConfig.HorizontalScaling.MinReplicas != nil {
			return actualPodsLength < int(*environmentConfig.HorizontalScaling.MinReplicas) ||
				actualPodsLength > int(environmentConfig.HorizontalScaling.MaxReplicas)
		}
		return actualPodsLength < deployment.DefaultReplicas ||
			actualPodsLength > int(environmentConfig.HorizontalScaling.MaxReplicas)
	}
	return actualPodsLength != deployment.DefaultReplicas
}

func runningReplicaDiffersFromSpec(component v1.RadixCommonDeployComponent, actualPods []corev1.Pod) bool {
	actualPodsLength := len(actualPods)
	// No HPA config
	if component.GetHorizontalScaling() == nil {
		if component.GetReplicas() != nil {
			return actualPodsLength != *component.GetReplicas()
		}
		return actualPodsLength != deployment.DefaultReplicas
	}
	// With HPA config
	if component.GetReplicas() != nil && *component.GetReplicas() == 0 {
		return actualPodsLength != *component.GetReplicas()
	}
	if component.GetHorizontalScaling().MinReplicas != nil {
		return actualPodsLength < int(*component.GetHorizontalScaling().MinReplicas) ||
			actualPodsLength > int(component.GetHorizontalScaling().MaxReplicas)
	}
	return actualPodsLength < deployment.DefaultReplicas ||
		actualPodsLength > int(component.GetHorizontalScaling().MaxReplicas)
}

func getRadixEnvironmentVariables(pods *[]corev1.Pod) map[string]string {
	radixEnvironmentVariables := make(map[string]string)

	for _, pod := range *pods {
		for _, container := range pod.Spec.Containers {
			for _, envVariable := range container.Env {
				if strings.HasPrefix(envVariable.Name, radixEnvVariablePrefix) {
					radixEnvironmentVariables[envVariable.Name] = envVariable.Value
				}
			}
		}
	}

	return radixEnvironmentVariables
}

func getReplicaSummaryList(pods []corev1.Pod) []deploymentModels.ReplicaSummary {
	var replicaSummaryList []deploymentModels.ReplicaSummary

	for _, pod := range pods {
		replicaSummaryList = append(replicaSummaryList, deploymentModels.GetReplicaSummary(pod))
	}

	return replicaSummaryList
}

func runningReplicaIsOutdated(component v1.RadixCommonDeployComponent, actualPods []corev1.Pod) bool {
	switch component.GetType() {
	case defaults.RadixComponentTypeComponent:
		return runningComponentReplicaIsOutdated(component, actualPods)
	case defaults.RadixComponentTypeJobScheduler:
		return false
	default:
		return false
	}
}

func runningComponentReplicaIsOutdated(component v1.RadixCommonDeployComponent, actualPods []corev1.Pod) bool {
	// Check if running component's image is not the same as active deployment image tag and that active rd image is equal to 'starting' component image tag
	componentIsInconsistent := false
	for _, pod := range actualPods {
		if pod.DeletionTimestamp != nil {
			// Pod is in termination phase
			continue
		}
		for _, container := range pod.Spec.Containers {
			if container.Image != component.GetImage() {
				// Container is running an outdate image
				componentIsInconsistent = true
			}
		}
	}

	return componentIsInconsistent
}

func getStatusOfActiveDeployment(
	component v1.RadixCommonDeployComponent,
	deploymentStatus v1.RadixDeployStatus,
	environmentConfig *v1.RadixEnvironmentConfig,
	pods []corev1.Pod) (deploymentModels.ComponentStatus, error) {

	status := deploymentModels.ConsistentComponent

	if runningReplicaDiffersFromConfig(environmentConfig, pods) &&
		!runningReplicaDiffersFromSpec(component, pods) &&
		len(pods) == 0 {
		status = deploymentModels.StoppedComponent
	} else if runningReplicaIsOutdated(component, pods) {
		status = deploymentModels.ComponentOutdated
	} else if runningReplicaDiffersFromSpec(component, pods) {
		status = deploymentModels.ComponentReconciling
	} else {
		restarted := (*component.GetEnvironmentVariables())[defaults.RadixRestartEnvironmentVariable]
		if !strings.EqualFold(restarted, "") {
			restartedTime, err := utils.ParseTimestamp(restarted)
			if err != nil {
				return status, err
			}

			reconciledTime := deploymentStatus.Reconciled
			if reconciledTime.IsZero() || restartedTime.After(reconciledTime.Time) {
				status = deploymentModels.ComponentRestarting
			}
		}
	}

	return status, nil
}
