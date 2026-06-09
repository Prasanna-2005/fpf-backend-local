package utility

import (
	"errors"
	"form_pf_backend/models"
	"regexp"
	"strings"
	"unicode"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
var quarterConv = map[string]string{
	"Q1": "first",
	"Q2": "second",
	"Q3": "third",
	"Q4": "fourth",
}

// ValidateUser returns nil if both username and email are OK.
func ValidateUser(username, email string) error {
	// 1. username: only letters
	for _, r := range username {
		if !(r == '.' || unicode.IsLetter(r) || r == ' ') {
			return errors.New("username must contain alphabets(a-z, A-Z),blankspaces and period('.')")
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

func CleanSubmitReq(req *models.SubmitReq) {
	for i := range req.FileMappings {
		period := req.FileMappings[i].Period

		// Normalize: convert Q1-Q4 to ordinal words, else lowercase
		if converted, ok := quarterConv[period]; ok {
			period = converted
		} else {
			period = strings.ToLower(period)
		}

		req.FileMappings[i].Period = period
	}
}
