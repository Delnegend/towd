package utils

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// strips spaces, uppercase first letter, remove trailing period
func CleanupString(s string) string {
	s = strings.TrimSpace(s)
	s = cases.Title(language.English).String(s)
	s = strings.TrimSuffix(s, ".")
	return s
}
