package utility

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"math/big"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
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

func SendOTPEmail(cfg OTPConfig, to, emailOTP string) error {
	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	headers := map[string]string{
		"From":         fmt.Sprintf("%s <%s>", cfg.FromName, cfg.FromEmail),
		"To":           to,
		"Subject":      "Your Quantrium Verification Code",
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=UTF-8",
	}

	var msg bytes.Buffer

	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	msg.WriteString("\r\n" + emailOTP)

	return smtp.SendMail(
		fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort),
		auth,
		cfg.FromEmail,
		[]string{to},
		msg.Bytes(),
	)
}

func CreateQuantriumEmail(otp string, name string, expiry time.Duration) (string, error) {
	// Read the email template from the file.
	templateFile, err := os.ReadFile("static/email_template.html")
	if err != nil {
		return "", err
	}
	// Parse the template.
	template := string(templateFile)
	template = strings.ReplaceAll(template, "{{.Name}}", name)
	template = strings.ReplaceAll(template, "{{.OTP}}", otp)
	template = strings.ReplaceAll(template, "{{.Expiry}}", expiry.String())
	return template, nil
}

func OtpService(user *models.User) error {
	cfg := LoadOTPConfig()
	code, hash, err := GenerateSecureOTP(cfg.OTPLength)
	if err != nil {
		// c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error"})
		return err
	}
	err = db.OtpUpdate(hash, user)
	if err != nil {
		return err
	}
	emailOTP, err := CreateQuantriumEmail(code, user.UserName, cfg.OTPExpiry)
	err = SendOTPEmail(cfg, user.Email, emailOTP)
	if err != nil {
		// c.JSON(http.StatusFailedDependency, gin.H{"message": "SMTP client or User email doesn't exist"})
		return err
	}
	return nil
}

func CheckOtp(hashedOTPBytes, plainOTPBytes []byte) error {
	err := bcrypt.CompareHashAndPassword(hashedOTPBytes, plainOTPBytes)
	return err
}
