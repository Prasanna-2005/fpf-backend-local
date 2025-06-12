package ctrl

import (
	"fmt"
	"net/http"
	"time"

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
	err = utility.OtpService(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error"})
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "OTP sent successfully",
		"user_id": user.ID,
	})

}

func VerifyOTP(c *gin.Context) {
	cfg := utility.LoadOTPConfig()
	var payload struct {
		UserId string `json:"user_id" binding:"required"`
		Otp    string `json:"otp" binding:"required"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request payload"})
		return
	}

	otp, createdAt, err := db.GetOTP(payload.UserId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request Otp is not generated "})
		return
	}
	fmt.Println(createdAt)
	fmt.Println(createdAt.Add(cfg.OTPExpiry))
	fmt.Println(time.Now())
	fmt.Println(createdAt.Add(cfg.OTPExpiry).Before(time.Now()))
	if createdAt.Add(cfg.OTPExpiry).Before(time.Now()) {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "OTP has expired"})
		return
	}
	err = utility.CheckOtp([]byte(otp), []byte(payload.Otp))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid OTP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OTP verification successful",
	})
}
