package ctrl

import (
	"fmt"
	"net/http"

	"form_pf_backend/db"
	"form_pf_backend/middleware"
	"form_pf_backend/models"
	"form_pf_backend/utility"

	"github.com/gin-gonic/gin"
)

const (
	ServerError  = "Internal server Error"
	PayloadError = "Invalid request payload"
)

// Login handles the user login request
func Login(c *gin.Context) {
	var user models.User
	var payload struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	user, err := db.ValidateCredentials(payload.Username, payload.Password)

	if err != nil {
		if err.Error() == "record not found" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Incorrect Username or Password"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login verified, generating OTP",
		"user_id": user.ID,
	})
}

func GenerateOTP(c *gin.Context) {
	var payload struct {
		UserId string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}
	err := middleware.OtpService(payload.UserId)
	if err != nil {
		fmt.Print(err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "OTP generated successful",
	})
}

func VerifyOTP(c *gin.Context) {

	var payload struct {
		UserId string `json:"user_id" binding:"required"`
		Otp    string `json:"otp" binding:"required"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	val, err := utility.CheckOtp(payload.Otp, payload.UserId)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid OTP"})
		return
	}
	if !val {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid OTP"})
		return
	}
	token, err := utility.GenerateToken(payload.UserId)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not authenticate user."})
		return
	}
	user, err := db.GetUser(payload.UserId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user ID"})
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "OTP verification successful",
		"response": gin.H{
			"username":      user.UserName,
			"user_id":       user.ID,
			"role":          user.AccessType,
			"refresh_token": token,
			"access_token":  token,
			"expiry_time":   "60 min",
		},
	})

}
