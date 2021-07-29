package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func StepNotFoundError(stepName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("step %s not found", stepName))
}

func StepScanOutputNotDefined(stepName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("scan output for step %s not defined", stepName))
}

func StepScanOutputMissing(stepName string) error {
	return radixhttp.NotFoundError(fmt.Sprintf("scan output for step %s is missing", stepName))
}

func InvalidScanOutputConfig(stepName string) error {
	return &radixhttp.Error{
		Type:    radixhttp.Server,
		Message: fmt.Sprintf("scan output configuration for step %s is invalid", stepName),
	}
}

func MissingKeyInScanOutputData(stepName string) error {
	return &radixhttp.Error{
		Type:    radixhttp.Server,
		Message: fmt.Sprintf("scan output data for step %s not found", stepName),
	}
}

func InvalidScanOutputData(stepName string) error {
	return &radixhttp.Error{
		Type:    radixhttp.Server,
		Message: fmt.Sprintf("scan output data for step %s is invalid", stepName),
	}
}

func (jh JobHandler) GetPipelineJobStepScanOutput(appName, jobName, stepName string) ([]jobModels.Vulnerability, error) {
	namespace := crdUtils.GetAppNamespace(appName)
	job, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(namespace).Get(context.TODO(), jobName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	step := getStepFromRadixJob(job, stepName)
	if step == nil {
		return nil, StepNotFoundError(stepName)
	}

	if step.Output == nil || step.Output.Scan == nil {
		return nil, StepScanOutputNotDefined(stepName)
	}

	if step.Output.Scan.Status == v1.ScanMissing {
		return nil, StepScanOutputMissing(stepName)
	}

	scanOutputName, scanOutputKey := strings.TrimSpace(step.Output.Scan.VulnerabilityListConfigMap), strings.TrimSpace(step.Output.Scan.VulnerabilityListKey)
	if scanOutputName == "" || scanOutputKey == "" {
		return nil, InvalidScanOutputConfig(stepName)
	}

	scanOutputConfigMap, err := jh.userAccount.Client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), scanOutputName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	scanOutput, found := scanOutputConfigMap.Data[scanOutputKey]
	if !found {
		return nil, MissingKeyInScanOutputData(stepName)
	}

	var vulnerabilities []jobModels.Vulnerability
	if err := json.Unmarshal([]byte(scanOutput), &vulnerabilities); err != nil {
		return nil, InvalidScanOutputData(stepName)
	}

	return vulnerabilities, nil
}

func getStepFromRadixJob(job *v1.RadixJob, stepName string) *v1.RadixJobStep {
	for _, step := range job.Status.Steps {
		if step.Name == stepName {
			return &step
		}
	}

	return nil
}
