package utils

import (
	"time"
)

// for the interactive components like buttons, dropdowns, etc
type MsgComponentInfo struct {
	DateAdded time.Time
	Data      interface{}
}
