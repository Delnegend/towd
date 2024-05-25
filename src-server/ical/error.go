package ical

import (
	"fmt"
	"towd/src-server/utils"
)

var (
	errEventNotRecursive = "this event is not recurring"
)

func errNestedBlock(blockName string, lineCount int, content string) *utils.SlogError {
	return &utils.SlogError{
		Msg:  fmt.Sprintf("nested %s block", blockName),
		Args: []interface{}{"line", lineCount, "content", content},
	}
}

func errUnexpectedEnd(lineCount int, content string) *utils.SlogError {
	return &utils.SlogError{
		Msg:  "unexpected END block",
		Args: []interface{}{"line", lineCount, "content", content},
	}
}
