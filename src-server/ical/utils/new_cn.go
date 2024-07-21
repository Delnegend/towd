package utils

import (
	"fmt"
	"strings"
)

// Create a new iCalendar-compatible common name.
//
// The name and email must not contain any of the following characters:
// `:`, `;`, `,`, `\n`, `\r`, `\t`.
//
// Use empty string for email to create a Common Name without an email address.
func NewCommonName(name string, email string) (string, error) {
	prohibitChars := []string{":", ";", ",", "\n", "\r", "\t"}
	for _, c := range prohibitChars {
		if strings.Contains(name, c) || strings.Contains(email, c) {
			return "", fmt.Errorf("name and email must not contain %s", c)
		}
	}
	if name == "" || email == "" {
		return "", fmt.Errorf("name must not be empty")
	}
	return fmt.Sprintf("CN=%s:mailto:%s", name, email), nil
}
