package buildstatus

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"sort"
	"strings"

	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/marstr/guid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const BUILD_STATUS_SVG_RELATIVE_PATH = "../../badges/build-status.svg"

type BuildStatusHandler struct {
	accounts models.Accounts
}

func Init(accounts models.Accounts) BuildStatusHandler {
	return BuildStatusHandler{accounts: accounts}
}

// GetBuildStatusForApplication Gets a list of build status for environments
func (handler BuildStatusHandler) GetBuildStatusForApplication(appName, env string) (*[]byte, error) {
	var output []byte

	// Get latest RJ
	serviceAccount := handler.getServiceAccount()
	namespace := fmt.Sprintf("%s-app", appName)

	// Get list of Jobs in the namespace
	radixJobs, err := serviceAccount.RadixClient.RadixV1().RadixJobs(namespace).List(metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	latestBuildDeployJob, err := getLatestBuildJobToEnvironment(radixJobs.Items, env)

	if err != nil {
		return nil, utils.NotFoundError(err.Error())
	}

	buildStatus := latestBuildDeployJob.Status.Condition

	if buildStatus == "Succeeded" {
		output = append(output, *writePassingSvg("build")...)
	} else if buildStatus == "Failed" {
		output = append(output, *writeFailingSvg("build")...)
	} else {
		output = append(output, *writeUnknownSvg("build")...)
	}

	return &output, nil
}

func getLatestBuildJobToEnvironment(jobs []v1.RadixJob, env string) (v1.RadixJob, error) {
	// Filter out all BuildDeploy jobs
	allBuildDeployJobs := []v1.RadixJob{}
	for _, job := range jobs {
		if job.Spec.PipeLineType == v1.BuildDeploy {
			allBuildDeployJobs = append(allBuildDeployJobs, job)
		}
	}

	// Sort the slice by created date (In descending order)
	sort.Slice(allBuildDeployJobs[:], func(i, j int) bool {
		return allBuildDeployJobs[j].Status.Created.Before(allBuildDeployJobs[i].Status.Created)
	})

	// Get status of the last job to requested environment
	for _, buildDeployJob := range allBuildDeployJobs {
		for _, targetEnvironment := range buildDeployJob.Status.TargetEnvs {
			if targetEnvironment == env {
				return buildDeployJob, nil
			}
		}
	}

	return v1.RadixJob{}, fmt.Errorf("No build-deploy jobs were found in %s environment", env)

}

func (handler BuildStatusHandler) getServiceAccount() models.Account {
	return handler.accounts.ServiceAccount
}

type BuildStatus struct {
	Env          string
	Status       string
	ColorLeft    string
	ColorRight   string
	ColorShadow  string
	ColorFont    string
	Width        int
	Height       int
	StatusOffset int
	EnvTextId    string
	StatusTextId string
}

func writePassingSvg(env string) *[]byte {
	return getStatus(BuildStatus{Env: env, Status: "passing", ColorLeft: "#aaa", ColorRight: "#4c1", ColorShadow: "#010101", ColorFont: "#fff"})
}

func writeUnknownSvg(env string) *[]byte {
	return getStatus(BuildStatus{Env: env, Status: "unknown", ColorLeft: "#aaa", ColorRight: "#9f9f9f", ColorShadow: "#010101", ColorFont: "#fff"})
}

func writeFailingSvg(env string) *[]byte {
	return getStatus(BuildStatus{Env: env, Status: "failing", ColorLeft: "#aaa", ColorRight: "#e05d44", ColorShadow: "#010101", ColorFont: "#fff"})
}

func getStatus(status BuildStatus) *[]byte {
	envWidth := calculateWidth(9, status.Env)
	statusWidth := calculateWidth(12, status.Status)
	status.Width = statusWidth + envWidth
	status.Height = 30
	status.StatusOffset = envWidth
	status.EnvTextId = guid.NewGUID().String()
	status.StatusTextId = guid.NewGUID().String()
	svgTemplate, err := template.ParseFiles(BUILD_STATUS_SVG_RELATIVE_PATH)
	if err != nil {
		log.Fatal(err)
	}
	var buff bytes.Buffer
	err = svgTemplate.Execute(&buff, status)
	if err != nil {
		log.Fatal(err)
	}
	bytes := buff.Bytes()
	return &bytes
}

func calculateWidth(charWidth float32, value string) int {
	var width float32 = 0.0
	narrowCharWidth := charWidth * 0.55
	for _, ch := range value {
		if strings.Contains("tfrijl1", string(ch)) {
			width += narrowCharWidth
		} else {
			width += charWidth
		}
	}
	return int(width + 5)
}
