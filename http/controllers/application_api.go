package controllers

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/statoil/radix-api-go/http/types"
	"github.com/statoil/radix-api-go/http/utils"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"regexp"
	"strings"
)

var repoPattern = regexp.MustCompile("https://github.com/(.*?)")

const sshURL = "git@github.com:"

func CreateApplication(radixclient radixclient.Interface, registration types.ApplicationRegistration) (*types.ApplicationRegistration, error) {
	deployKey, err := utils.GenerateDeployKey()
	if err != nil {
		return nil, err
	}

	radixRegistration, err := buildRRFromAppRegistration(registration, deployKey)
	if err != nil {
		return nil, err
	}

	_, err = radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(radixRegistration)
	if err != nil {
		return nil, err
	}

	registration.PublicKey = deployKey.PublicKey
	return &registration, nil
}

func DeleteApplication(appName string) error {
	logrus.Infof("Deleting app with name %s", appName)
	return nil
}

func buildRRFromAppRegistration(registration types.ApplicationRegistration, deployKey *utils.DeployKey) (*v1.RadixRegistration, error) {
	projectName, err := getProjectNameFromRepo(registration.Repository)
	if err != nil {
		return nil, err
	}

	cloneURL, err := getCloneURLFromRepo(registration.Repository)
	if err != nil {
		return nil, err
	}

	radixRegistration := &v1.RadixRegistration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixRegistration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: projectName,
		},
		Spec: v1.RadixRegistrationSpec{
			Repository:   registration.Repository,
			CloneURL:     cloneURL,
			SharedSecret: registration.SharedSecret,
			DeployKey:    deployKey.PrivateKey,
			AdGroups:     registration.AdGroups,
		},
	}
	return radixRegistration, nil
}

func getProjectNameFromRepo(repo string) (string, error) {
	b := repoPattern.MatchString(repo)
	if !b {
		return "", errors.New("Repo string does not match the expected pattern")
	}

	lastIndex := strings.LastIndex(repo, "/") + 1
	return repo[lastIndex:len(repo)], nil
}

func getCloneURLFromRepo(repo string) (string, error) {
	b := repoPattern.MatchString(repo)
	if !b {
		return "", errors.New("Repo string does not match the expected pattern")
	}

	cloneURL := repoPattern.ReplaceAllString(repo, sshURL)
	cloneURL += ".git"
	return cloneURL, nil
}
