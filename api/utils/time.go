package utils

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ParseTimestamp Converts timestamp to time
func ParseTimestamp(timestamp string) (time.Time, error) {
	return time.Parse(time.RFC3339, timestamp)
}

// FormatTimestamp Converts time to formatted timestamp
func FormatTimestamp(timestamp time.Time) string {
	emptyTime := time.Time{}

	if timestamp != emptyTime {
		return timestamp.Format(time.RFC3339)
	}

	return ""
}

// FormatTime Converts kubernetes time to formatted timestamp
func FormatTime(time *metav1.Time) string {
	if time != nil {
		return FormatTimestamp(time.Time)
	}

	return ""
}
