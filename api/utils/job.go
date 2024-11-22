package utils

import (
	"time"
)

type Job interface {
	GetCreated() time.Time
	GetStarted() *time.Time
	GetEnded() *time.Time
	GetStatus() string
}

// IsBefore Checks that job-j is before job-i
func IsBefore(j, i Job) bool {
	jCreated := j.GetCreated()
	iCreated := i.GetCreated()
	jStarted := j.GetStarted()
	iStarted := i.GetStarted()

	if jStarted != nil && (iStarted == nil || jStarted.Before(*iStarted)) {
		return true
	}

	if jCreated.Equal(iCreated) {
		return false
	}

	return iCreated.IsZero() || jCreated.Before(iCreated)
}
