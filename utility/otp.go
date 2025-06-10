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

func GetEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(val)
	if err != nil {
		fmt.Printf("Error parsing %s as duration: %v\n", key, err)
		return defaultValue
	}
	return duration
}

func GenerateSecureOTP(length int) (string, string, error) {
	if length < 6 || length > 8 {
		return "", "", fmt.Errorf("OTP length must be between 6 and 8 digits")
	}
	const digits = "0123456789"
	otp := make([]byte, length)
	for i := range otp {
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", "", fmt.Errorf("failed to generate OTP digit: %w", err)
		}
		otp[i] = digits[num.Int64()]
	}
	code := string(otp)

	// Hash the OTP using bcrypt
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("failed to hash OTP: %w", err)
	}
	hash := string(hashBytes)

	return code, hash, nil
}
