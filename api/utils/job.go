package utils

import radixutils "github.com/equinor/radix-common/utils"

type Job interface {
	GetCreated() string
	GetStarted() string
	GetEnded() string
	GetStatus() string
}

// IsBefore Checks that job-j is before job-i
func IsBefore(j, i Job) bool {
	jCreated := radixutils.ParseTimestampNillable(j.GetCreated())
	if jCreated == nil {
		return false
	}

	iCreated := radixutils.ParseTimestampNillable(i.GetCreated())
	if iCreated == nil {
		return true
	}

	jStarted := radixutils.ParseTimestampNillable(j.GetStarted())
	iStarted := radixutils.ParseTimestampNillable(i.GetStarted())

	return (jCreated.Equal(*iCreated) && jStarted != nil && iStarted != nil && jStarted.Before(*iStarted)) ||
		jCreated.Before(*iCreated)
}
