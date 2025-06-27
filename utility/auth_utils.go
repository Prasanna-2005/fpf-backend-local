/*
Package utility provides authentication and communication utilities including OTP generation,
JWT token management, and email services.
*/
package utility

import (
	"bytes"
	"encoding/base32"
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

// logs holds the global logger instance for utility operations
var logs = LoggerFunc() // Initialize logger

/*
SendOTPEmail sends an OTP email to the specified recipient using SMTP.

This function validates SMTP configuration, constructs an HTML email with headers,
and sends the OTP email to the recipient.

:param to: Email address of the recipient
:type to: string
:param emailOTP: HTML content of the OTP email
:type emailOTP: string

:returns: Error if email sending fails, nil on success
:rtype: error
*/
func SendOTPEmail(to, emailOTP string) error {
	cfg := config.SMTP()

	// Validate SMTP configuration
	if cfg.SMTPHost == "" || cfg.SMTPPort == "" || cfg.SMTPUser == "" || cfg.SMTPPassword == "" {
		logs.Log("CRITICAL", "SendOTPEmail: incomplete SMTP configuration")
		return nil
	}

	logs.Log("INFO", "SendOTPEmail: attempting to send OTP email to "+to)

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
		logs.Log("ERROR", "SendOTPEmail: failed to send email to "+to+" - "+err.Error())
		return nil
	}

	logs.Log("INFO", "SendOTPEmail: successfully sent OTP email to "+to)
	return nil
}

/*
CreateQuantriumEmail creates a personalized OTP email from HTML template.

This function reads an HTML template file and replaces placeholders with
user-specific data including name, OTP code, and expiry time.

:param otp: The OTP code to include in the email
:type otp: string
:param name: User's name for personalization
:type name: string
:param expiry: OTP expiry duration as string
:type expiry: string

:returns: Formatted HTML email content and error if template reading fails
:rtype: (string, error)
*/
func CreateQuantriumEmail(otp, name, expiry string) (string, error) {
	logs.Log("DEBUG", "CreateQuantriumEmail: reading email template file")
	templateFile, err := os.ReadFile("static/email_template.html")
	if err != nil {
		logs.Log("ERROR", "CreateQuantriumEmail: failed to read email template - "+err.Error())
		return "", err
	}

	template := string(templateFile)
	template = strings.ReplaceAll(template, "{{.Name}}", name)
	template = strings.ReplaceAll(template, "{{.OTP}}", otp)
	template = strings.ReplaceAll(template, "{{.Expiry}}", expiry)

	logs.Log("DEBUG", "CreateQuantriumEmail: successfully created email template for user "+name)
	return template, nil
}

/*
CheckOtp validates a TOTP code against the user's unique secret.

This function creates a user-specific secret by combining the base secret key
with the user ID, then validates the provided TOTP code.

:param totpCode: The TOTP code to validate
:type totpCode: string
:param userId: Unique identifier of the user
:type userId: string

:returns: Boolean indicating if OTP is valid and error if validation fails
:rtype: (bool, error)
*/
func CheckOtp(totpCode, userId string) (bool, error) {
	logs.Log("DEBUG", "CheckOtp: validating TOTP for user "+userId)
	cfg := config.OTP()
	if cfg.SecretKey == "" {
		logs.Log("CRITICAL", "CheckOtp: missing OTP secret key in configuration")
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
		logs.Log("ERROR", "CheckOtp: TOTP validation failed for user "+userId+" - "+err.Error())
		return false, nil
	}

	if isValid {
		logs.Log("INFO", "CheckOtp: TOTP validation successful for user "+userId)
	} else {
		logs.Log("WARNING", "CheckOtp: TOTP validation failed for user "+userId+" - invalid code")
	}

	return isValid, nil
}

/*
GenTOTP generates a TOTP code for the specified user.

This function creates a user-specific secret and generates a time-based
one-time password using the configured parameters.

:param cfg: OTP configuration containing secret key, expiry, and length
:type cfg: *config.OTPconfig
:param userId: Unique identifier of the user
:type userId: string

:returns: Generated TOTP code and error if generation fails
:rtype: (string, error)
*/
func GenTOTP(cfg *config.OTPconfig, userId string) (string, error) {
	logs.Log("DEBUG", "GenTOTP: generating TOTP for user "+userId)

	if cfg.SecretKey == "" {
		logs.Log("CRITICAL", "GenTOTP: missing OTP secret key in configuration")
		return "", nil
	}

	var opts totp.ValidateOpts
	opts.Period = uint(cfg.OTPExpiry.Seconds())
	opts.Digits = cfg.OTPLength

	uniqueSecret := cfg.SecretKey + userId
	encodedSecretKey := base32.StdEncoding.EncodeToString([]byte(uniqueSecret))

	code, err := totp.GenerateCodeCustom(encodedSecretKey, time.Now(), opts)
	if err != nil {
		logs.Log("ERROR", "GenTOTP: failed to generate TOTP for user "+userId+" - "+err.Error())
		return "", nil
	}

	logs.Log("DEBUG", "GenTOTP: successfully generated TOTP for user "+userId)
	return code, nil
}

/*
GenerateRefreshToken creates a JWT refresh token for the specified user.

This function generates a long-lived refresh token (5 days) containing
user information and refresh token type claim.

:param userId: Unique identifier of the user
:type userId: string

:returns: Signed JWT refresh token and error if generation fails
:rtype: (string, error)

Token Claims:
- userId: User's unique identifier
- type: Token type (refresh)
- exp: Expiration time (5 days)
- iat: Issued at time
- email: User's email address
*/
func GenerateRefreshToken(userId string) (string, error) {
	logs.Log("DEBUG", "GenerateRefreshToken: generating refresh token for user "+userId)

	cfg := config.Token()
	if cfg.TokenSecret == "" {
		logs.Log("CRITICAL", "GenerateRefreshToken: missing token secret in configuration")
		return "", nil
	}

	user, err := db.GetUser(userId)
	if err != nil {
		logs.Log("ERROR", "GenerateRefreshToken: failed to get user "+userId+" from database - "+err.Error())
		return "", nil
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": userId,
		"type":   "refresh",
		"exp":    time.Now().Add(time.Hour * 24 * 5).Unix(), // 5 days
		"iat":    time.Now().Unix(),
		"email":  user.Email,
	})

	signedToken, err := token.SignedString([]byte(cfg.TokenSecret))
	if err != nil {
		logs.Log("ERROR", "GenerateRefreshToken: failed to sign token for user "+userId+" - "+err.Error())
		return "", nil
	}

	logs.Log("INFO", "GenerateRefreshToken: successfully generated refresh token for user "+userId)
	return signedToken, nil
}

/*
GenerateAccessToken creates a JWT access token from a valid refresh token.

This function parses the refresh token, validates its claims, and generates
a short-lived access token (4 hours) with user permissions.

:param refreshToken: Valid JWT refresh token
:type refreshToken: string
:param accessType: User's access level or role
:type accessType: string

:returns: Signed JWT access token and error if generation fails
:rtype: (string, error)

Token Claims:
- AccessType: User's access level
- type: Token type (access)
- exp: Expiration time (4 hours)
- iat: Issued at time
- email: User's email address
- userId: User's unique identifier
*/
func GenerateAccessToken(refreshToken, accessType string) (string, error) {
	logs.Log("DEBUG", "GenerateAccessToken: parsing refresh token")

	cfg := config.Token()
	if cfg.TokenSecret == "" {
		logs.Log("CRITICAL", "GenerateAccessToken: missing token secret in configuration")
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
		logs.Log("ERROR", "GenerateAccessToken: failed to parse refresh token - "+err.Error())
		return "", nil
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		logs.Log("ERROR", "GenerateAccessToken: invalid refresh token claims")
		return "", nil
	}

	// Check if it's a refresh token
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		logs.Log("ERROR", "GenerateAccessToken: provided token is not a refresh token")
		return "", nil
	}

	// Extract email from claims
	email, ok := claims["email"].(string)
	if !ok {
		logs.Log("ERROR", "GenerateAccessToken: refresh token missing email claim")
		return "", nil
	}

	// Extract userId from claims
	userId, ok := claims["userId"].(string)
	if !ok {
		logs.Log("ERROR", "GenerateAccessToken: refresh token missing userId claim")
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
		logs.Log("ERROR", "GenerateAccessToken: failed to sign access token for user "+userId+" - "+err.Error())
		return "", nil
	}

	logs.Log("INFO", "GenerateAccessToken: successfully generated access token for user "+userId)
	return signedToken, nil
}

/*
VerifyAccessToken validates a JWT access token and extracts the user ID.

This function parses the access token, validates its signature and claims,
and returns the user ID if the token is valid.

:param token: JWT access token to verify
:type token: string

:returns: User ID from token claims and error if verification fails
:rtype: (string, error)

Validation Steps:
1. Parse token with secret key
2. Verify token signature and validity
3. Check token type is 'access'
4. Extract userId from claims
*/
func VerifyAccessToken(token string) (string, error) {
	logs.Log("DEBUG", "VerifyAccessToken: verifying access token")

	cfg := config.Token()
	if cfg.TokenSecret == "" {
		logs.Log("CRITICAL", "VerifyAccessToken: missing token secret in configuration")
		return "", nil
	}

	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.TokenSecret), nil
	})

	if err != nil {
		logs.Log("ERROR", "VerifyAccessToken: failed to parse token - "+err.Error())
		return "", nil
	}

	if !parsedToken.Valid {
		logs.Log("ERROR", "VerifyAccessToken: token is invalid")
		return "", nil
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		logs.Log("ERROR", "VerifyAccessToken: invalid token claims")
		return "", nil
	}

	// Check if it's an access token
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "access" {
		logs.Log("ERROR", "VerifyAccessToken: provided token is not an access token")
		return "", nil
	}

	// Get userId from claims
	userId, ok := claims["userId"].(string)
	if !ok {
		logs.Log("ERROR", "VerifyAccessToken: token missing userId claim")
		return "", nil
	}

	logs.Log("DEBUG", "VerifyAccessToken: successfully verified access token for user "+userId)
	return userId, nil
}
