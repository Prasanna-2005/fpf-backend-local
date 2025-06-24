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
		return "", err
	}

	template := string(templateFile)
	template = strings.ReplaceAll(template, "{{.Name}}", name)
	template = strings.ReplaceAll(template, "{{.OTP}}", otp)
	template = strings.ReplaceAll(template, "{{.Expiry}}", expiry)

	Debugf("CreateQuantriumEmail: successfully created email template for user %s", name)
	return template, nil
}

// CheckOtp validates a TOTP code against the user's secret
func CheckOtp(totpCode, userId string) (bool, error) {
	Debugf("CheckOtp: validating TOTP for user %s", userId)
	cfg := config.OTP()
	if cfg.SecretKey == "" {
		Critical("CheckOtp: missing OTP secret key in configuration")
		return false, nil
	}
	var opts totp.ValidateOpts
	opts.Period = uint(cfg.OTPExpiry.Seconds())
	opts.Skew = 1 // Allow for 1 second of skew
	opts.Digits = cfg.OTPLength

	uniqueSecret := cfg.SecretKey + userId
	encodedSecretKey := base32.StdEncoding.EncodeToString([]byte(uniqueSecret))

	isValid, err := totp.ValidateCustom(totpCode, encodedSecretKey, time.Now(), opts)
	if err != nil {
		Errorf("CheckOtp: TOTP validation failed for user %s - %v", userId, err)
		return false, nil
	}

	if isValid {
		Infof("CheckOtp: TOTP validation successful for user %s", userId)
	} else {
		Warningf("CheckOtp: TOTP validation failed for user %s - invalid code", userId)
	}

	return isValid, nil
}

// GenTOTP generates a TOTP code for the specified user
func GenTOTP(userId string) (string, error) {
	Debugf("GenTOTP: generating TOTP for user %s", userId)

	cfg := config.OTP()
	if cfg.SecretKey == "" {
		Critical("GenTOTP: missing OTP secret key in configuration")
		return "", nil
	}

	var opts totp.ValidateOpts
	opts.Period = uint(cfg.OTPExpiry.Seconds())
	opts.Digits = cfg.OTPLength

	uniqueSecret := cfg.SecretKey + userId
	encodedSecretKey := base32.StdEncoding.EncodeToString([]byte(uniqueSecret))

	code, err := totp.GenerateCodeCustom(encodedSecretKey, time.Now(), opts)
	if err != nil {
		Errorf("GenTOTP: failed to generate TOTP for user %s - %v", userId, err)
		return "", nil
	}

	Debugf("GenTOTP: successfully generated TOTP for user %s", userId)
	return code, nil
}

// GenerateRefreshToken creates a refresh token for the specified user
func GenerateRefreshToken(userId string) (string, error) {
	Debugf("GenerateRefreshToken: generating refresh token for user %s", userId)

	cfg := config.Token()
	if cfg.TokenSecret == "" {
		Critical("GenerateRefreshToken: missing token secret in configuration")
		return "", nil
	}

	user, err := db.GetUser(userId)
	if err != nil {
		Errorf("GenerateRefreshToken: failed to get user %s from database - %v", userId, err)
		return "", nil
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email":  user.Email,
		"userId": userId,
		"type":   "refresh",
		"exp":    time.Now().Add(time.Hour * 24 * 5).Unix(), // 5 days
		"iat":    time.Now().Unix(),
	})

	signedToken, err := token.SignedString([]byte(cfg.TokenSecret))
	if err != nil {
		Errorf("GenerateRefreshToken: failed to sign token for user %s - %v", userId, err)
		return "", nil
	}

	Infof("GenerateRefreshToken: successfully generated refresh token for user %s", userId)
	return signedToken, nil
}

func GenerateAccessToken(refreshToken, accessType string) (string, error) {
	Debug("GenerateAccessToken: parsing refresh token")

	cfg := config.Token()
	if cfg.TokenSecret == "" {
		Critical("GenerateAccessToken: missing token secret in configuration")
		return "", nil
	}

	// Parse refresh token
	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.TokenSecret), nil
	})

	if err != nil {
		Errorf("GenerateAccessToken: failed to parse refresh token - %v", err)
		return "", nil
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		Error("GenerateAccessToken: invalid refresh token claims")
		return "", nil
	}

	// Check if it's a refresh token
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		Error("GenerateAccessToken: provided token is not a refresh token")
		return "", nil
	}

	// Extract email from claims
	email, ok := claims["email"].(string)
	if !ok {
		Error("GenerateAccessToken: refresh token missing email claim")
		return "", nil
	}

	// Extract userId from claims
	userId, ok := claims["userId"].(string)
	if !ok {
		Error("GenerateAccessToken: refresh token missing userId claim")
		return "", nil
	}

	// Generate access token
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"AccessType": accessType,
		"type":       "access",
		"exp":        time.Now().Add(time.Hour * 4).Unix(), // 4 hours
		"iat":        time.Now().Unix(),
		"email":      email,
		"userId":     userId,
	})

	signedToken, err := accessToken.SignedString([]byte(cfg.TokenSecret))
	if err != nil {
		Errorf("GenerateAccessToken: failed to sign access token for user %s - %v", userId, err)
		return "", nil
	}

	Infof("GenerateAccessToken: successfully generated access token for user %s", userId)
	return signedToken, nil
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
