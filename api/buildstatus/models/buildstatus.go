package models

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/marstr/guid"
)

// embed https://golang.org/pkg/embed/ - For embedding a single file, a variable of type []byte or string is often best
//go:embed badges/build-status.svg
var statusBadge string

const buildStatusFailing = "failing"
const buildStatusPassing = "passing"
const buildStatusStopped = "stopped"
const buildStatusPending = "pending"
const buildStatusRunning = "running"
const buildStatusUnknown = "unknown"

type Status interface {
	WriteSvg(condition v1.RadixJobCondition) (*[]byte, error)
}

func NewBuildStatus() Status {
	return &radixBuildStatus{
		Operation:   "build",
		ColorLeft:   "#aaa",
		ColorShadow: "#010101",
		ColorFont:   "#fff",
	}
}

type radixBuildStatus struct {
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

func (rbs *radixBuildStatus) WriteSvg(condition v1.RadixJobCondition) (*[]byte, error) {
	rbs.Status = translateCondition(condition)
	color := getColor(rbs.Status)
	rbs.ColorRight = color
	return getStatus(rbs)
}

func translateCondition(condition v1.RadixJobCondition) string {
	switch condition {
	case v1.JobSucceeded:
		return buildStatusPassing
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

func getStatus(status *radixBuildStatus) (*[]byte, error) {
	operationWidth := calculateWidth(9, status.Operation)
	statusWidth := calculateWidth(12, status.Status)
	status.Width = statusWidth + operationWidth
	status.Height = 30
	status.StatusOffset = operationWidth
	status.OperationTextId = guid.NewGUID().String()
	status.StatusTextId = guid.NewGUID().String()

	svgTemplate := template.New("status-badge.svg")
	svgTemplate.Parse(statusBadge)
	var buff bytes.Buffer
	err := svgTemplate.Execute(&buff, status)
	if err != nil {
		return nil, fmt.Errorf("Failed to create SVG template")
	}
	bytes := buff.Bytes()
	return &bytes, nil
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

func getColor(status string) string {
	switch status {
	case buildStatusPassing:
		return "#4c1"
	case buildStatusFailing:
		return "#e05d44"
	case buildStatusPending:
		return "#9f9f9f"
	case buildStatusStopped:
		return "#e05d44"
	case buildStatusUnknown:
		return "#9f9f9f"
	case buildStatusRunning:
		return "#33cccc"
	default:
		return "#9f9f9f"
	}
}
