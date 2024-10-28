package models

import (
	"bytes"
	_ "embed"
	"errors"
	"html/template"
	"strings"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// embed https://golang.org/pkg/embed/ - For embedding a single file, a variable of type []byte or string is often best
//
//go:embed badges/build-status.svg
var defaultBadgeTemplate string

const (
	buildStatusFailing = "Failing"
	buildStatusSuccess = "Succeeded"
	buildStatusStopped = "Stopped"
	buildStatusPending = "Pending"
	buildStatusRunning = "Running"
	buildStatusUnknown = "Unknown"
)

const (
	pipelineStatusSuccessColor = "#34d058"
	pipelineStatusFailedColor  = "#e05d44"
	pipelineStatusStoppedColor = "#e05d44"
	pipelineStatusRunningColor = "#33cccc"
	pipelineStatusUnknownColor = "#9f9f9f"
)

type PipelineBadge interface {
	GetBadge(condition v1.RadixJobCondition, pipeline v1.RadixPipelineType) ([]byte, error)
}

func NewPipelineBadge() PipelineBadge {
	return &pipelineBadge{
		badgeTemplate: defaultBadgeTemplate,
	}
}

type pipelineBadgeData struct {
	Operation      string
	OperationWidth int
	Status         string
	StatusColor    string
	StatusWidth    int

	Width int
}

type pipelineBadge struct {
	badgeTemplate string
}

func (rbs *pipelineBadge) GetBadge(condition v1.RadixJobCondition, pipeline v1.RadixPipelineType) ([]byte, error) {
	return rbs.getBadge(condition, pipeline)
}

func (rbs *pipelineBadge) getBadge(condition v1.RadixJobCondition, pipeline v1.RadixPipelineType) ([]byte, error) {
	operation := translatePipeline(pipeline)
	status := translateCondition(condition)
	color := getColor(condition)
	operationWidth := calculateWidth(10, operation)
	statusWidth := calculateWidth(10, status)
	badgeData := pipelineBadgeData{
		Operation:      operation,
		OperationWidth: operationWidth, // Build-deploy:  111px
		Status:         status,
		StatusColor:    color,
		StatusWidth:    statusWidth, // Success: 75
		Width:          30 + operationWidth + statusWidth,
	}

	funcMap := template.FuncMap{
		// The name "inc" is what the function will be called in the template text.
		"sum": func(arg0 int, args ...int) int {
			x := arg0
			for _, arg := range args {
				x += arg
			}

			return x
		},
	}

	svgTemplate := template.New("status-badge.svg").Funcs(funcMap)
	_, err := svgTemplate.Parse(rbs.badgeTemplate)
	if err != nil {
		return nil, err
	}
	var buff bytes.Buffer
	err = svgTemplate.Execute(&buff, &badgeData)
	if err != nil {
		return nil, errors.New("failed to create SVG template")
	}
	bytes := buff.Bytes()
	return bytes, nil
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

func translateCondition(condition v1.RadixJobCondition) string {
	switch condition {
	case v1.JobSucceeded:
		return buildStatusSuccess
	case v1.JobFailed:
		return buildStatusFailing
	case v1.JobStopped:
		return buildStatusStopped
	case v1.JobWaiting, v1.JobQueued:
		return buildStatusPending
	case v1.JobRunning:
		return buildStatusRunning
	default:
		return buildStatusUnknown
	}
}

func translatePipeline(pipeline v1.RadixPipelineType) string {
	switch pipeline {
	case v1.BuildDeploy, v1.Build, v1.Deploy, v1.Promote:
		return string(pipeline)
	default:
		return ""
	}
}

func getColor(condition v1.RadixJobCondition) string {
	switch condition {
	case v1.JobSucceeded:
		return pipelineStatusSuccessColor
	case v1.JobFailed:
		return pipelineStatusFailedColor
	case v1.JobStopped:
		return pipelineStatusStoppedColor
	case v1.JobRunning:
		return pipelineStatusRunningColor
	default:
		return pipelineStatusUnknownColor
	}
}
