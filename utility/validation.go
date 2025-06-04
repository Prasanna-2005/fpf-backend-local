package utility

import (
	"errors"
	"regexp"
)

var (
	EmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

func ValidateEmail(email string) error {
	if !EmailRegex.MatchString(email) {
		return errors.New("invalid email address")
	}
	return nil
}
