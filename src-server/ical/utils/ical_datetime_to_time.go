package utils

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	datePattern      = regexp.MustCompile(`^\d{4}\d{2}\d{2}$`)
	localTimePattern = regexp.MustCompile(`^\d{4}\d{2}\d{2}T\d{2}\d{2}\d{2}$`)
	UTCTimePattern   = regexp.MustCompile(`^\d{4}\d{2}\d{2}T\d{2}\d{2}\d{2}Z$`)
)

// Parsing fields containing date-time values. For example:
//   - DTSTART;TZID=Europe/Paris:20220101T000000
//   - END:20220101T000000Z
//
// `DTSTART`, `DTEND` will be ignored; If the datetime doesn't have a postfix "Z"
//   - if TZID is present, it will be used to parse the datetime
//   - otherwise, the datetime will be parsed in the local timezone
//
// else, the datetime will be parsed in UTC
func IcalDatetimeToTime(rawText string) (time.Time, error) {
	slice := strings.SplitN(rawText, ":", 2)
	if len(slice) != 2 {
		return time.Time{}, fmt.Errorf("must be splitable by ':', got %s", rawText)
	}

	firstPart := slice[0]
	timePart := slice[1]

	switch {
	case datePattern.MatchString(timePart):
		result, err := time.Parse("20060102", timePart)
		if err != nil {
			return time.Time{}, err
		}
		return result.UTC(), nil
	case localTimePattern.MatchString(timePart):
		var tzidString string
		if strings.Contains(firstPart, ";") {
			for _, prop := range strings.Split(firstPart, ";") {
				parts := strings.SplitN(prop, "=", 2)
				if len(parts) == 2 {
					if parts[0] == "TZID" {
						tzidString = parts[1]
					}
				}
			}
		}
		location, err := time.LoadLocation(tzidString)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid TZID: %s", err)
		}
		result, error := time.ParseInLocation("20060102T150405", timePart, location)
		if error != nil {
			return time.Time{}, error
		}
		return result.UTC(), nil
	case UTCTimePattern.MatchString(timePart):
		result, err := time.Parse("20060102T150405Z", timePart)
		if err != nil {
			return time.Time{}, err
		}
		return result, nil
	default:
		return time.Time{}, fmt.Errorf("invalid date-time format")
	}
}
