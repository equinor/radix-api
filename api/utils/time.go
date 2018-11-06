package utils

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const timestampFormat = "2006-01-02T15:04:05-0700"

// ParseTimestamp Converts timestamp to time
func ParseTimestamp(timestamp string) (time.Time, error) {
	return time.Parse(timestampFormat, timestamp)
}

// FormatTimestamp Converts time to formatted timestamp
func FormatTimestamp(timestamp time.Time) string {
	return timestamp.Format(timestampFormat)
}

// FormatTime Converts kubernetes time to formatted timestamp
func FormatTime(time *metav1.Time) string {
	if time != nil {
		return FormatTimestamp(time.Time)
	}

	return ""
}
