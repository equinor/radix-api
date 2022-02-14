package deployments

import (
	"context"
	"fmt"
	"sort"
	"strings"

	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/api/utils/labelselector"
	radixutils "github.com/equinor/radix-common/utils"
	jobSchedulerModels "github.com/equinor/radix-job-scheduler/models"
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
)

const (
	defaultTargetCPUUtilization = int32(80)
	k8sJobNameLabel             = "job-name" // A label that k8s automatically adds to a Pod created by a Job
)

// GetComponentsForDeployment Gets a list of components for a given deployment
func (deploy *deployHandler) GetComponentsForDeployment(appName string, deployment *deploymentModels.DeploymentSummary) ([]*deploymentModels.Component, error) {
	return deploy.getComponents(appName, deployment)
}

// GetComponentsForDeploymentName handler for GetDeployments
func (deploy *deployHandler) GetComponentsForDeploymentName(appName, deploymentID string) ([]*deploymentModels.Component, error) {
	deployments, err := deploy.GetDeploymentsForApplication(appName, false)
	if err != nil {
		return nil, err
	}

	for _, depl := range deployments {
		if depl.Name != deploymentID {
			continue
		}
		return deploy.getComponents(appName, depl)
	}

	return nil, deploymentModels.NonExistingDeployment(nil, deploymentID)
}

func (deploy *deployHandler) getComponents(appName string, deployment *deploymentModels.DeploymentSummary) ([]*deploymentModels.Component, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, deployment.Environment)
	rd, err := deploy.radixClient.RadixV1().RadixDeployments(envNs).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	ra, _ := deploy.radixClient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(context.TODO(), appName, metav1.GetOptions{})
	var components []*deploymentModels.Component

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

func (deploy *deployHandler) getComponent(component v1.RadixCommonDeployComponent, ra *v1.RadixApplication, rd *v1.RadixDeployment, deployment *deploymentModels.DeploymentSummary) (*deploymentModels.Component, error) {
	envNs := crdUtils.GetEnvironmentNamespace(ra.Name, deployment.Environment)

	// TODO: Add interface for RA + EnvConfig
	environmentConfig := configUtils.GetComponentEnvironmentConfig(ra, deployment.Environment, component.GetName())

	deploymentComponent, err :=
		GetComponentStateFromSpec(deploy.kubeClient, ra.Name, deployment, rd.Status, environmentConfig, component)
	if err != nil {
		return nil, err
	}

	hpa, err := deploy.kubeClient.AutoscalingV1().HorizontalPodAutoscalers(envNs).Get(context.TODO(), component.GetName(), metav1.GetOptions{})
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
	kubeClient kubernetes.Interface,
	appName string,
	deployment *deploymentModels.DeploymentSummary,
	deploymentStatus v1.RadixDeployStatus,
	environmentConfig *v1.RadixEnvironmentConfig,
	component v1.RadixCommonDeployComponent) (*deploymentModels.Component, error) {

	var environmentVariables map[string]string

	envNs := crdUtils.GetEnvironmentNamespace(appName, deployment.Environment)
	var componentPodNames []string

	var replicaSummaryList []deploymentModels.ReplicaSummary
	var scheduledJobSummaryList []deploymentModels.ScheduledJobSummary
	var auxResource deploymentModels.AuxiliaryResource
	status := deploymentModels.ConsistentComponent

	if deployment.ActiveTo == "" {
		// current active deployment - we get existing pods
		componentPods, err := getComponentPodsByNamespace(kubeClient, envNs, component.GetName())
		if err != nil {
			return nil, err
		}
		componentPodNames = getPodNames(componentPods)
		environmentVariables = getRadixEnvironmentVariables(componentPods)
		replicaSummaryList = getReplicaSummaryList(componentPods)
		auxResource, err = getAuxiliaryResources(kubeClient, appName, component, envNs)
		if err != nil {
			return nil, err
		}

		if component.GetType() == v1.RadixComponentTypeJobScheduler {
			scheduledJobs, scheduledJobPodMap, err := getComponentJobsByNamespace(kubeClient, envNs, component.GetName()) //scheduledJobs
			if err != nil {
				return nil, err
			}
			scheduledJobSummaryList = getScheduledJobSummaryList(kubeClient, scheduledJobs, scheduledJobPodMap)
		}

		status, err = getStatusOfActiveDeployment(component,
			deploymentStatus, environmentConfig, componentPods)
		if err != nil {
			return nil, err
		}
	}

	componentBuilder := deploymentModels.NewComponentBuilder()
	if jobComponent, ok := component.(*v1.RadixDeployJobComponent); ok {
		componentBuilder.WithSchedulerPort(jobComponent.SchedulerPort)
		if jobComponent.Payload != nil {
			componentBuilder.WithScheduledJobPayloadPath(jobComponent.Payload.Path)
		}
	}

	return componentBuilder.
		WithComponent(component).
		WithStatus(status).
		WithPodNames(componentPodNames).
		WithReplicaSummaryList(replicaSummaryList).
		WithScheduledJobSummaryList(scheduledJobSummaryList).
		WithRadixEnvironmentVariables(environmentVariables).
		WithAuxiliaryResource(auxResource).
		BuildComponent()

}

func getScheduledJobSummaryList(kubeClient kubernetes.Interface, jobs []batchv1.Job, jobPodsMap map[string][]corev1.Pod) []deploymentModels.ScheduledJobSummary {
	var summaries []deploymentModels.ScheduledJobSummary
	for _, job := range jobs {
		creationTimestamp := job.GetCreationTimestamp()
		summary := deploymentModels.ScheduledJobSummary{
			Name:    job.Name,
			Created: radixutils.FormatTimestamp(creationTimestamp.Time),
			Started: radixutils.FormatTime(job.Status.StartTime),
			Ended:   radixutils.FormatTime(job.Status.CompletionTime),
		}
		if jobPods, ok := jobPodsMap[job.Name]; ok {
			summary.ReplicaList = getReplicaSummariesForPods(jobPods)
		}
		jobStatus := jobSchedulerModels.GetJobStatusFromJob(kubeClient, &job, jobPodsMap[job.Name])
		summary.Status = jobStatus.Status
		summary.Message = jobStatus.Message
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

func getPodNames(pods []corev1.Pod) []string {
	var names []string
	for _, pod := range pods {
		names = append(names, pod.GetName())
	}
	return names
}

func getComponentPodsByNamespace(client kubernetes.Interface, envNs, componentName string) ([]corev1.Pod, error) {
	var componentPods []corev1.Pod
	pods, err := client.CoreV1().Pods(envNs).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixComponentLabel, componentName),
	})
	if err != nil {
		log.Errorf("error getting pods: %v", err)
		return nil, err
	}

	for _, pod := range pods.Items {
		pod := pod

		// A previous version of the job-scheduler added the "radix-component" label to job pods.
		// For backward compatibility, we need to ignore these pods in the list of pods returned for a component
		if _, isScheduledJobPod := pod.GetLabels()[kube.RadixJobTypeLabel]; !isScheduledJobPod {
			componentPods = append(componentPods, pod)
		}
	}

	return componentPods, nil
}

func getComponentJobsByNamespace(client kubernetes.Interface, envNs, componentName string) ([]batchv1.Job, map[string][]corev1.Pod, error) {
	jobLabelSelector := map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
	}
	jobList, err := client.BatchV1().Jobs(envNs).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(jobLabelSelector).String(),
	})
	if err != nil {
		log.Errorf("error getting jobs: %v", err)
		return nil, nil, err
	}

	jobPodMap := make(map[string][]corev1.Pod)

	// Make API call to k8s only if there are actual jobs
	if len(jobList.Items) > 0 {
		jobPodLabelSelector := labels.Set{
			kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule,
		}
		podList, err := client.CoreV1().Pods(envNs).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(jobPodLabelSelector).String(),
		})
		if err != nil {
			return nil, nil, err
		}

		for _, pod := range podList.Items {
			pod := pod
			if jobName, labelExist := pod.GetLabels()[k8sJobNameLabel]; labelExist {
				jobPodList := jobPodMap[jobName]
				jobPodMap[jobName] = append(jobPodList, pod)
			}
		}

	}

	return jobList.Items, jobPodMap, nil
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

func getRadixEnvironmentVariables(pods []corev1.Pod) map[string]string {
	radixEnvironmentVariables := make(map[string]string)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			for _, envVariable := range container.Env {
				if crdUtils.IsRadixEnvVar(envVariable.Name) {
					radixEnvironmentVariables[envVariable.Name] = envVariable.Value
				}
			}
		}
	}

	return radixEnvironmentVariables
}

func getReplicaSummaryList(pods []corev1.Pod) []deploymentModels.ReplicaSummary {
	replicaSummaryList := make([]deploymentModels.ReplicaSummary, 0, len(pods))

	for _, pod := range pods {
		replicaSummaryList = append(replicaSummaryList, deploymentModels.GetReplicaSummary(pod))
	}

	return replicaSummaryList
}

func getAuxiliaryResources(kubeClient kubernetes.Interface, appName string, component v1.RadixCommonDeployComponent, envNamespace string) (auxResource deploymentModels.AuxiliaryResource, err error) {
	if auth := component.GetAuthentication(); auth != nil && auth.OAuth2 != nil {
		auxResource.OAuth2, err = getOAuth2AuxiliaryResource(kubeClient, appName, component.GetName(), envNamespace, *auth.OAuth2)
		if err != nil {
			return
		}
	}

	return
}

func getOAuth2AuxiliaryResource(kubeClient kubernetes.Interface, appName, componentName, envNamespace string, oauth2 v1.OAuth2) (*deploymentModels.OAuth2AuxiliaryResource, error) {
	var oauth2Resource deploymentModels.OAuth2AuxiliaryResource
	oauthDeployment, err := getAuxiliaryResourceDeployment(kubeClient, appName, componentName, envNamespace, defaults.OAuthProxyAuxiliaryComponentType)
	if err != nil {
		return nil, err
	}
	if oauthDeployment != nil {
		oauth2Resource.Deployment = *oauthDeployment
	}

	return &oauth2Resource, nil

}

func getAuxiliaryResourceDeployment(kubeClient kubernetes.Interface, appName, componentName, envNamespace, auxType string) (*deploymentModels.AuxiliaryResourceDeployment, error) {
	var auxResourceDeployment deploymentModels.AuxiliaryResourceDeployment

	selector := labelselector.ForAuxiliaryResource(appName, componentName, auxType).String()
	deployments, err := kubeClient.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	if len(deployments.Items) == 0 {
		auxResourceDeployment.Status = deploymentModels.ComponentReconciling.String()
		return &auxResourceDeployment, nil
	}
	deployment := deployments.Items[0]

	pods, err := kubeClient.CoreV1().Pods(envNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	auxResourceDeployment.ReplicaList = getReplicaSummaryList(pods.Items)
	auxResourceDeployment.Status = deploymentModels.ComponentStatusFromDeployment(&deployment).String()
	return &auxResourceDeployment, nil
}

func runningReplicaIsOutdated(component v1.RadixCommonDeployComponent, actualPods []corev1.Pod) bool {
	switch component.GetType() {
	case v1.RadixComponentTypeComponent:
		return runningComponentReplicaIsOutdated(component, actualPods)
	case v1.RadixComponentTypeJobScheduler:
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
				// Container is running an outdated image
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
		restarted := component.GetEnvironmentVariables()[defaults.RadixRestartEnvironmentVariable]
		if !strings.EqualFold(restarted, "") {
			restartedTime, err := radixutils.ParseTimestamp(restarted)
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
