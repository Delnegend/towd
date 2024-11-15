package utils

import (
	"time"
)

// Convert a time to a string in iCalendar format: YYYYMMDD or YYYYMMDDTHHMMSSZ
func Unix2Datetime(unixTime int64) string {
	t := time.Unix(unixTime, 0).UTC()

	hour, min, sec := t.Clock()
	if hour == 0 && min == 0 && sec == 0 {
		return t.Format("20060102Z")
	}
	return t.Format("20060102T150405Z")
}
