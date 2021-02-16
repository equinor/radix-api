package models

import (
	"bytes"
	"log"
	"strings"
	"text/template"

	"github.com/marstr/guid"
)

const BUILD_STATUS_SVG_RELATIVE_PATH = "./badges/build-status.svg"
const BUILD_STATUS_FAILING = "failing"
const BUILD_STATUS_PASSING = "passing"
const BUILD_STATUS_STOPPED = "stopped"
const BUILD_STATUS_PENDING = "pending"
const BUILD_STATUS_UNKNOWN = "unknown"

type Status interface {
	WriteSvg(status string) *[]byte
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

func (rbs *radixBuildStatus) WriteSvg(status string) *[]byte {
	color := getColor(status)
	rbs.ColorRight = color
	rbs.Status = status
	return getStatus(rbs)
}

func getStatus(status *radixBuildStatus) *[]byte {
	operationWidth := calculateWidth(9, status.Operation)
	statusWidth := calculateWidth(12, status.Status)
	status.Width = statusWidth + operationWidth
	status.Height = 30
	status.StatusOffset = operationWidth
	status.OperationTextId = guid.NewGUID().String()
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

func getColor(status string) string {
	switch status {
	case BUILD_STATUS_PASSING:
		return "#4c1"
	case BUILD_STATUS_FAILING:
		return "#e05d44"
	case BUILD_STATUS_PENDING:
		return "9f9f9f"
	case BUILD_STATUS_STOPPED:
		return "#e05d44"
	case BUILD_STATUS_UNKNOWN:
		return "9f9f9f"
	default:
		return "9f9f9f"
	}
}
