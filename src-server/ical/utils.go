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
		return nil, fmt.Errorf("must be splitable by ':', got %s", rawText)
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

// Create a new iCalendar-compatible CalAddress.
//
// The name and email must not contain any of the following characters:
// `:`, `;`, `,`, `\n`, `\r`, `\t`.
//
// Use empty string for email to create a CalAddress without email.
func NewCalAddr(name string, email string) (AttendeeCalAdrr, error) {
	prohibitChars := []string{":", ";", ",", "\n", "\r", "\t"}
	for _, c := range prohibitChars {
		if strings.Contains(name, c) || strings.Contains(email, c) {
			return "", fmt.Errorf("name and email must not contain %s", c)
		}
	}
	if name == "" || email == "" {
		return "", fmt.Errorf("name must not be empty")
	}
	return AttendeeCalAdrr(fmt.Sprintf("CN=%s:mailto:%s", name, email)), nil
}

func timeToStr(time_ time.Time) (string, error) {
	if time_.IsZero() {
		return "", fmt.Errorf("time is zero")
	}
	hour, min, sec := time_.Clock()
	if hour == 0 && min == 0 && sec == 0 {
		return time_.Format("20060102"), nil
	}
	return time_.Format("20060102T150405Z"), nil
}
