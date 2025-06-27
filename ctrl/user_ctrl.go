/*
Package ctrl provides authentication controllers for user login and OTP verification.
*/
package ctrl

import (
	"net/http"

	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/middleware"
	"form_pf_backend/models"
	"form_pf_backend/utility"

	"github.com/gin-gonic/gin"
)

const (
	ServerError  = "Internal server Error"
	PayloadError = "Invalid request payload"
	AuthError    = "User not found"
)

var logger = utility.LoggerFunc() // Initialize logger

/*
Login handles user authentication and initiates OTP generation.

This function validates user credentials and sends an OTP for two-factor authentication.

:param c: Gin context containing the HTTP request
:type c: *gin.Context

Request Body:
- username (string): User's login username
- password (string): User's password

Response:
- 200: Login verified, OTP sent
- 400: Invalid request payload
- 401: Incorrect username or password
- 500: Internal server error
*/
func Login(c *gin.Context) {
	logger.Log("INFO", "Login attempt initiated")

	var user models.User
	var payload struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	// Bind and validate request payload
	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Log("ERROR", "Login failed: invalid payload - "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	logger.Log("DEBUG", "Login attempt for username: "+payload.Username)

	// Validate user credentials
	user, err := db.ValidateCredentials(payload.Username, payload.Password)
	if err != nil {
		if err.Error() == "record not found" {
			logger.Log("WARNING", "Login failed: user not found for username: "+payload.Username)
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Incorrect Username or Password"})
		} else {
			logger.Log("ERROR", "Login failed: credential validation error - "+err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		}
		return
	}

	logger.Log("INFO", "Credentials validated successfully for user: "+user.ID)

	// Generate and send OTP
	err = middleware.OtpService(user.ID)
	if err != nil {
		logger.Log("ERROR", "Login failed: OTP service error for user "+user.ID+" - "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	logger.Log("INFO", "Login successful: OTP sent to user "+user.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Login verified, generating OTP",
		"user_id": user.ID,
	})
}

/*
ResendOTP generates and sends a new OTP to the user.

:param c: Gin context containing the HTTP request
:type c: *gin.Context

Request Body:
- user_id (string): User's unique identifier

Response:
- 200: OTP resent successfully
- 400: Invalid request payload
- 500: Internal server error
*/
func ResendOTP(c *gin.Context) {
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

	logger.Log("DEBUG", "OTP resend requested for user: "+payload.UserId)

	// Generate and send new OTP
	err := middleware.OtpService(payload.UserId)
	if err != nil {
		logger.Log("ERROR", "OTP resend failed for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	logger.Log("INFO", "OTP resend successful for user: "+payload.UserId)
	c.JSON(http.StatusOK, gin.H{
		"message": "OTP generated successful",
	})
}

/*
VerifyOTP validates OTP and generates authentication tokens.

This function verifies the provided OTP and returns access/refresh tokens upon successful validation.

:param c: Gin context containing the HTTP request
:type c: *gin.Context

Request Body:
- user_id (string): User's unique identifier
- otp (string): One-time password to verify

Response:
- 200: OTP verified, tokens generated
- 400: Invalid request payload or user ID
- 401: Invalid OTP
- 500: Internal server error
*/
func VerifyOTP(c *gin.Context) {
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

	logger.Log("DEBUG", "OTP verification attempt for user: "+payload.UserId)

	// Verify OTP
	isValid, err := utility.CheckOtp(payload.Otp, payload.UserId)
	if err != nil {
		logger.Log("ERROR", "OTP verification failed for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid OTP"})
		return
	}

	if !isValid {
		logger.Log("WARNING", "OTP verification failed: invalid OTP for user "+payload.UserId)
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid OTP"})
		return
	}

	logger.Log("INFO", "OTP verified successfully for user: "+payload.UserId)

	// Get user details
	user, err := db.GetUser(payload.UserId)
	if err != nil {
		logger.Log("ERROR", "Failed to retrieve user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user ID"})
		return
	}

	logger.Log("DEBUG", "Generating refresh token")

	// Generate refresh token
	refresh, err := utility.GenerateRefreshToken(payload.UserId)
	if err != nil {
		logger.Log("ERROR", "Failed to generate refresh token for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	logger.Log("DEBUG", "Generating access token")

	// Generate access token
	access, err := utility.GenerateAccessToken(refresh, user.AccessType)
	if err != nil {
		logger.Log("ERROR", "Failed to generate access token for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	logger.Log("DEBUG", "Hashing access token")
	tokenCfg := config.Token()
	// Hash access token
	hashedAccess, err := utility.HashPassword(access, tokenCfg.TokenSecret)
	if err != nil {
		logger.Log("ERROR", "Failed to hash access token for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	logger.Log("DEBUG", "Hashing refresh token")

	// Hash refresh token
	hashedRefresh, err := utility.HashPassword(refresh, tokenCfg.TokenSecret)
	if err != nil {
		logger.Log("ERROR", "Failed to hash refresh token for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	logger.Log("DEBUG", "Updating user tokens and session")

	// Update tokens and session
	if err := db.UpdateToken(user, hashedAccess, hashedRefresh); err != nil {
		logger.Log("ERROR", "Failed to update tokens for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	if err := db.UpdateSession(user); err != nil {
		logger.Log("ERROR", "Failed to update session for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	logger.Log("INFO", "Authentication completed successfully for user: "+payload.UserId)

	c.JSON(http.StatusOK, gin.H{
		"message": "OTP verification successful",
		"response": gin.H{
			"username":      user.UserName,
			"user_id":       user.ID,
			"role":          user.AccessType,
			"refresh_token": hashedRefresh,
			"access_token":  hashedAccess,
			"expiry_time":   "4hr",
		},
	})
}
