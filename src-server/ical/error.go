package ical

import (
	"fmt"
	"strings"
)

type CustomError struct {
	msg  string
	args map[string]any
}

// Create a new custom error
func NewCustomError(msg string, args map[string]any) *CustomError {
	if args == nil {
		args = make(map[string]any)
	}
	return &CustomError{
		msg:  msg,
		args: args,
	}
}

// Get the error message
func (e CustomError) Error() string {
	var sb strings.Builder
	sb.WriteString(e.msg)
	sb.WriteString(" | ")
	for key, value := range e.args {
		sb.WriteString(fmt.Sprintf(" %s: %v", key, value))
	}
	return sb.String()
}
