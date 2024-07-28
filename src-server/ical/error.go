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

// Conver the custom error to string
func (e *CustomError) Error() string {
	var sb strings.Builder
	sb.WriteString(e.msg)
	if len(e.args) > 0 {
		sb.WriteString(" |")
		for key, value := range e.args {
			sb.WriteString(key + ": " + fmt.Sprintf("%v", value))
		}
	}
	return sb.String()
}

func (e *CustomError) GetMsg() string {
	return e.msg
}

func (e *CustomError) GetArgs() []any {
	temp := make([]any, 0)
	for k, v := range e.args {
		temp = append(temp, k, v)
	}
	return temp
}
