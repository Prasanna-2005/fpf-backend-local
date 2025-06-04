package server

import (
	"form_pf_backend/ctrl"

	"github.com/gin-gonic/gin"
)

func SetupRoutes() *gin.Engine {
	r := gin.Default()

	r.POST("/auth/login", ctrl.Login)
	r.POST("/auth/verify-otp", ctrl.VerifyOTP)

	return r
}
