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

var log = utility.LoggerFunc() // Initialize logger

// Login handles the user login request
func Login(c *gin.Context) {
	log.Log("INFO", "Login attempt initiated")

	var user models.User
	var payload struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	// Bind and validate request payload
	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Log("ERROR", "Login failed: invalid payload - "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	log.Log("DEBUG", "Login attempt for username: "+payload.Username)

	// Validate user credentials
	user, err := db.ValidateCredentials(payload.Username, payload.Password)
	if err != nil {
		if err.Error() == "record not found" {
			log.Log("WARNING", "Login failed: user not found for username: "+payload.Username)
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Incorrect Username or Password"})
		} else {
			log.Log("ERROR", "Login failed: credential validation error - "+err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		}
		return
	}

	log.Log("INFO", "Credentials validated successfully for user: "+user.ID)

	// Generate and send OTP
	err = middleware.OtpService(user.ID)
	if err != nil {
		log.Log("ERROR", "Login failed: OTP service error for user "+user.ID+" - "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	log.Log("INFO", "Login successful: OTP sent to user "+user.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Login verified, generating OTP",
		"user_id": user.ID,
	})
}

// ResendOTP handles OTP resend requests
func ResendOTP(c *gin.Context) {
	log.Log("INFO", "OTP resend request initiated")

	var payload struct {
		UserId string `json:"user_id" binding:"required"`
	}

	// Bind and validate request payload
	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Log("ERROR", "OTP resend failed: invalid payload - "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	log.Log("DEBUG", "OTP resend requested for user: "+payload.UserId)

	// Generate and send new OTP
	err := middleware.OtpService(payload.UserId)
	if err != nil {
		log.Log("ERROR", "OTP resend failed for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	log.Log("INFO", "OTP resend successful for user: "+payload.UserId)
	c.JSON(http.StatusOK, gin.H{
		"message": "OTP generated successful",
	})
}

// VerifyOTP handles OTP verification and token generation
func VerifyOTP(c *gin.Context) {
	log.Log("INFO", "OTP verification initiated")

	var payload struct {
		UserId string `json:"user_id" binding:"required"`
		Otp    string `json:"otp" binding:"required"`
	}

	// Bind and validate request payload
	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Log("ERROR", "OTP verification failed: invalid payload - "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	log.Log("DEBUG", "OTP verification attempt for user: "+payload.UserId)

	// Verify OTP
	isValid, err := utility.CheckOtp(payload.Otp, payload.UserId)
	if err != nil {
		log.Log("ERROR", "OTP verification failed for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid OTP"})
		return
	}

	if !isValid {
		log.Log("WARNING", "OTP verification failed: invalid OTP for user "+payload.UserId)
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid OTP"})
		return
	}

	log.Log("INFO", "OTP verified successfully for user: "+payload.UserId)

	// Get user details
	user, err := db.GetUser(payload.UserId)
	if err != nil {
		log.Log("ERROR", "Failed to retrieve user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user ID"})
		return
	}

	log.Log("DEBUG", "Generating refresh token")

	// Generate refresh token
	refresh, err := utility.GenerateRefreshToken(payload.UserId)
	if err != nil {
		log.Log("ERROR", "Failed to generate refresh token for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	log.Log("DEBUG", "Generating access token")

	// Generate access token
	access, err := utility.GenerateAccessToken(refresh, user.AccessType)
	if err != nil {
		log.Log("ERROR", "Failed to generate access token for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	log.Log("DEBUG", "Hashing access token")
	tokenCfg := config.Token()
	// Hash access token
	hashedAccess, err := utility.HashPassword(access, tokenCfg.TokenSecret)
	if err != nil {
		log.Log("ERROR", "Failed to hash access token for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	log.Log("DEBUG", "Hashing refresh token")

	// Hash refresh token
	hashedRefresh, err := utility.HashPassword(refresh, tokenCfg.TokenSecret)
	if err != nil {
		log.Log("ERROR", "Failed to hash refresh token for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	log.Log("DEBUG", "Updating user tokens and session")

	// Update tokens and session
	if err := db.UpdateToken(user, hashedAccess, hashedRefresh); err != nil {
		log.Log("ERROR", "Failed to update tokens for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	if err := db.UpdateSession(user); err != nil {
		log.Log("ERROR", "Failed to update session for user "+payload.UserId+": "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	log.Log("INFO", "Authentication completed successfully for user: "+payload.UserId)

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
