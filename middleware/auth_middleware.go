package middleware

import (
	"form_pf_backend/utility"
	"net/http"

	"github.com/gin-gonic/gin"
)

var AuthError = "User not found / Invalid User session"
var Restriction = "Insufficient privellege.Contact admin"
var logger = utility.LoggerFunc()

func Authenticate(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("access")
		if token == "" || err != nil {
			logger.Log("Error", "Authenticate error: "+err.Error())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": AuthError})
			return
		}

		claims, err := utility.VerifyAccessToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": AuthError})
			return
		}

		userID := claims.ID
		role := claims.AccessType

		if requiredRole != "" && role != requiredRole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": Restriction})
			return
		}

		c.Set("userId", userID)
		c.Set("role", role)

		c.Next()
	}
}
