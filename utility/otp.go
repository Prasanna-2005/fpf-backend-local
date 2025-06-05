package utility

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	// "html/template"
	"math/big"
	"net/smtp"
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
		OTPLength:    GetEnvAsInt("OTP_LENGTH", 6),
		OTPExpiry:    GetEnvAsDuration("OTP_EXPIRY", 100*time.Second),
	}
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
	hash := sha256.Sum256([]byte(code))
	return code, hex.EncodeToString(hash[:]), nil
}

// func sendOTPEmail(cfg OTPConfig, to, otp string) error {
// 	fmt.Printf("%s <%s>", cfg.FromName, cfg.FromEmail)

// 	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
// 	fmt.Print(auth)
// 	var msg bytes.Buffer

//		headers := map[string]string{
//			"From":         fmt.Sprintf("%s <%s>", cfg.FromName, cfg.FromEmail),
//			"To":           to,
//			"Subject":      "Your Quantrium Verification Code",
//			"MIME-Version": "1.0",
//			"Content-Type": "text/html; charset=UTF-8",
//		}
//		for k, v := range headers {
//			msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
//		}
//		tmpl := createQuantriumEmail(otp, cfg.OTPExpiry)
//		// if err != nil {
//		// 	return err
//		// }
//		msg.WriteString("\r\n" + tmpl)
//		fmt.Print(msg)
//		return smtp.SendMail(
//			fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort),
//			auth,
//			cfg.FromEmail,
//			[]string{to},
//			msg.Bytes(),
//		)
//	}
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

//	func createQuantriumEmail(otp string, expiry time.Duration) string {
//		// Read the email template from the file.
//		templateFile, err := os.ReadFile("static/email_template.html")
//		if err != nil {
//			return "" //"", fmt.Errorf("error reading email template: %w", err)
//		}
//		// Parse the template.
//		tmpl, err := template.New("email").Parse(string(templateFile))
//		if err != nil {
//			return "email template parsing failed" // "", fmt.Errorf("error parsing email template: %w", err)
//		}
//		// Execute the template with the given values.
//		var msg bytes.Buffer
//		err = tmpl.Execute(&msg, map[string]interface{}{
//			"Name":   "Vineeta minz",
//			"OTP":    otp,
//			"Expiry": expiry.Minutes(),
//		})
//		if err != nil {
//			return "mail did not go" //fmt.Errorf("error executing email template: %w", err)
//		}
//		// Return the resulting HTML as a string.
//		return msg.String() //, nil
//	}
func SendOTPEmail(cfg OTPConfig, to, otp string) error {
	fmt.Print(to)
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
	msg.WriteString(otp)
	// msg.WriteString("\r\n" + createQuantriumEmail(otp, cfg.OTPExpiry))
	// fmt.Print(msg.Bytes())

	return smtp.SendMail(
		fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort),
		auth,
		cfg.FromEmail,
		[]string{to},
		msg.Bytes(),
	)
}
func HashOTP(otp string) (string, error) {
	if otp == "" {
		return "", errors.New("otp cannot be empty")
	}

	// Create a new SHA-256 hash
	hash := sha256.Sum256([]byte(otp))

	// Encode the hash bytes to a hexadecimal string
	hashedOTP := hex.EncodeToString(hash[:])

	return hashedOTP, nil
}
