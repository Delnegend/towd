package ical

import (
	"fmt"
)

var (
	errEventNotRecursive        = "this event is not recurring"
	errWrongAlarmTriggerFormat  = "alarm trigger must be in the format of `-PTxxM` or `-PTxxH`"
	errWrongAlarmDurationFormat = "alarm duration must be in the format of `PTxxM` or `PTxxH`"
)

type slogError struct {
	Msg  string
	Args []interface{}
}

func errNestedBlock(blockName string, lineCount int, content string) *slogError {
	return &slogError{
		Msg:  fmt.Sprintf("nested %s block", blockName),
		Args: []interface{}{"line", lineCount, "content", content},
	}
}

func errUnexpectedEnd(lineCount int, content string) *slogError {
	return &slogError{
		Msg:  "unexpected END block",
		Args: []interface{}{"line", lineCount, "content", content},
	}
}
