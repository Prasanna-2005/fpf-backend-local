package config

import (
	"os"
	"strconv"
	"time"
)

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

func LoadOTPConfig() OTPConfig {
	return OTPConfig{
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     os.Getenv("SMTP_PORT"),
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		FromEmail:    os.Getenv("FROM_EMAIL"),
		FromName:     os.Getenv("FROM_NAME"),
		OTPLength:    mustParseInt(os.Getenv("OTP_LENGTH")),
		OTPExpiry:    mustParseDuration(os.Getenv("OTP_EXPIRY")),
	}
}

func mustParseInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return i
}

func mustParseDuration(s string) time.Duration {
	d, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return time.Duration(d) * time.Minute
}
