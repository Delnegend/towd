package ical

import (
	"fmt"
	"strings"
	"time"
)

// Parsing fields containing date-time values
//
// - `aaa;TZID=bbb:ccc`
// - `aaa:cccZ`
//
// `aaa` will be ignored; `bbb` is the time zone; `ccc` is the date-time value
func parseDate(rawText string) (*time.Time, error) {
	slice := strings.Split(rawText, ":")
	if len(slice) < 2 {
		return nil, fmt.Errorf("must be splitable by ':'")
	}

	// parse UTC time
	switch len(slice[1]) {
	case 16:
		res, err := time.Parse("20060102T150405Z", slice[1])
		if err != nil {
			return nil, err
		}
		return &res, nil
	case 8:
		res, err := time.Parse("20060102", slice[1])
		if err != nil {
			return nil, err
		}
		return &res, nil
	}

	properties := make(map[string]string)
	if strings.Contains(slice[0], ";") {
		for _, prop := range strings.Split(slice[0], ";") {
			if strings.Contains(prop, "=") {
				parts := strings.Split(prop, "=")
				properties[parts[0]] = parts[1]
			}
		}
	}

	// parse time zone
	var tzidString string
	var ok bool
	if tzidString, ok = properties["TZID"]; !ok {
		return nil, fmt.Errorf("time zone is missing")
	}
	location, err := time.LoadLocation(tzidString)
	if err != nil {
		return nil, fmt.Errorf("invalid TZID: %s", err)
	}

	// parse local time
	result, error := time.ParseInLocation("20060102T150405", slice[1], location)
	if error != nil {
		return nil, error
	}

	return &result, nil
}
