/*
Package ctrl provides authentication controllers for user login and OTP verification.
*/
package ctrl

import (
	"net/http"
	"strconv"

	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/modules"
	"form_pf_backend/utility"

	"github.com/gin-gonic/gin"
)

const (
	ServerError  = "Internal server error"
	PayloadError = "Invalid request payload"
	AuthError    = "User not found"
	InvalidOTP   = "Invalid OTP"
	LoginFailed  = "Login failed: "
)

var cookieCfg *config.CookieConfig
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
		logger.Log("ERROR", LoginFailed+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	// Validate user credentials
	user, err := db.ValidateCredentials(payload.Username, payload.Password)
	if err != nil {
		if err.Error() == "record not found" {
			logger.Log("WARNING", LoginFailed+err.Error())
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Incorrect Username or Password"})
		} else {
			logger.Log("ERROR", LoginFailed+err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		}
		return
	}

	logger.Log("INFO", "Credentials validated successfully for user")

	// Generate and send OTP
	err = modules.OtpService(user.ID)
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
		  200: OTP resent successfully
		  400: Invalid request payload
		  500: Internal server error
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

	// Generate and send new OTP
	err := modules.OtpService(payload.UserId)
	if err != nil {
		logger.Log("ERROR", LoginFailed+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	logger.Log("INFO", "OTP resend successful for user")
	c.JSON(http.StatusOK, gin.H{
		"message": "OTP generated successful",
	})
}

// func VerifyOTP(c *gin.Context) {
// 	/*
// 		VerifyOTP validates OTP and generates authentication tokens.

// 		This function verifies the provided OTP and returns access/refresh tokens upon successful validation.

// 		:param c: Gin context containing the HTTP request
// 		:type c: *gin.Context

// 		Request Body:
// 		  user_id (string): User's unique identifier
// 		  otp (string): One-time password to verify

// 		Response:
// 		  200: OTP verified, tokens generated
// 		  400: Invalid request payload or user ID
// 		  401: Invalid OTP
// 		  500: Internal server error
// 	*/

// 	logger.Log("INFO", "OTP verification initiated")

// 	var payload struct {
// 		UserId string `json:"user_id" binding:"required"`
// 		Otp    string `json:"otp" binding:"required"`
// 	}

// 	// Bind and validate request payload
// 	if err := c.ShouldBindJSON(&payload); err != nil {
// 		logger.Log("ERROR", "OTP verification failed: invalid payload - "+err.Error())
// 		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
// 		return
// 	}

// 	// Verify OTP
// 	isValid, err := utility.CheckOtp(payload.Otp, payload.UserId)
// 	if err != nil {
// 		logger.Log("ERROR", "OTP verification failed: "+err.Error())
// 		c.JSON(http.StatusUnauthorized, gin.H{"message": InvalidOTP})
// 		return
// 	}

// 	if !isValid {
// 		logger.Log("WARNING", "OTP verification failed: invalid OTP for user ")
// 		c.JSON(http.StatusUnauthorized, gin.H{"message": InvalidOTP})
// 		return
// 	}

// 	logger.Log("INFO", "OTP verified successfully for user")

// 	// Get user details
// 	user, err := db.GetUser(payload.UserId)
// 	if err != nil {
// 		logger.Log("ERROR", "Failed to retrieve user: "+err.Error())
// 		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user ID"})
// 		return
// 	}

// 	logger.Log("INFO", "Generating refresh token")

// 	// Generate refresh token
// 	refresh, err := utility.GenerateRefreshToken(payload.UserId)
// 	if err != nil {
// 		logger.Log("ERROR", "Failed to generate refresh token for user "+payload.UserId+": "+err.Error())
// 		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
// 		return
// 	}

// 	logger.Log("INFO", "Generating access token")

// 	// Generate access token
// 	access, err := utility.GenerateAccessToken(refresh, user.AccessType)
// 	if err != nil {
// 		logger.Log("ERROR", "Failed to generate access token for user "+payload.UserId+": "+err.Error())
// 		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
// 		return
// 	}

// 	logger.Log("INFO", "Hashing access token")
// 	tokenCfg := config.Token()
// 	// Hash access token
// 	hashedAccess, err := utility.HashPassword(access, tokenCfg.TokenSecret)
// 	if err != nil {
// 		logger.Log("ERROR", "Failed to hash access token for user "+payload.UserId+": "+err.Error())
// 		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
// 		return
// 	}

// 	logger.Log("INFO", "Hashing refresh token")

// 	// Hash refresh token
// 	hashedRefresh, err := utility.HashPassword(refresh, tokenCfg.TokenSecret)
// 	if err != nil {
// 		logger.Log("ERROR", "Failed to hash refresh token for user "+payload.UserId+": "+err.Error())
// 		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
// 		return
// 	}

// 	logger.Log("INFO", "Updating user tokens and session")

// 	// Update tokens and session
// 	if err := db.UpdateToken(user, hashedAccess, hashedRefresh); err != nil {
// 		logger.Log("ERROR", "Failed to update tokens for user "+payload.UserId+": "+err.Error())
// 		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
// 		return
// 	}

// 	if err := db.UpdateSession(user); err != nil {
// 		logger.Log("ERROR", "Failed to update session for user "+payload.UserId+": "+err.Error())
// 		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
// 		return
// 	}

// 	logger.Log("INFO", "Authentication completed successfully for user")

// 	c.JSON(http.StatusOK, gin.H{
// 		"message": "OTP verification successful",
// 		"response": gin.H{
// 			"username":      user.UserName,
// 			"user_id":       user.ID,
// 			"role":          user.AccessType,
// 			"refresh_token": hashedRefresh,
// 			"access_token":  hashedAccess,
// 			"expiry_time":   strconv.Itoa(int(tokenCfg.TokenExpiry.Hours())) + " Hours",
// 		},
// 	})
// }

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

	logger.Log("INFO", "Generating access token")

	// Generate access token
	access, err := utility.GenerateAccessToken(refresh, user.AccessType)
	if err != nil {
		logger.Log("ERROR", "Failed to generate access token for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	logger.Log("INFO", "Hashing access token")
	tokenCfg := config.Token()
	// Hash access token
	hashedAccess, err := utility.HashPassword(access, tokenCfg.TokenSecret)
	if err != nil {
		logger.Log("ERROR", "Failed to hash access token for user "+payload.UserId+": "+err.Error())
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

	logger.Log("INFO", "Updating user tokens and session")

	// Update tokens and session
	// if err := db.UpdateToken(user, hashedAccess, hashedRefresh); err != nil {
	// 	logger.Log("ERROR", "Failed to update tokens for user "+payload.UserId+": "+err.Error())
	// 	c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
	// 	return
	// }

	if err := db.UpdateSession(user); err != nil {
		logger.Log("ERROR", "Failed to update session for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	logger.Log("INFO", "Setting authentication cookies")

	// Get cookie configuration
	cookieCfg, err = config.Cookie()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		logger.Log("ERROR", "Cookie error: "+err.Error())
		return
	}

	// Calculate expiry times in seconds for cookie MaxAge
	accessTokenExpiry := int(tokenCfg.TokenExpiry.Seconds())
	refreshTokenExpiry := int(tokenCfg.RefreshTokenExpiry.Seconds()) // Assuming you have this config

	// Set access token cookie using configuration
	c.SetCookie(
		access,                         // name
		hashedAccess,                   // value
		accessTokenExpiry,              // maxAge in seconds
		cookieCfg.AccessToken.Path,     // path from config
		cookieCfg.AccessToken.Domain,   // domain from config
		cookieCfg.AccessToken.Secure,   // secure from config
		cookieCfg.AccessToken.HTTPOnly, // httpOnly from config
	)

	// Set refresh token cookie using configuration
	c.SetCookie(
		refresh,                         // name
		hashedRefresh,                   // value
		refreshTokenExpiry,              // maxAge in seconds
		cookieCfg.RefreshToken.Path,     // path from config
		cookieCfg.RefreshToken.Domain,   // domain from config
		cookieCfg.RefreshToken.Secure,   // secure from config
		cookieCfg.RefreshToken.HTTPOnly, // httpOnly from config
	)

	logger.Log("INFO", "Authentication completed successfully for user")

	c.JSON(http.StatusOK, gin.H{
		"message": "OTP verification successful",
		"response": gin.H{
			"username":    user.UserName,
			"user_id":     user.ID,
			"role":        user.AccessType,
			"expiry_time": strconv.Itoa(int(tokenCfg.TokenExpiry.Hours())) + " Hours",
		},
	})
}
