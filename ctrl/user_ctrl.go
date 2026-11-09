/*
Package ctrl provides authentication controllers for user login and OTP verification.
*/
package ctrl

import (
	"errors"
	"fmt"
	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/middleware"
	"form_pf_backend/modules"
	"form_pf_backend/utility"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	ServerError  = "Internal Sever Error. Please reach out to your admin"
	PayloadError = "Invalid request payload"
	AuthError    = "User not found"
	InvalidOTP   = "Invalid OTP"
	LoginFailed  = "Login failed: "
	CookieError  = "Cookie error: "
)

var CookieCfg *config.CookieConfig
var logger = utility.LoggerFunc() // Initialize logger

func Login(c *gin.Context) {
	/*
		Login handles user authentication and initiates OTP generation.

		This function validates user credentials and sends an OTP for two-factor authentication.

		:param c: Gin context containing the HTTP request
		:type c: *gin.Context

		Request Body:
		  username (string): User's login username
		  password (string): User's password

		Response:
		  200: Login verified, OTP sent
		  400: Invalid request payload
		  401: Incorrect username or password
		  500: Internal server error
	*/
	logger.Log("INFO", "Login attempt initiated")

	var payload struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	// Bind and validate request payload
	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Log("ERROR", PayloadError+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	// Validate user credentials
	user, err := db.ValidateCredentials(payload.Username, payload.Password)
	if err != nil {
		if errors.Is(err, db.ErrConnectionLost) {
			logger.Log("WARNING", "SQL Db is down")
			c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
			return
		}
		logger.Log("INFO", "Invalid credentials: "+err.Error())
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	logger.Log("INFO", "Credentials validated successfully for user")

	// Generate the OTP. Mail delivery continues after the response is written.
	err = modules.OtpService(&user)
	if err != nil {
		logger.Log("ERROR", LoginFailed+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login verified, generating OTP",
		"user_id": user.ID,
	})
}

func ResendOTP(c *gin.Context) {
	/*
		ResendOTP generates and sends a new OTP to the user.

		:param c: Gin context containing the HTTP request
		:type c: *gin.Context

		Request Body:
		  user_id (string): User's unique identifier

		Response:
		  200: OTP resent after the email was accepted
		  400: Invalid request payload
		  502: The verification email could not be sent
	*/
	logger.Log("INFO", "OTP resend request initiated")

	var payload struct {
		UserId string `json:"user_id" binding:"required"`
	}

	// Bind and validate request payload
	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Log("ERROR", "OTP resend failed: invalid payload - "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	user, err := db.GetUser(payload.UserId)
	if err != nil {
		logger.Log("ERROR", "Failed to retrieve user: "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user ID"})
		return
	}

	// Wait for Gmail. Resend is the user's retry, so a failed send must be visible.
	err = modules.SendOtpAndWait(user)
	if err != nil {
		logger.Log("ERROR", LoginFailed+err.Error())
		c.JSON(http.StatusBadGateway, gin.H{"message": "The verification code could not be sent. Please try again."})
		return
	}

	logger.Log("INFO", "OTP resend successful for user")
	c.JSON(http.StatusOK, gin.H{
		"message": "OTP generated successful",
	})
}

func VerifyOTP(c *gin.Context) {
	/*
		VerifyOTP validates OTP and generates authentication tokens.

		This function verifies the provided OTP and sets access/refresh tokens as HTTP cookies upon successful validation.

		:param c: Gin context containing the HTTP request
		:type c: *gin.Context

		Request Body:
		  user_id (string): User's unique identifier
		  otp (string): One-time password to verify

		Response:
		  200: OTP verified, tokens set as cookies
		  400: Invalid request payload or user ID
		  401: Invalid OTP
		  500: Internal server error
	*/

	logger.Log("INFO", "OTP verification initiated")

	var payload struct {
		UserId string `json:"user_id" binding:"required"`
		Otp    string `json:"otp" binding:"required"`
	}
	tokenCfg := config.Token()
	// Bind and validate request payload
	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Log("ERROR", "OTP verification failed: invalid payload - "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	// Verify OTP
	isValid, err := utility.CheckOtp(payload.Otp, payload.UserId)
	if err != nil {
		logger.Log("ERROR", "OTP verification failed: "+err.Error())
		c.JSON(http.StatusUnauthorized, gin.H{"message": InvalidOTP})
		return
	}

	if !isValid {
		logger.Log("WARNING", "OTP verification failed: invalid OTP for user ")
		c.JSON(http.StatusUnauthorized, gin.H{"message": InvalidOTP})
		return
	}

	logger.Log("INFO", "OTP verified successfully for user")

	// Get user details
	user, err := db.GetUser(payload.UserId)
	if err != nil {
		logger.Log("ERROR", "Failed to retrieve user: "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user ID"})
		return
	}

	logger.Log("INFO", "Generating refresh token")

	// Generate refresh token
	refresh, err := utility.GenerateRefreshToken(payload.UserId)
	if err != nil {
		logger.Log("ERROR", "Failed to generate refresh token for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	logger.Log("INFO", "Hashing refresh token")

	// Hash refresh token
	hashedRefresh, err := utility.HashPassword(refresh, tokenCfg.TokenSecret)
	if err != nil {
		logger.Log("ERROR", "Failed to hash refresh token for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	logger.Log("INFO", "Generating access token")

	// Generate access token
	access, err := utility.GenerateAccessToken(hashedRefresh, c.ClientIP())
	if err != nil {
		logger.Log("ERROR", "Failed to generate access token for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	logger.Log("INFO", "Hashing access token")

	// Hash access token
	hashedAccess, err := utility.HashPassword(access, tokenCfg.TokenSecret)
	if err != nil {
		logger.Log("ERROR", "Failed to hash access token for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	logger.Log("INFO", "Updating user tokens and session")

	// Update tokens and session
	// if err := db.UpdateToken(user, hashedAccess, hashedRefresh); err != nil {
	// 	logger.Log("ERROR", "Failed to update tokens for user "+payload.UserId+": "+err.Error())
	// 	c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
	// 	return
	// }

	if err := db.UpdateSession(user); err != nil {
		logger.Log("ERROR", "Failed to update session for user "+err.Error())
		c.JSON(http.StatusAlreadyReported, gin.H{"message": ServerError})
		return
	}

	logger.Log("INFO", "Setting authentication cookies")

	// Get cookie configuration
	CookieCfg, err = config.Cookie()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		logger.Log("ERROR", CookieError+err.Error())
		return
	}

	// Calculate expiry times in seconds for cookie MaxAge
	accessTokenExpiry := int(tokenCfg.TokenExpiry.Seconds())
	refreshTokenExpiry := int(tokenCfg.RefreshTokenExpiry.Seconds())

	logger.Log("DEBUG", fmt.Sprintf("Cookie config - Domain: '%s', Path: '%s'",
		CookieCfg.AccessToken.Domain, CookieCfg.AccessToken.Path))

	fmt.Printf("Setting access cookie with domain: '%s'\n", CookieCfg.AccessToken.Domain)

	c.SetCookie(
		user.ID+"access",               // name
		hashedAccess,                   // value
		accessTokenExpiry,              // maxAge in seconds
		CookieCfg.AccessToken.Path,     // path from config
		CookieCfg.AccessToken.Domain,   // domain from config
		CookieCfg.AccessToken.Secure,   // secure from config
		CookieCfg.AccessToken.HTTPOnly, // httpOnly from config
	)
	c.SetSameSite(http.SameSiteLaxMode)
	// cookie := http.Cookie{
	// 	Name:     user.ID + "access",
	// 	Value:    hashedAccess,
	// 	Path:     "/",
	// 	MaxAge:   3600,
	// 	HttpOnly: true,
	// 	Secure:   true,
	// 	SameSite: http.SameSiteLaxMode,
	// }
	// http.SetCookie(c.Writer, &cookie)
	// Set refresh token cookie using configuration
	c.SetCookie(
		user.ID+"refresh",               // name
		hashedRefresh,                   // value
		refreshTokenExpiry,              // maxAge in seconds
		CookieCfg.RefreshToken.Path,     // path from config
		CookieCfg.RefreshToken.Domain,   // domain from config
		CookieCfg.RefreshToken.Secure,   // secure from config
		CookieCfg.RefreshToken.HTTPOnly, // httpOnly from config
	)
	c.SetSameSite(http.SameSiteLaxMode)
	logger.Log("INFO", "Authentication completed successfully for user")

	c.JSON(http.StatusOK, gin.H{
		"message": "OTP verification successful",
		"response": gin.H{
			"username":    user.UserName,
			"user_id":     user.ID,
			"role":        user.AccessType,
			"expiry_time": strconv.Itoa(int(tokenCfg.TokenExpiry.Hours())) + " Hours",
			"client_id":   user.ClientID,
		},
	})
}

func NewAccessToken(c *gin.Context) {
	var payload struct {
		UserId string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Log("ERROR", "New Access token generation error: "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	var hashedAccess string

	tokenCfg := config.Token()
	accessTokenExpiry := int(tokenCfg.TokenExpiry.Seconds())

	logger.Log("INFO", "Access token generation")
	refreshCookie, err := c.Cookie(payload.UserId + "refresh")
	fmt.Println(payload.UserId + "refresh")
	if err != nil {
		logger.Log("ERROR", CookieError+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	access, err := utility.GenerateAccessToken(refreshCookie, c.ClientIP())
	if err != nil {
		logger.Log("ERROR", "access token generation error: "+err.Error())
	}

	hashedAccess, err = utility.HashPassword(access, tokenCfg.TokenSecret)
	if err != nil {
		logger.Log("ERROR", "Failed to hash access token for user "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}
	CookieCfg, err = config.Cookie()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		logger.Log("ERROR", CookieError+err.Error())
		return
	}

	c.SetCookie(
		payload.UserId+"access",        // name
		hashedAccess,                   // value
		accessTokenExpiry,              // maxAge in seconds
		CookieCfg.AccessToken.Path,     // path from config
		CookieCfg.AccessToken.Domain,   // domain from config
		CookieCfg.AccessToken.Secure,   // secure from config
		CookieCfg.AccessToken.HTTPOnly, // httpOnly from config
	)
	c.SetSameSite(http.SameSiteLaxMode)
	logger.Log("INFO", "Authentication completed successfully for user")

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"expiry_time": strconv.Itoa(int(tokenCfg.TokenExpiry.Hours())) + " Hours",
		},
	})

}

func UpdatePasswd(c *gin.Context) {
	var payload struct {
		NewPswd string `json:"new_password" binding:"required"`
	}
	uid, err := middleware.ExtractUserID(c)
	if err != nil {
		logger.Log("ERROR", "Header doesn't have uid")
		return
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Log("ERROR", PayloadError+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}
	cost := config.BcryptCost()
	user, err := db.GetUser(uid)
	if err != nil {
		logger.Log("ERROR", "user not found"+err.Error())
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Incorrect UserId"})
		return
	}
	hashedPswd, err := bcrypt.GenerateFromPassword([]byte(payload.NewPswd), cost.Cost)
	if err != nil {
		logger.Log("Error", "passwd length error: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Reset Password again"})
		return
	}
	user.Password = string(hashedPswd)
	err = db.UpdatePasswd(user)
	if err != nil {
		logger.Log("ERROR", "Password update failed"+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password reset successful"})
}

func Logout(c *gin.Context) {
	var payload struct {
		UserId      string `json:"user_id" binding:"required"`
		Logout_time string `json:"logout_time" binding:"required"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Log("ERROR", "logout error: "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	// Logout in DB
	err := db.UpdateLogout(payload.UserId, payload.Logout_time)
	if err != nil {
		logger.Log("ERROR", "Db error: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	// Load cookie config for domain/path
	CookieCfg, err := config.Cookie()
	if err != nil {
		logger.Log("ERROR", "Cookie config error: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	// Clear access and refresh cookies
	accessName := payload.UserId + "access"
	refreshName := payload.UserId + "refresh"

	// MaxAge < 0 means delete cookie now
	c.SetCookie(accessName, "", -1, CookieCfg.AccessToken.Path, CookieCfg.AccessToken.Domain, CookieCfg.AccessToken.Secure, CookieCfg.AccessToken.HTTPOnly)
	c.SetCookie(refreshName, "", -1, CookieCfg.RefreshToken.Path, CookieCfg.RefreshToken.Domain, CookieCfg.RefreshToken.Secure, CookieCfg.RefreshToken.HTTPOnly)

	logger.Log("INFO", "User logged out and cookies cleared")
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}
