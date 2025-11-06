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
	"form_pf_backend/models"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/pquerna/otp/totp"
)

var logger = LoggerFunc() // Initialize logger

func SendOTPEmail(to, emailOTP string) error {
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
	cfg := config.SMTP()

	// Validate SMTP configuration
	if cfg.SMTPHost == "" || cfg.SMTPPort == "" || cfg.SMTPUser == "" || cfg.SMTPPassword == "" {
		logger.Log("CRITICAL", "SendOTPEmail: incomplete SMTP configuration")
		return nil
	}

	logger.Log("INFO", "SendOTPEmail: attempting to send OTP email to "+to)

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
		logger.Log("ERROR", "SendOTPEmail: failed to send email to "+to+" - "+err.Error())
		return err
	}

	logger.Log("INFO", "SendOTPEmail: successfully sent OTP email to "+to)
	return nil
}

func CreateQuantriumEmail(otp, name, expiry string) (string, error) {
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
	logger.Log("INFO", "CreateQuantriumEmail: reading email template file")
	templateFile, err := os.ReadFile("static/email_template.html")
	if err != nil {
		logger.Log("ERROR", "CreateQuantriumEmail: failed to read email template - "+err.Error())
		return "", err
	}

	template := string(templateFile)
	template = strings.ReplaceAll(template, "{{.Name}}", name)
	template = strings.ReplaceAll(template, "{{.OTP}}", otp)
	template = strings.ReplaceAll(template, "{{.Expiry}}", expiry)

	logger.Log("INFO", "CreateQuantriumEmail: successfully created email template for user "+name)
	return template, nil
}

func CheckOtp(totpCode, userId string) (bool, error) {
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
	logger.Log("INFO", "CheckOtp: validating TOTP for user "+userId)
	cfg := config.OTP()
	if cfg.SecretKey == "" {
		logger.Log("CRITICAL", "CheckOtp: missing OTP secret key in configuration")
		return false, nil
	}
	var opts totp.ValidateOpts
	opts.Period = uint(cfg.OTPExpiry.Seconds())
	opts.Skew = 1 // Allow for 1 second of skew
	opts.Digits = cfg.OTPLength

	uniqueSecret := cfg.SecretKey + userId
	encodedSecretKey := base32.StdEncoding.EncodeToString([]byte(uniqueSecret))
	fmt.Println(userId)
	fmt.Println(cfg.SecretKey)
	isValid, err := totp.ValidateCustom(totpCode, encodedSecretKey, time.Now(), opts)
	if err != nil {
		logger.Log("ERROR", "CheckOtp: TOTP validation failed for user: "+err.Error())
		return false, err
	}

	if isValid {
		logger.Log("INFO", "CheckOtp: TOTP validation successful for user ")
	} else {
		logger.Log("WARNING", "CheckOtp: TOTP validation failed for user - invalid code")
	}

	return isValid, nil
}

func GenTOTP(cfg *config.OTPconfig, userId string) (string, error) {
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
	logger.Log("INFO", "GenTOTP: generating TOTP for user ")

	if cfg.SecretKey == "" {
		logger.Log("CRITICAL", "GenTOTP: missing OTP secret key in configuration")
		return "", nil
	}

	var opts totp.ValidateOpts
	opts.Period = uint(cfg.OTPExpiry.Seconds())
	opts.Digits = cfg.OTPLength

	uniqueSecret := cfg.SecretKey + userId
	encodedSecretKey := base32.StdEncoding.EncodeToString([]byte(uniqueSecret))

	code, err := totp.GenerateCodeCustom(encodedSecretKey, time.Now(), opts)
	if err != nil {
		logger.Log("ERROR", "GenTOTP: failed to generate TOTP for user "+userId+" - "+err.Error())
		return "", err
	}

	logger.Log("INFO", "GenTOTP: successfully generated TOTP for user "+userId)
	return code, nil
}

func GenerateRefreshToken(userId string) (string, error) {

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
	logger.Log("INFO", "GenerateRefreshToken: generating refresh token for user")

	cfg := config.Token()
	if cfg.TokenSecret == "" {
		logger.Log("CRITICAL", "GenerateRefreshToken: missing token secret in configuration")
		return "", nil
	}

	user, err := db.GetUser(userId)
	if err != nil {
		logger.Log("ERROR", "GenerateRefreshToken: failed to get user  from database: "+err.Error())
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId":     userId,
		"type":       "refresh",
		"exp":        time.Now().Add(time.Duration(cfg.RefreshTokenExpiry)).Unix(), // 5 days
		"iat":        time.Now().Unix(),
		"email":      user.Email,
		"role":       user.AccessType,
		"jti":        user.Jti,
		"iss":        cfg.Issuer,
		"AccessType": user.AccessType,
		"cid":        user.ClientID,
	})

	signedToken, err := token.SignedString([]byte(cfg.TokenSecret))
	if err != nil {
		logger.Log("ERROR", "GenerateRefreshToken: failed to sign token for user "+userId+" - "+err.Error())
		return "", err
	}

	logger.Log("INFO", "GenerateRefreshToken: successfully generated refresh token for user "+userId)
	return signedToken, nil
}

func GenerateAccessToken(refreshToken, clientIP string) (string, error) {
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
	logger.Log("INFO", "GenerateAccessToken: parsing refresh token")

	cfg := config.Token()
	if cfg.TokenSecret == "" {
		logger.Log("CRITICAL", "GenerateAccessToken: missing token secret in configuration")
		return "", nil
	}
	refresh, err := DecryptToken(refreshToken, cfg.TokenSecret)
	if err != nil {
		logger.log("Error", "Decryption error:"+err.Error())
	}
	// Parse refresh token
	token, err := jwt.Parse(refresh, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.TokenSecret), nil
	})

	if err != nil {
		logger.Log("ERROR", "GenerateAccessToken: failed to parse refresh token - "+err.Error())
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		logger.Log("ERROR", "GenerateAccessToken: invalid refresh token claims")
		return "", err
	}

	// Check if it's a refresh token
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		logger.Log("ERROR", "GenerateAccessToken: provided token is not a refresh token")
		return "", err
	}

	// Extract email from claims
	email, ok := claims["email"].(string)
	if !ok {
		logger.Log("ERROR", "GenerateAccessToken: refresh token missing email claim")
		return "", err
	}

	// Extract userId from claims
	userId, ok := claims["userId"].(string)
	if !ok {
		logger.Log("ERROR", "GenerateAccessToken: refresh token missing userId claim")
		return "", err
	}

	accessType, ok := claims["AccessType"].(string)
	if !ok {
		logger.Log("ERROR", "Access type is missing for refresh token!")
	}

	jti, ok := claims["jti"]
	if !ok {
		logger.Log("ERROR", "JWT id is missing for refresh token!")
	}
	// Generate access token
	cid, ok := claims["cid"].(float64)
	if !ok {
		logger.Log("ERROR", "client id not present in refresh token")
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"AccessType": accessType,
		"type":       "access",
		"exp":        time.Now().Add(cfg.TokenExpiry).Unix(), // 4 hours
		"iat":        time.Now().Unix(),
		"nbf":        time.Now().Unix(),
		"ip":         clientIP,
		"email":      email,
		"userId":     userId,
		"jti":        jti,
		"cid":        cid,
	})

	signedToken, err := accessToken.SignedString([]byte(cfg.TokenSecret))
	if err != nil {
		logger.Log("ERROR", "GenerateAccessToken: failed to sign access token for user "+userId+" - "+err.Error())
		return "", err
	}

	logger.Log("INFO", "GenerateAccessToken: successfully generated access token for user "+userId)
	return signedToken, err

}

func VerifyAccessToken(token string) (models.User, error) {
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

	logger.Log("INFO", "VerifyAccessToken: verifying access token")

	cfg := config.Token()
	if cfg.TokenSecret == "" {
		logger.Log("CRITICAL", "VerifyAccessToken: missing token secret in configuration")
		return models.User{}, fmt.Errorf("missing token secret configuration")
	}

	decryptedToken, err := DecryptToken(token, cfg.TokenSecret)
	if err != nil {
		logger.Log("ERROR", "VerifyAccessToken: decryption failed - "+err.Error())
		return models.User{}, fmt.Errorf("token decryption failed: %w", err)
	}

	parsedToken, err := jwt.Parse(decryptedToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.TokenSecret), nil
	})

	if err != nil {
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorExpired != 0 {
				logger.Log("WARNING", "VerifyAccessToken: token has expired")
				return models.User{}, fmt.Errorf("token has expired")
			}
		}
		logger.Log("ERROR", "VerifyAccessToken: failed to parse token - "+err.Error())
		return models.User{}, fmt.Errorf("token parse failed: %w", err)
	}

	if !parsedToken.Valid {
		logger.Log("ERROR", "VerifyAccessToken: token is invalid")
		return models.User{}, fmt.Errorf("invalid token")
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		logger.Log("ERROR", "VerifyAccessToken: invalid token claims")
		return models.User{}, fmt.Errorf("invalid token claims format")
	}

	if err := claims.Valid(); err != nil {
		logger.Log("WARNING", "VerifyAccessToken: token validation failed (likely expired) - "+err.Error())
		return models.User{}, fmt.Errorf("token expired or invalid: %w", err)
	}

	if exp, ok := claims["exp"].(float64); ok {
		if int64(exp) < time.Now().Unix() {
			logger.Log("WARNING", "VerifyAccessToken: token has expired")
			return models.User{}, fmt.Errorf("token has expired")
		}
	} else {
		logger.Log("ERROR", "VerifyAccessToken: token missing expiration claim")
		return models.User{}, fmt.Errorf("token missing required 'exp' claim")
	}

	tokenType, ok := claims["type"].(string)
	if !ok {
		logger.Log("ERROR", "VerifyAccessToken: missing token type claim")
		return models.User{}, fmt.Errorf("missing token type claim")
	}
	if tokenType != "access" {
		logger.Log("ERROR", "VerifyAccessToken: provided token is not an access token")
		return models.User{}, fmt.Errorf("invalid token type: expected 'access', got '%s'", tokenType)
	}

	userId, ok := claims["userId"].(string)
	if !ok || userId == "" {
		logger.Log("ERROR", "VerifyAccessToken: token missing userId claim")
		return models.User{}, fmt.Errorf("missing or invalid userId claim")
	}

	accessType, ok := claims["AccessType"].(string)
	if !ok || accessType == "" {
		logger.Log("ERROR", "VerifyAccessToken: token missing AccessType claim")
		return models.User{}, fmt.Errorf("missing or invalid AccessType claim")
	}

	email, ok := claims["email"].(string)
	if !ok || email == "" {
		logger.Log("ERROR", "VerifyAccessToken: token missing email claim")
		return models.User{}, fmt.Errorf("missing or invalid email claim")
	}

	cid, ok := claims["cid"].(string)
	if !ok {
		logger.Log("ERROR", "VerifyAccessToken: token missing clientId claim")
		return models.User{}, fmt.Errorf("missing or invalid clientId claim")
	}

	jwtId, ok := claims["jti"].(float64)
	if !ok {
		logger.Log("ERROR", "VerifyAccessToken: token missing jti claim")
		return models.User{}, fmt.Errorf("missing or invalid jti claim")
	}

	usr, err := db.GetUser(userId)
	if err != nil {
		logger.Log("ERROR", "VerifyAccessToken: failed to fetch user from DB - "+err.Error())
		return models.User{}, fmt.Errorf("user validation failed: %w", err)
	}

	if jwtId != float64(usr.Jti) {
		logger.Log("WARNING", "VerifyAccessToken: JTI mismatch for user "+userId+" - token may be revoked")
		return models.User{}, fmt.Errorf("token has been revoked or is invalid")
	}

	user := models.User{
		ID:         userId,
		AccessType: accessType,
		Email:      email,
		ClientID:   cid,
	}

	logger.Log("INFO", "VerifyAccessToken: successfully verified access token for user "+userId)

	return user, nil
}
