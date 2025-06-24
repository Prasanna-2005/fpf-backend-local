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

// Login handles the user login request
func Login(c *gin.Context) {
	utility.Info("Login attempt initiated")

	var user models.User
	var payload struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	// Bind and validate request payload
	if err := c.ShouldBindJSON(&payload); err != nil {
		utility.Errorf("Login failed: invalid payload - %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	utility.Debugf("Login attempt for username: %s", payload.Username)

	// Validate user credentials
	user, err := db.ValidateCredentials(payload.Username, payload.Password)
	if err != nil {
		if err.Error() == "record not found" {
			utility.Warningf("Login failed: user not found for username: %s", payload.Username)
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Incorrect Username or Password"})
		} else {
			utility.Errorf("Login failed: credential validation error - %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		}
		return
	}

	utility.Infof("Credentials validated successfully for user: %s", user.ID)

	// Generate and send OTP
	err = middleware.OtpService(user.ID)
	if err != nil {
		utility.Errorf("Login failed: OTP service error for user %s - %v", user.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	utility.Infof("Login successful: OTP sent to user %s", user.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Login verified, generating OTP",
		"user_id": user.ID,
	})
}

// ResendOTP handles OTP resend requests
func ResendOTP(c *gin.Context) {
	utility.Info("OTP resend request initiated")

	var payload struct {
		UserId string `json:"user_id" binding:"required"`
	}

	// Bind and validate request payload
	if err := c.ShouldBindJSON(&payload); err != nil {
		utility.Errorf("OTP resend failed: invalid payload - %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	utility.Debugf("OTP resend requested for user: %s", payload.UserId)

	// Generate and send new OTP
	err := middleware.OtpService(payload.UserId)
	if err != nil {
		utility.Errorf("OTP resend failed for user %s: %v", payload.UserId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	utility.Infof("OTP resend successful for user: %s", payload.UserId)
	c.JSON(http.StatusOK, gin.H{
		"message": "OTP generated successful",
	})
}

// VerifyOTP handles OTP verification and token generation
func VerifyOTP(c *gin.Context) {
	utility.Info("OTP verification initiated")

	var payload struct {
		UserId string `json:"user_id" binding:"required"`
		Otp    string `json:"otp" binding:"required"`
	}

	// Bind and validate request payload
	if err := c.ShouldBindJSON(&payload); err != nil {
		utility.Errorf("OTP verification failed: invalid payload - %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	utility.Debugf("OTP verification attempt for user: %s", payload.UserId)

	// Verify OTP
	isValid, err := utility.CheckOtp(payload.Otp, payload.UserId)
	if err != nil {
		utility.Errorf("OTP verification failed for user %s: %v", payload.UserId, err)
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid OTP"})
		return
	}

	if !isValid {
		utility.Warningf("OTP verification failed: invalid OTP for user %s", payload.UserId)
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid OTP"})
		return
	}

	utility.Infof("OTP verified successfully for user: %s", payload.UserId)

	// Get user details
	user, err := db.GetUser(payload.UserId)
	if err != nil {
		utility.Errorf("Failed to retrieve user %s: %v", payload.UserId, err)
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user ID"})
		return
	}

	utility.Debug("Generating refresh token")

	// Generate refresh token
	refresh, err := utility.GenerateRefreshToken(payload.UserId)
	if err != nil {
		utility.Errorf("Failed to generate refresh token for user %s: %v", payload.UserId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	utility.Debug("Generating access token")

	// Generate access token
	access, err := utility.GenerateAccessToken(refresh, user.AccessType)
	if err != nil {
		utility.Errorf("Failed to generate access token for user %s: %v", payload.UserId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	utility.Debug("Hashing access token")
	tokenCfg := config.Token()
	// Hash access token
	hashedAccess, err := utility.HashPassword(access, tokenCfg.TokenSecret)
	if err != nil {
		utility.Errorf("Failed to hash access token for user %s: %v", payload.UserId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	utility.Debug("Hashing refresh token")

	// Hash refresh token
	hashedRefresh, err := utility.HashPassword(refresh, tokenCfg.TokenSecret)
	if err != nil {
		utility.Errorf("Failed to hash refresh token for user %s: %v", payload.UserId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": AuthError})
		return
	}

	utility.Debug("Updating user tokens and session")

	// Update tokens and session
	if err := db.UpdateToken(user, hashedAccess, hashedRefresh); err != nil {
		utility.Errorf("Failed to update tokens for user %s: %v", payload.UserId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	if err := db.UpdateSession(user); err != nil {
		utility.Errorf("Failed to update session for user %s: %v", payload.UserId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	utility.Infof("Authentication completed successfully for user: %s", payload.UserId)

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
