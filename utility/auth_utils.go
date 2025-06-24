package utility

import (
	"bytes"
	"encoding/base32"
	"errors"
	"fmt"
	"form_pf_backend/config"
	"form_pf_backend/db"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/pquerna/otp/totp"
)

// SendOTPEmail sends an OTP email to the specified recipient
func SendOTPEmail(to, emailOTP string) error {
	cfg := config.SMTP()

	// Validate SMTP configuration
	if cfg.SMTPHost == "" || cfg.SMTPPort == "" || cfg.SMTPUser == "" || cfg.SMTPPassword == "" {
		Critical("SendOTPEmail: incomplete SMTP configuration")
		return nil
	}

	Infof("SendOTPEmail: attempting to send OTP email to %s", to)

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

	err := smtp.SendMail(
		fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort),
		auth,
		cfg.FromEmail,
		[]string{to},
		msg.Bytes(),
	)

	if err != nil {
		Errorf("SendOTPEmail: failed to send email to %s - %v", to, err)
		return nil
	}

	Infof("SendOTPEmail: successfully sent OTP email to %s", to)
	return nil
}

func CreateQuantriumEmail(otp, name, expiry string) (string, error) {
	Debug("CreateQuantriumEmail: reading email template file")
	templateFile, err := os.ReadFile("static/email_template.html")
	if err != nil {
		Errorf("CreateQuantriumEmail: failed to read email template - %v", err)
		return "", nil
	}

	template := string(templateFile)
	template = strings.ReplaceAll(template, "{{.Name}}", name)
	template = strings.ReplaceAll(template, "{{.OTP}}", otp)
	template = strings.ReplaceAll(template, "{{.Expiry}}", expiry)

	Debugf("CreateQuantriumEmail: successfully created email template for user %s", name)
	return template, nil
}

func CheckOtp(totpCode, userId string) (bool, error) {
	cfg := config.OTP()
	var opts totp.ValidateOpts
	opts.Period = uint(cfg.OTPExpiry.Seconds())
	opts.Skew = 1 // Allow for 1 second of skew
	opts.Digits = cfg.OTPLength
	uniqueSecret := cfg.SecretKey + userId

	// Encode the unique secret key to base32
	encodedSecretKey := base32.StdEncoding.EncodeToString([]byte(uniqueSecret))
	val, err := totp.ValidateCustom(totpCode, encodedSecretKey, time.Now(), opts)

	if err != nil {
		return val, err
	}
	return val, nil
}

func GenTOTP(cfg config.OTPconfig, userId string) (string, error) {
	var opts totp.ValidateOpts                  // GenerateCodeCustom actually expects ValidateOpts
	opts.Period = uint(cfg.OTPExpiry.Seconds()) // Match the period used in validation
	opts.Digits = cfg.OTPLength
	uniqueSecret := cfg.SecretKey + userId

	// Encode the unique secret key to base32
	encodedSecretKey := base32.StdEncoding.EncodeToString([]byte(uniqueSecret))
	code, err := totp.GenerateCodeCustom(encodedSecretKey, time.Now(), opts)
	fmt.Printf("code:%s\n", code)
	fmt.Println(err)
	if err != nil {
		return "", err
	}
	return code, nil
}

// Generate refresh token for user
func GenerateRefreshToken(userId string) (string, error) {
	cfg := config.LoadOTPConfig()
	user, err := db.GetUser(userId)
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email":  user.Email,
		"userId": userId,
		"type":   "refresh",
		"exp":    time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 days
	})

	return token.SignedString([]byte(cfg.SecretKey))
}

// Generate access token from refresh token
func GenerateAccessToken(refreshToken, AccessType string) (string, error) {
	cfg := config.LoadOTPConfig()

	// Parse refresh token
	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.SecretKey), nil
	})
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid refresh token")
	}

	// Check if it's a refresh token
	if claims["type"] != "refresh" {
		return "", fmt.Errorf("not a refresh token")
	}

	// Generate access token
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"AccessType": AccessType,
		"type":       "access",
		"exp":        time.Now().Add(time.Hour * 24).Unix(), // 24 minutes
		"email":      claims["email"],
	})

	return accessToken.SignedString([]byte(cfg.SecretKey))

}

func VerifyAccessToken(token string) (string, error) {
	cfg := config.OTP()
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		_, ok := token.Method.(*jwt.SigningMethodHMAC)

		if !ok {
			return nil, errors.New("unexpected signing method")
		}

		return []byte(cfg.SecretKey), nil
	})

	if err != nil {
		fmt.Println("could not parse token")
		return "", errors.New("could not parse token")
	}

	if !parsedToken.Valid {
		return "", errors.New("invalid token")
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid token claims")
	}

	// Check if it's an access token
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "access" {
		return "", errors.New("not an access token")
	}

	// Get userId as string and convert to int64
	userId, ok := claims["userId"].(string)
	if !ok {
		return "", errors.New("invalid userId type")
	}

	return userId, nil
}
