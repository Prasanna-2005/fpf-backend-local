package utility

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// ValidateUser returns nil if both username and email are OK.
func ValidateUser(username, email string) error {
	// 1. username: only letters
	for _, r := range username {
		if !(r == '.' || unicode.IsLetter(r)) {
			return errors.New("username must contain alphabets(a-z, A-Z) and period('.')")
		}
	}
	if username == "" {
		return errors.New("username cannot be empty")
	}

	// 2. email: basic format check
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !emailRegex.MatchString(email) {
		return errors.New("invalid email format")
	}

	return nil
}
