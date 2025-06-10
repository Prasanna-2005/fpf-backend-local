package utility

import "time"

type OTPConfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	FromEmail    string
	FromName     string
	OTPLength    int
	OTPExpiry    time.Duration
}
