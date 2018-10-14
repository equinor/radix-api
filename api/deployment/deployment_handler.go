package deployment

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-api/api/utils"
	"k8s.io/client-go/kubernetes"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HandleGetDeployments handler for GetDeployments
func HandleGetDeployments(radixclient radixclient.Interface, appName, environment string, latest bool) ([]*ApplicationDeployment, error) {
	var listOptions metav1.ListOptions
	if strings.TrimSpace(appName) != "" {
		listOptions.LabelSelector = fmt.Sprintf("radixApp=%s", appName)
	}

	var namespace = corev1.NamespaceAll
	if strings.TrimSpace(appName) != "" && strings.TrimSpace(environment) != "" {
		namespace = getNameSpaceForApplicationEnvironment(appName, environment)
	}

	radixDeploymentList, err := radixclient.RadixV1().RadixDeployments(namespace).List(listOptions)

	if err != nil {
		return nil, err
	}

	radixDeployments := make([]*ApplicationDeployment, 0)
	for _, rd := range radixDeploymentList.Items {
		radixDeployments = append(radixDeployments, &ApplicationDeployment{Name: rd.Name, AppName: rd.Spec.AppName, Environment: rd.Spec.Environment, Created: rd.CreationTimestamp.Time})
	}

	return postFiltering(radixDeployments, latest), nil
}

// HandlePromoteEnvironment handler for PromoteEnvironment
func HandlePromoteEnvironment(client kubernetes.Interface, radixclient radixclient.Interface, appName string, promotionParameters PromotionParameters) (*ApplicationDeployment, error) {
	if strings.TrimSpace(appName) == "" {
		return nil, utils.ValidationError("Radix Promotion", "App name is required")
	}

	fromNs := getNameSpaceForApplicationEnvironment(appName, promotionParameters.FromEnvironment)
	toNs := getNameSpaceForApplicationEnvironment(appName, promotionParameters.ToEnvironment)

	log.Infof("Promoting %s from %s to %s", appName, promotionParameters.FromEnvironment, promotionParameters.ToEnvironment)

	var err error
	var radixDeployment *v1.RadixDeployment

	if strings.TrimSpace(promotionParameters.ImageTag) != "" {
		radixDeployment, err = radixclient.RadixV1().RadixDeployments(fromNs).Get(getDeploymentName(appName, promotionParameters.ImageTag), metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	} else {
		// Get latest deployment
	}

	// TODO: Merge with information from RA, by using common library

	radixDeployment.ResourceVersion = ""
	radixDeployment.Namespace = toNs
	radixDeployment.Spec.Environment = promotionParameters.ToEnvironment

	radixDeployment, err = radixclient.RadixV1().RadixDeployments(toNs).Create(radixDeployment)
	if err != nil {
		return nil, err
	}

	return &ApplicationDeployment{Name: radixDeployment.Name}, nil
}

func postFiltering(all []*ApplicationDeployment, latest bool) []*ApplicationDeployment {
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

func isLatest(theOne *ApplicationDeployment, all []*ApplicationDeployment) bool {
	for _, rd := range all {
		if rd.AppName == theOne.AppName &&
			rd.Environment == theOne.Environment &&
			rd.Name != theOne.Name &&
			rd.Created.After(theOne.Created) {
			return false
		}
	}

	return true
}

// TODO : Separate out into library functions
func getNameSpaceForApplicationEnvironment(appName, environment string) string {
	return fmt.Sprintf("%s-%s", appName, environment)
}

func getDeploymentName(appName, imageTag string) string {
	return fmt.Sprintf("%s-%s", appName, imageTag)
}
