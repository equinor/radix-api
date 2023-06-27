package applications

import (
	"context"
	"sort"
	"strings"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	deployment "github.com/equinor/radix-api/api/deployments"
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	authorizationapi "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type hasAccessToRR func(client kubernetes.Interface, rr v1.RadixRegistration) bool

type GetApplicationsOptions struct {
	IncludeLatestJobSummary            bool // include LatestJobSummary
	IncludeEnvironmentActiveComponents bool // include Environment ActiveDeployment components
}

// GetApplications handler for ShowApplications - NOTE: does not get latestJob.Environments
func (ah *ApplicationHandler) GetApplications(matcher applicationModels.ApplicationMatch, hasAccess hasAccessToRR, options GetApplicationsOptions) ([]*applicationModels.ApplicationSummary, error) {
	radixRegistationList, err := ah.getServiceAccount().RadixClient.RadixV1().RadixRegistrations().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	filteredRegistrations := make([]v1.RadixRegistration, 0, len(radixRegistationList.Items))
	for _, rr := range radixRegistationList.Items {
		if matcher(&rr) {
			filteredRegistrations = append(filteredRegistrations, rr)
		}
	}

	radixRegistrations := ah.filterRadixRegByAccess(filteredRegistrations, hasAccess)

	var latestApplicationJobs map[string]*jobModels.JobSummary
	if options.IncludeLatestJobSummary {
		if latestApplicationJobs, err = ah.getJobsForApplication(radixRegistrations); err != nil {
			return nil, err
		}
	}

	var environmentActiveComponents map[string]map[string][]*deploymentModels.Component
	if options.IncludeEnvironmentActiveComponents {
		if environmentActiveComponents, err = ah.getEnvironmentActiveComponentsForApplications(radixRegistrations); err != nil {
			return nil, err
		}
	}

	applications := make([]*applicationModels.ApplicationSummary, 0)
	for _, rr := range radixRegistrations {
		appName := rr.GetName()
		applications = append(
			applications,
			&applicationModels.ApplicationSummary{
				Name:                        appName,
				LatestJob:                   latestApplicationJobs[appName],
				EnvironmentActiveComponents: environmentActiveComponents[appName],
			},
		)
	}
	return applications, nil
}

func (ah *ApplicationHandler) getEnvironmentActiveComponentsForApplications(radixRegistrations []v1.RadixRegistration) (map[string]map[string][]*deploymentModels.Component, error) {
	type ChannelData struct {
		key           string
		envComponents map[string][]*deploymentModels.Component
	}

	var g errgroup.Group
	g.SetLimit(10)

	deploy := deployment.Init(ah.accounts)
	chanData := make(chan *ChannelData, len(radixRegistrations))
	for _, rr := range radixRegistrations {
		appName := rr.GetName()
		g.Go(func() error {
			environments, err := ah.environmentHandler.GetEnvironmentSummary(appName)
			if err != nil {
				return err
			}

			envComponents, err := getComponentsForActiveDeploymentsInEnvironments(deploy, appName, environments)
			if err == nil {
				chanData <- &ChannelData{key: appName, envComponents: envComponents}
			}
			return err
		})
	}

	err := g.Wait()
	close(chanData)
	if err != nil {
		return nil, err
	}

	envComponents := make(map[string]map[string][]*deploymentModels.Component)
	for data := range chanData {
		envComponents[data.key] = data.envComponents
	}
	return envComponents, nil
}

func getComponentsForActiveDeploymentsInEnvironments(deploy deployment.DeployHandler, appName string, environments []*environmentModels.EnvironmentSummary) (map[string][]*deploymentModels.Component, error) {
	type ChannelData struct {
		key        string
		components []*deploymentModels.Component
	}

	var g errgroup.Group
	g.SetLimit(5)

	chanData := make(chan *ChannelData, len(environments))
	for _, env := range environments {
		deployment := env.ActiveDeployment
		if deployment == nil || deployment.ActiveTo != "" {
			continue
		}

		envName := env.Name
		g.Go(func() error {
			componentModels, err := deploy.GetComponentsForDeployment(appName, deployment)
			if err == nil {
				chanData <- &ChannelData{key: envName, components: componentModels}
			}
			return err
		})
	}

	err := g.Wait()
	close(chanData)
	if err != nil {
		return nil, err
	}

	components := make(map[string][]*deploymentModels.Component)
	for data := range chanData {
		components[data.key] = data.components
	}
	return components, nil
}

func (ah *ApplicationHandler) getJobsForApplication(radixRegistations []v1.RadixRegistration) (map[string]*jobModels.JobSummary, error) {
	forApplications := map[string]bool{}
	for _, app := range radixRegistations {
		forApplications[app.GetName()] = true
	}

	applicationJobs, err := ah.jobHandler.GetLatestJobPerApplication(forApplications)
	if err != nil {
		return nil, err
	}
	return applicationJobs, nil
}

func (ah *ApplicationHandler) filterRadixRegByAccess(radixregs []v1.RadixRegistration, hasAccess hasAccessToRR) []v1.RadixRegistration {
	result := []v1.RadixRegistration{}

	limit := 25
	semaphore := make(chan struct{}, limit)
	rrChan := make(chan v1.RadixRegistration, len(radixregs))
	kubeClient := ah.getUserAccount().Client
	for _, rr := range radixregs {
		semaphore <- struct{}{}
		go func(rr v1.RadixRegistration) {
			defer func() { <-semaphore }()

			if rr.Status.Reconciled.IsZero() {
				return
			}

			if hasAccess(kubeClient, rr) {
				rrChan <- rr
			}
		}(rr)
	}

	// Wait for goroutines to release semaphore channel
	for i := limit; i > 0; i-- {
		semaphore <- struct{}{}
	}
	close(semaphore)
	close(rrChan)

	for rr := range rrChan {
		result = append(result, rr)
	}

	sort.Slice(result, func(i, j int) bool {
		return strings.Compare(result[i].Name, result[j].Name) == -1
	})
	return result
}

// cannot run as test - does not return correct values
func hasAccess(client kubernetes.Interface, rr v1.RadixRegistration) bool {
	sar := authorizationapi.SelfSubjectAccessReview{
		Spec: authorizationapi.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationapi.ResourceAttributes{
				Verb:     "get",
				Group:    "radix.equinor.com",
				Resource: "radixregistrations",
				Version:  "*",
				Name:     rr.GetName(),
			},
		},
	}

	r, err := postSelfSubjectAccessReviews(client, sar)
	if err != nil {
		log.Warnf("failed to verify access: %v", err)
		return false
	}
	return r.Status.Allowed
}

func postSelfSubjectAccessReviews(client kubernetes.Interface, sar authorizationapi.SelfSubjectAccessReview) (*authorizationapi.SelfSubjectAccessReview, error) {
	return client.AuthorizationV1().SelfSubjectAccessReviews().Create(context.TODO(), &sar, metav1.CreateOptions{})
}
