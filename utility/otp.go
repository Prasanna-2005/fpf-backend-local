package utility

import (
	"os"
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
		OTPLength:    GetEnvAsInt("OTP_LENGTH", 6),
		OTPExpiry:    GetEnvAsDuration("OTP_EXPIRY", 100*time.Second),
	}
}
func GetEnvAsInt(key string, defaultValue int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	res, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		fmt.Printf("Error parsing %s as int: %v\n", key, err)
		return defaultValue
	}
	return int(res)
}
