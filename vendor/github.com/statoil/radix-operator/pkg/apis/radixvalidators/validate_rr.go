package radixvalidators

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CanRadixRegistrationBeInserted(client radixclient.Interface, radixRegistration *v1.RadixRegistration) (bool, error) {
	// cannot be used from admission control - returns the same radix reg that we try to validate
	errUniqueAppName := validateDoesNameAlreadyExist(client, radixRegistration.Name)

	isValid, err := CanRadixRegistrationBeUpdated(client, radixRegistration)
	if isValid && errUniqueAppName == nil {
		return true, nil
	}
	if isValid && errUniqueAppName != nil {
		return false, errUniqueAppName
	}
	if !isValid && errUniqueAppName == nil {
		return false, err
	}
	return false, concatErrors([]error{errUniqueAppName, err})
}

func CanRadixRegistrationBeUpdated(client radixclient.Interface, radixRegistration *v1.RadixRegistration) (bool, error) {
	errs := []error{}
	err := validateAppName(radixRegistration.Name)
	if err != nil {
		errs = append(errs, err)
	}
	err = validateGitSSHUrl(radixRegistration.Spec.CloneURL)
	if err != nil {
		errs = append(errs, err)
	}
	err = validateSSHKey(radixRegistration.Spec.DeployKey)
	if err != nil {
		errs = append(errs, err)
	}
	err = validateAdGroups(radixRegistration.Spec.AdGroups)
	if err != nil {
		errs = append(errs, err)
	}
	err = validateNoDuplicateGitRepo(client, radixRegistration.Name, radixRegistration.Spec.CloneURL)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) <= 0 {
		return true, nil
	}
	return false, concatErrors(errs)
}

func validateDoesNameAlreadyExist(client radixclient.Interface, appName string) error {
	rr, _ := client.RadixV1().RadixRegistrations("default").Get(appName, metav1.GetOptions{})
	if rr != nil && rr.Name != "" {
		return fmt.Errorf("App name must be unique in cluster - %s already exist", appName)
	}
	return nil
}

func validateAppName(appName string) error {
	if len(appName) > 253 {
		return fmt.Errorf("app name (%s) max length is 253", appName)
	}

	if appName == "" {
		return fmt.Errorf("app name is required")
	}

	re := regexp.MustCompile("^[a-z0-9.-]{0,}$")

	isValid := re.MatchString(appName)
	if isValid {
		return nil
	}
	return fmt.Errorf("app name %s can only consist of lower case alphanumeric characters, '.' and '-'", appName)
}

func validateAdGroups(groups []string) error {
	re := regexp.MustCompile("^([A-Za-z0-9]{8})-([A-Za-z0-9]{4})-([A-Za-z0-9]{4})-([A-Za-z0-9]{4})-([A-Za-z0-9]{12})$")

	if groups == nil || len(groups) <= 0 {
		return fmt.Errorf("AD group is required")
	}

	for _, group := range groups {
		isValid := re.MatchString(group)
		if !isValid {
			return fmt.Errorf("refer ad group %s by object id. It should be in uuid format %s", group, re.String())
		}
	}
	return nil
}

func validateGitSSHUrl(sshURL string) error {
	re := regexp.MustCompile("^(git@github.com:)([\\w-]+)/([\\w-]+)(.git)$")

	if sshURL == "" {
		return fmt.Errorf("ssh url is required")
	}

	isValid := re.MatchString(sshURL)

	if isValid {
		return nil
	}
	return fmt.Errorf("ssh url not valid %s. Must match regex %s", sshURL, re.String())
}

func validateNoDuplicateGitRepo(client radixclient.Interface, appName, sshURL string) error {
	if sshURL == "" {
		return nil
	}

	registrations, err := client.RadixV1().RadixRegistrations("default").List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, reg := range registrations.Items {
		if reg.Spec.CloneURL == sshURL && !strings.EqualFold(reg.Name, appName) {
			return fmt.Errorf("Repository is in use by %s", reg.Name)
		}
	}
	return nil
}

func validateSSHKey(deployKey string) error {
	// todo - how can this be validated..e.g. checked that the key isn't protected by a password
	return nil
}

func concatErrors(errs []error) error {
	var errstrings []string
	for _, err := range errs {
		errstrings = append(errstrings, err.Error())
	}

	return fmt.Errorf(strings.Join(errstrings, "\n"))

}
