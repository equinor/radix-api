package deployment

import (
	"fmt"
	"strings"
	"time"

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
		namespace = getNamespaceForApplicationEnvironment(appName, environment)
	}

	radixDeploymentList, err := radixclient.RadixV1().RadixDeployments(namespace).List(listOptions)

	if err != nil {
		return nil, err
	}

	radixDeployments := make([]*ApplicationDeployment, 0)
	for _, rd := range radixDeploymentList.Items {
		radixDeployments = append(radixDeployments, NewDeploymentBuilder().withRadixDeployment(&rd).BuildApplicationDeployment())
	}

	return postFiltering(radixDeployments, latest), nil
}

// HandlePromoteEnvironment handler for PromoteEnvironment
func HandlePromoteEnvironment(client kubernetes.Interface, radixclient radixclient.Interface, appName string, promotionParameters PromotionParameters) (*ApplicationDeployment, error) {
	if strings.TrimSpace(appName) == "" {
		return nil, utils.ValidationError("Radix Promotion", "App name is required")
	}

	fromNs := getNamespaceForApplicationEnvironment(appName, promotionParameters.FromEnvironment)
	toNs := getNamespaceForApplicationEnvironment(appName, promotionParameters.ToEnvironment)

	_, err := client.CoreV1().Namespaces().Get(fromNs, metav1.GetOptions{})
	if err != nil {
		return nil, utils.TypeMissingError("Non existing from environment", err)
	}

	_, err = client.CoreV1().Namespaces().Get(toNs, metav1.GetOptions{})
	if err != nil {
		return nil, utils.TypeMissingError("Non existing to environment", err)
	}

	log.Infof("Promoting %s from %s to %s", appName, promotionParameters.FromEnvironment, promotionParameters.ToEnvironment)
	var radixDeployment *v1.RadixDeployment

	if strings.TrimSpace(promotionParameters.ImageTag) != "" {
		radixDeployment, err = radixclient.RadixV1().RadixDeployments(fromNs).Get(getDeploymentName(appName, promotionParameters.ImageTag), metav1.GetOptions{})
		if err != nil {
			return nil, utils.TypeMissingError("Non existing image", err)
		}
	} else {
		// Get latest deployment
		deployments, err := HandleGetDeployments(radixclient, appName, promotionParameters.FromEnvironment, true)
		if err != nil {
			return nil, utils.TypeMissingError("No deployment was found", err)
		}

		if len(deployments) != 1 {
			return nil, utils.UnexpectedError("No latest deployment was found", err)
		}

		radixDeployment, err = radixclient.RadixV1().RadixDeployments(fromNs).Get(deployments[0].Name, metav1.GetOptions{})
		if err != nil {
			return nil, utils.TypeMissingError("Non existing image", err)
		}
	}

	radixDeployment.ResourceVersion = ""
	radixDeployment.Namespace = toNs
	radixDeployment.Spec.Environment = promotionParameters.ToEnvironment

	err = mergeWithRadixApplication(radixclient, radixDeployment, promotionParameters.ToEnvironment)
	if err != nil {
		return nil, utils.UnexpectedError("Uable to merge deployment with application", err)
	}

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

func mergeWithRadixApplication(radixclient radixclient.Interface, radixDeployment *v1.RadixDeployment, environment string) error {
	appName := radixDeployment.Spec.AppName
	_, err := radixclient.RadixV1().RadixApplications(getAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	if err != nil {
		return utils.UnexpectedError(fmt.Sprintf("Unable to get application for app %s", appName), err)
	}

	return nil
}

// TODO : Separate out into library functions
func getAppNamespace(appName string) string {
	return fmt.Sprintf("%s-app", appName)
}

func getNamespaceForApplicationEnvironment(appName, environment string) string {
	return fmt.Sprintf("%s-%s", appName, environment)
}

func getDeploymentName(appName, imageTag string) string {
	return fmt.Sprintf("%s-%s", appName, imageTag)
}

func getAppAndImagePairFromName(name string) (string, string) {
	pair := strings.Split(name, "-")
	return pair[0], pair[1]
}

// Builder Handles construction of RD
type Builder interface {
	withRadixDeployment(*v1.RadixDeployment) Builder
	withImageTag(string) Builder
	withAppName(string) Builder
	withEnvironment(string) Builder
	withCreated(time.Time) Builder
	BuildApplicationDeployment() *ApplicationDeployment
	BuildRD() *v1.RadixDeployment
}

type deploymentBuilder struct {
	imageTag    string
	appName     string
	environment string
	created     time.Time
}

func (db *deploymentBuilder) withRadixDeployment(radixDeployment *v1.RadixDeployment) Builder {
	_, imageTag := getAppAndImagePairFromName(radixDeployment.Name)
	db.withImageTag(imageTag)
	db.withAppName(radixDeployment.Spec.AppName)
	db.withEnvironment(radixDeployment.Spec.Environment)
	db.withCreated(radixDeployment.CreationTimestamp.Time)
	return db
}

func (db *deploymentBuilder) withImageTag(imageTag string) Builder {
	db.imageTag = imageTag
	return db
}

func (db *deploymentBuilder) withAppName(appName string) Builder {
	db.appName = appName
	return db
}

func (db *deploymentBuilder) withEnvironment(environment string) Builder {
	db.environment = environment
	return db
}

func (db *deploymentBuilder) withCreated(created time.Time) Builder {
	db.created = created
	return db
}

func (db *deploymentBuilder) BuildApplicationDeployment() *ApplicationDeployment {
	name := getDeploymentName(db.appName, db.imageTag)
	return &ApplicationDeployment{Name: name, AppName: db.appName, Environment: db.environment, Created: db.created}
}

func (db *deploymentBuilder) BuildRD() *v1.RadixDeployment {
	radixDeployment := &v1.RadixDeployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixDeployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      getDeploymentName(db.appName, db.imageTag),
			Namespace: getNamespaceForApplicationEnvironment(db.appName, db.environment),
			Labels: map[string]string{
				"radixApp": db.appName,
				"env":      db.environment,
			},
			CreationTimestamp: metav1.Time{Time: db.created},
		},
		Spec: v1.RadixDeploymentSpec{
			AppName:     db.appName,
			Environment: db.environment,
		},
	}
	return radixDeployment
}

// NewDeploymentBuilder Constructor for deployment builder
func NewDeploymentBuilder() Builder {
	return &deploymentBuilder{
		created: time.Now(),
	}
}
