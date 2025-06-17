package utility

import (
	"bytes"
	"errors"
	"fmt"
	"form_pf_backend/db"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

type SMTPconfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	FromEmail    string
	FromName     string
}
type OTPconfig struct {
	SecretKey string
	OTPExpiry uint
	OTPLength otp.Digits
}

func SetSMTP() SMTPconfig {
	return SMTPconfig{
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     os.Getenv("SMTP_PORT"),
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		FromEmail:    os.Getenv("FROM_EMAIL"),
		FromName:     os.Getenv("FROM_NAME"),
	}
}
func LoadOTPConfig() OTPconfig {
	return OTPconfig{
		SecretKey: os.Getenv("SECRET_KEY"),
		OTPExpiry: uint(GetEnvAsInt("OTP_EXPIRY", 100)),
		OTPLength: otp.Digits(GetEnvAsInt("OTP_LENGTH", 6)),
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

func SendOTPEmail(to, emailOTP string) error {
	cfg := SetSMTP()
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

func CreateQuantriumEmail(otp string, name string, expiry uint) (string, error) {
	// Read the email template from the file.
	templateFile, err := os.ReadFile("static/email_template.html")
	if err != nil {
		return "", err
	}
	// Parse the template.
	template := string(templateFile)
	template = strings.ReplaceAll(template, "{{.Name}}", name)
	template = strings.ReplaceAll(template, "{{.OTP}}", otp)
	template = strings.ReplaceAll(template, "{{.Expiry}}", strconv.FormatUint(uint64(expiry), 10))
	return template, nil
}

func CheckOtp(totpCode string) error {
	cfg := LoadOTPConfig()
	var opts totp.ValidateOpts
	opts.Period = cfg.OTPExpiry
	opts.Skew = 1 // Allow for 1 second of skew
	opts.Digits = cfg.OTPLength
	val, err := totp.ValidateCustom(totpCode, cfg.SecretKey, time.Now(), opts)
	// log.ErrorLevel(val)
	fmt.Println(val)
	if err != nil {
		return err
	}
	return nil
}

func GenerateTOTP(cfg OTPconfig) (string, error) {
	var opts totp.ValidateOpts  // GenerateCodeCustom actually expects ValidateOpts
	opts.Period = cfg.OTPExpiry // Match the period used in validation
	opts.Digits = cfg.OTPLength
	code, err := totp.GenerateCodeCustom(cfg.SecretKey, time.Now(), opts)
	if err != nil {
		return "", err
	}
	return code, nil
}
func GenerateToken(userId string) (string, error) {
	cfg := LoadOTPConfig() // To get secret key
	user, err := db.GetUser(userId)
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email":  user.Email,
		"userId": userId,
		"exp":    time.Now().Add(time.Hour * 2).Unix(),
	})

	return token.SignedString([]byte(cfg.SecretKey))
}

func VerifyToken(token string) (int64, error) {
	cfg := LoadOTPConfig()
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		_, ok := token.Method.(*jwt.SigningMethodHMAC)

		if !ok {
			return nil, errors.New("unexpected signing method")
		}

		return []byte(cfg.SecretKey), nil
	})

	if err != nil {
		fmt.Println("could not parse token")
		return 0, errors.New("could not parse token")
	}

	tokenIsValid := parsedToken.Valid

	if !tokenIsValid {
		return 0, errors.New("invalid token")
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)

	if !ok {
		return 0, errors.New("invalid token claims")
	}

	// email := claims["email"].(string)
	userId := int64(claims["userId"].(float64))
	return userId, nil
}
