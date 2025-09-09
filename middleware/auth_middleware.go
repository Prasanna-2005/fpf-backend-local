package middleware

import (
	"errors"
	"fmt"
	"form_pf_backend/utility"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	AuthError   = "User not found / Invalid User session"
	Restriction = "Insufficient privilege. Contact admin"
	UidError    = "User Id not present in Header"
)

var logger = utility.LoggerFunc()

/*
	------------------------------------------------------------------
	  Public API – three ways to call:
	    auth.Authenticate("admin")
	    auth.Authenticate([]string{"admin","user"})
	    auth.Authenticate("admin","user")

------------------------------------------------------------------
*/
func Authenticate(required ...interface{}) gin.HandlerFunc {
	// normalise whatever the caller passed into a []string
	allowed := make([]string, 0, len(required))
	for _, r := range required {
		switch v := r.(type) {
		case string:
			allowed = append(allowed, v)
		case []string:
			allowed = append(allowed, v...)
		default:
			panic(fmt.Sprintf("Authenticate: unsupported type %T", v))
		}
	}

	return func(c *gin.Context) {
		uid, err := ExtractUserID(c)
		if err != nil {
			abortWithMsg(c, http.StatusBadRequest, UidError,
				"Header doesn't have user id: "+err.Error())
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

		if !roleAllowed(claims.AccessType, allowed) {
			abortWithMsg(c, http.StatusForbidden, Restriction, "")
			return
		}

		logger.Log("DEBUG", fmt.Sprintf("clientID:%v ", claims.ClientID))
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
		logger.Log("DEBUG", cookieName)
		abortWithMsg(c, http.StatusUnauthorized, "access cookie not found", err.Error())
		return "", false
	}
	token, _ := c.Cookie(cookieName) // err already checked above
	if token == "" {
		abortWithMsg(c, http.StatusUnauthorized, AuthError, "")
		return "", false
	}
	return token, true
}

// true if userRole is in the allowed slice OR if no roles are required.
func roleAllowed(userRole string, allowed []string) bool {
	if len(allowed) == 0 {
		return true // empty requirement = allow all
	}
	for _, r := range allowed {
		if r == userRole {
			return true
		}
	}
	return false
}

// ExtractUserID pulls the user id from the X-User-ID header.
func ExtractUserID(c *gin.Context) (string, error) {
	id := c.GetHeader("X-User-ID")
	if id == "" {
		return "", errors.New("user_id missing in header")
	}
	return id, nil
}
