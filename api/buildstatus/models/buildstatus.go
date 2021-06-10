package models

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"strings"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/marstr/guid"
)

// embed https://golang.org/pkg/embed/ - For embedding a single file, a variable of type []byte or string is often best
//go:embed badges/build-status.svg
var defaultBadgeTemplate string

const buildStatusFailing = "failing"
const buildStatusSuccess = "success"
const buildStatusStopped = "stopped"
const buildStatusPending = "pending"
const buildStatusRunning = "running"
const buildStatusUnknown = "unknown"

type PiplineBadgeBuilder interface {
	BuildBadge(condition v1.RadixJobCondition, pipeline v1.RadixPipelineType) ([]byte, error)
}

func NewPiplineBadgeBuilder() PiplineBadgeBuilder {
	return &PipelineBadgeBuilder{
		// ColorLeft:     "#aaa",
		// ColorShadow:   "#010101",
		// ColorFont:     "#fff",
		BadgeTemplate: defaultBadgeTemplate,
	}
}

type pipelineBadgeData struct {
	Operation       string
	Status          string
	ColorLeft       string
	ColorRight      string
	ColorShadow     string
	ColorFont       string
	Width           int
	Height          int
	StatusOffset    int
	OperationTextId string
	StatusTextId    string
}

type PipelineBadgeBuilder struct {
	BadgeTemplate string
}

func (rbs *PipelineBadgeBuilder) BuildBadge(condition v1.RadixJobCondition, pipeline v1.RadixPipelineType) ([]byte, error) {
	return rbs.buildBadge(condition, pipeline)
}

func (rbs *PipelineBadgeBuilder) buildBadge(condition v1.RadixJobCondition, pipeline v1.RadixPipelineType) ([]byte, error) {
	operation := translatePipeline(pipeline)
	status := translateCondition(condition)
	color := getColor(condition)
	operationWidth := calculateWidth(10, operation)
	statusWidth := calculateWidth(10, status) + 24

	badgeData := pipelineBadgeData{
		Operation:       operation,
		Status:          status,
		ColorRight:      color,
		ColorLeft:       "#aaa",
		ColorShadow:     "#010101",
		ColorFont:       "#fff",
		Width:           statusWidth + operationWidth,
		Height:          30,
		StatusOffset:    operationWidth,
		OperationTextId: guid.NewGUID().String(),
		StatusTextId:    guid.NewGUID().String(),
	}

	svgTemplate := template.New("status-badge.svg")
	svgTemplate.Parse(rbs.BadgeTemplate)
	var buff bytes.Buffer
	err := svgTemplate.Execute(&buff, &badgeData)
	if err != nil {
		return nil, fmt.Errorf("Failed to create SVG template")
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
		return "#4c1"
	case v1.JobFailed:
		return "#e05d44"
	case v1.JobStopped:
		return "#e05d44"
	case v1.JobRunning:
		return "#33cccc"
	default:
		return "#9f9f9f"
	}
}
