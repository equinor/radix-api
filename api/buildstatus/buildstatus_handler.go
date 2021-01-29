package buildstatus

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"strings"

	"github.com/equinor/radix-api/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/marstr/guid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BuildStatusHandler struct {
	accounts models.Accounts
}

func Init(accounts models.Accounts) BuildStatusHandler {
	return BuildStatusHandler{accounts: accounts}
}

// GetBuildStatusForApplication Gets a list of build status for environments
func (handler BuildStatusHandler) GetBuildStatusForApplication(appName string) (*[]byte, error) {
	var output []byte

	// Get latest RJ
	serviceAccount := handler.getServiceAccount()
	namespace := fmt.Sprintf("%s-app", appName)

	// Get list of Jobs in the namespace
	rj, err := serviceAccount.RadixClient.RadixV1().RadixJobs(namespace).List(metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	mostRecentBuildJob := getLatestBuildJob(rj.Items)

	buildStep := getBuildStep(mostRecentBuildJob.Status.Steps)

	if buildStep.Condition == "Succeeded" {
		output = append(output, *writePassingSvg("build")...)
	} else if buildStep.Condition != "Succeeded" && mostRecentBuildJob.Status.Condition != "Succeeded" {
		output = append(output, *writeFailingSvg("build")...)
	} else {
		output = append(output, *writeUnknownSvg("build")...)
	}

	return &output, nil
}

func getBuildStep(steps []v1.RadixJobStep) v1.RadixJobStep {
	for _, step := range steps {
		if step.Name == "build-server" {
			return step
		}
	}

	return v1.RadixJobStep{}
}

func getLatestBuildJob(jobs []v1.RadixJob) v1.RadixJob {
	for i := len(jobs) - 1; i > 0; i-- {
		if len(jobs[i].Status.Steps) > 4 {
			return jobs[i]
		}
	}
	return v1.RadixJob{}
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
	svgTemplate, err := template.ParseFiles("/home/ole/go/src/github.com/equinor/radix-api/badges/build-status.svg")
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
