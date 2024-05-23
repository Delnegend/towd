package ical

import (
	"fmt"
	"towd/src-server/utils"
)

var (
	errIDNotInit        = "id not initialized"
	errSummaryNotSet    = "summary not set"
	errStartDateInvalid = "start date not set"
	errEndDateInvalid   = "end date not set"
	errOrganizerNotSet  = "organizer not set"

	errInvalidURL            = "invalid url"
	errStartDateAfterEndDate = "start date is after end date"
)

func parseSlogErr(msg string, lineCount int, content string) *utils.SlogError {
	return &utils.SlogError{
		Msg:   msg,
		Props: []interface{}{"line", lineCount, "content", content},
	}
}

func errNestedBlock(blockName string, lineCount int, content string) *utils.SlogError {
	return parseSlogErr(fmt.Sprintf("nested %s block", blockName), lineCount, content)
}

func errAlarmNotInEvent(lineCount int, content string) *utils.SlogError {
	return parseSlogErr("VALARM block not in VEVENT block", lineCount, content)
}

func errStandardNotInTimezone(lineCount int, content string) *utils.SlogError {
	return parseSlogErr("STANDARD block not in VTIMEZONE block", lineCount, content)
}

func errExpectBegin(lineCount int, content string) *utils.SlogError {
	return parseSlogErr("expecting BEGIN block", lineCount, content)
}

func errUnexpectedEnd(lineCount int, content string) *utils.SlogError {
	return parseSlogErr("unexpected END block", lineCount, content)
}

func errWrongDTFormat(lineCount int, content string) *utils.SlogError {
	return parseSlogErr("wrong DT field format", lineCount, content)
}
