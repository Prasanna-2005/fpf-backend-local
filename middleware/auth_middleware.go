package middleware

import (
	"errors"
	"form_pf_backend/utility"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	AuthError   = "User not found / Invalid User session"
	Restriction = "Insufficient privellege.Contact admin"
	UidError    = "User Id not present in Header"
)

var logger = utility.LoggerFunc()

func Authenticate(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, err := ExtractUserID(c)
		if err != nil {
			abortWithMsg(c, http.StatusBadRequest, UidError, "Payload doesn't have user id: "+err.Error())
			return
		}

		token, ok := findAccessToken(c, uid)
		if !ok {
			return // already aborted inside helper
		}

		claims, err := utility.VerifyAccessToken(token)
		if err != nil {
			abortWithMsg(c, http.StatusUnauthorized, AuthError, "")
			return
		}

		if !roleAllowed(claims.AccessType, requiredRole) {
			abortWithMsg(c, http.StatusForbidden, Restriction, "")
			return
		}

		c.Set("userId", claims.ID)
		c.Set("role", claims.AccessType)
		c.Set("clientId", claims.ClientID)
		c.Next()
	}
}

/* ---------- helpers ---------- */
func abortWithMsg(c *gin.Context, code int, msg, logMsg string) {
	if logMsg != "" {
		logger.Log("WARNING", logMsg)
	}
	c.AbortWithStatusJSON(code, gin.H{"message": msg})
}

func findAccessToken(c *gin.Context, uid string) (string, bool) {
	cookieName := uid + "access"
	if _, err := c.Cookie(cookieName); err != nil {
		abortWithMsg(c, http.StatusUnauthorized, "access cookie not found", "")
		return "", false
	}
	token, _ := c.Cookie(cookieName) // err already checked above
	if token == "" {
		abortWithMsg(c, http.StatusUnauthorized, AuthError, "")
		return "", false
	}
	return token, true
}

func roleAllowed(userRole, required string) bool {
	return required == "" || userRole == required
}

// Returns the user_id (string) or an error.
// Safe to call from middleware; downstream handlers can still bind JSON.
func ExtractUserID(c *gin.Context) (string, error) {
	id := c.GetHeader("X-User-ID")
	if id == "" {
		return "", errors.New("user_id missing in header")
	}
	return id, nil
}
