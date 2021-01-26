package buildstatus

import (
	"bytes"
	"github.com/marstr/guid"
	"html/template"
	"log"
	"strings"
)

type BuildStatusHandler struct {
}

// GetBuildStatusForApplication Gets a list of build status for environments
func (handler BuildStatusHandler) GetBuildStatusForApplication(appName string) (*[]byte, error) {
	var output []byte
	output = append(output, *writePassingSvg("qwertyuiopasdfghjklzxcvbnm123456789")...)
	output = append(output, " "...)
	output = append(output, *writeFailingSvg("qa")...)
	output = append(output, " "...)
	output = append(output, *writePassingSvg("ititititiiiiiijjjiiiijj111i")...)
	output = append(output, " "...)
	output = append(output, *writeUnknownSvg("production")...)
	return &output, nil
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
	svgTemplate, err := template.ParseFiles("build-status.svg")
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
