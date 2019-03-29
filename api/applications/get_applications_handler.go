package applications

import (
	"sort"
	"strings"

	applicationModels "github.com/equinor/radix-api/api/applications/models"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	log "github.com/sirupsen/logrus"

	authorizationapi "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type hasAccessToRR func(client kubernetes.Interface, rr v1.RadixRegistration) bool

// GetApplications handler for ShowApplications
func (ah ApplicationHandler) GetApplications(sshRepo string, hasAccess hasAccessToRR) ([]*applicationModels.ApplicationSummary, error) {
	radixRegistationList, err := ah.serviceAccount.RadixClient.RadixV1().RadixRegistrations().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	radixRegistations := ah.filterRadixRegByAccessAndSSHRepo(radixRegistationList.Items, sshRepo, hasAccess)

	applicationJobs, err := ah.getJobsForApplication(radixRegistations)
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
	adGroups := map[string]int{}
	result := []v1.RadixRegistration{}

	for _, rr := range radixregs {
		if filterOnSSHRepo(&rr, sshURL) {
			continue
		}

		adGroupsForApp := rr.Spec.AdGroups
		sort.Strings(adGroupsForApp)
		adGroupsAsKey := strings.Join(adGroupsForApp, ",")
		if adGroups[adGroupsAsKey] == 1 {
			result = append(result, rr)
		} else if adGroups[adGroupsAsKey] == -1 {
			continue
		} else if hasAccess(ah.userAccount.Client, rr) {
			adGroups[adGroupsAsKey] = 1

			result = append(result, rr)
		} else {
			adGroups[adGroupsAsKey] = -1
		}
	}

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
	return client.AuthorizationV1().SelfSubjectAccessReviews().Create(&sar)
}
