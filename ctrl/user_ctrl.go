package ctrl

import (
	"fmt"
	"net/http"

	"form_pf_backend/db"
	"form_pf_backend/models"
	"form_pf_backend/utility"

	"github.com/gin-gonic/gin"
)

// Login handles the user login request
func Login(c *gin.Context) {

	var user models.User
	var payload struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request payload"})
		return
	}

	// Find user by username and password
	user, err := db.ValidateCredentials(payload.Username, payload.Password)
	if err != nil {
		if err.Error() == "record not found" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Incorrect Username or Password"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error1"})
		}
		return
	}
	cfg := utility.LoadOTPConfig()
	code, hash, err := utility.GenerateSecureOTP(cfg.OTPLength)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error"})
		return
	}
	err = db.OtpUpdate(hash, &user)
	if err != nil {
		fmt.Print(err)
		return
	}
	err = utility.SendOTPEmail(cfg, user.Email, code)
	if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"message": "SMTP client or User email doesn't exist"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OTP sent successfully",
		"user_id": user.ID,
		"otp":     code,
	})

}
