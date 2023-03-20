package logs

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils"
	commonErrors "github.com/equinor/radix-common/utils/errors"
)

// GetLogParams Gets parameters for a log output
func GetLogParams(r *http.Request) (time.Time, bool, *int64, error, bool) {
	sinceTime := r.FormValue("sinceTime")
	lines := r.FormValue("lines")
	file := r.FormValue("file")
	previous := r.FormValue("previous")
	var since time.Time
	var errs []error

	if !strings.EqualFold(strings.TrimSpace(sinceTime), "") {
		var err error
		since, err = utils.ParseTimestamp(sinceTime)
		if err != nil {
			errs = append(errs, err)
		}
	}
	var asFile = false
	if strings.TrimSpace(file) != "" {
		var err error
		asFile, err = strconv.ParseBool(file)
		if err != nil {
			errs = append(errs, err)
		}
	}
	var previousLog = false
	if strings.TrimSpace(file) != "" {
		var err error
		previousLog, err = strconv.ParseBool(previous)
		if err != nil {
			errs = append(errs, err)
		}
	}
	var logLines *int64
	if strings.TrimSpace(lines) != "" {
		var err error
		val, err := strconv.ParseInt(lines, 10, 64)
		if err != nil {
			errs = append(errs, err)
		}
		logLines = &val
	}
	return since, asFile, logLines, commonErrors.Concat(errs), previousLog
}
