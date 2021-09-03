package applications

import (
	"context"
	"sort"
	"strings"
	"time"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	log "github.com/sirupsen/logrus"

	authorizationapi "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type hasAccessToRR func(client kubernetes.Interface, rr v1.RadixRegistration) bool

// GetApplications handler for ShowApplications - NOTE: does not get latestJob.Environments
func (ah ApplicationHandler) GetApplications(sshRepo string, hasAccess hasAccessToRR) ([]*applicationModels.ApplicationSummary, error) {
	start := time.Now()
	radixRegistationList, err := ah.getServiceAccount().RadixClient.RadixV1().RadixRegistrations().List(context.TODO(), metav1.ListOptions{})
	log.Debugf("get all application took %s", time.Since(start))
	if err != nil {
		return nil, err
	}

	start = time.Now()
	radixRegistations := ah.filterRadixRegByAccessAndSSHRepo(radixRegistationList.Items, sshRepo, hasAccess)
	log.Debugf("check application permission took %s", time.Since(start))

	start = time.Now()
	applicationJobs, err := ah.getJobsForApplication(radixRegistations)
	log.Debugf("get application jobs took %s", time.Since(start))
	if err != nil {
		return nil, err
	}

	applications := make([]*applicationModels.ApplicationSummary, 0)
	for _, rr := range radixRegistations {
		jobSummary := applicationJobs[rr.Name]
		applications = append(applications, &applicationModels.ApplicationSummary{Name: rr.Name, LatestJob: jobSummary})
	}

	return applications, nil
}

func (ah ApplicationHandler) getJobsForApplication(radixRegistations []v1.RadixRegistration) (map[string]*jobModels.JobSummary, error) {
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

func (ah ApplicationHandler) filterRadixRegByAccessAndSSHRepo(radixregs []v1.RadixRegistration, sshURL string, hasAccess hasAccessToRR) []v1.RadixRegistration {
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

			if filterOnSSHRepo(&rr, sshURL) {
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

func filterOnSSHRepo(rr *v1.RadixRegistration, sshURL string) bool {
	filter := true

	if strings.TrimSpace(sshURL) == "" ||
		strings.EqualFold(rr.Spec.CloneURL, sshURL) {
		filter = false
	}

	return filter
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
