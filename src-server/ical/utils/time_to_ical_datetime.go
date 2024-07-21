package utils

import (
	"fmt"
	"time"
)

// Convert a time to a string in iCalendar format: YYYYMMDD or YYYYMMDDTHHMMSSZ
func TimeToIcalDatetime(time_ time.Time) (string, error) {
	if time_.IsZero() {
		return "", fmt.Errorf("time is zero")
	}
	hour, min, sec := time_.Clock()
	if hour == 0 && min == 0 && sec == 0 {
		return time_.Format("20060102"), nil
	}
	return time_.Format("20060102T150405Z"), nil
}
