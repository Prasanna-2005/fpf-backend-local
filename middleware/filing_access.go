package middleware

import (
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/utility"
	"net/http"
	"github.com/gin-gonic/gin"
)

const (
	FilingNotFoundMsg  = "Filing not found"
	FilingForbiddenMsg = "You do not have access to this filing"
)

var filingLogger = utility.LoggerFunc()

// RequireFilingAccess ensures the request's filing (from path or query) belongs to
// one of the authenticated user's client codes. Must be used after Authenticate()
// so that clientId is set in context.
//
// Reads filing_id from:
//   - c.Param("filing_id") for path params (e.g. /edit/response/:filing_id)
//   - c.Query("filingid") for query params (e.g. /submissions/history?filingid=...)
//
// If no filing identifier is present, passes through to the handler (which may
// return 400 for missing params). Use this middleware on any route group where
// APIs use filing_id.
func RequireFilingAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		filingID := c.Param("filing_id")
		if filingID == "" {
			filingID = c.Query("filingid")
		}
		if filingID == "" {
			c.Next()
			return
		}

		allowedCodes, err := getAllowedClientCodes(c)
		if err != nil {
			filingLogger.Log("WARNING", "RequireFilingAccess: "+err.Error())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": AuthError})
			return
		}
		if len(allowedCodes) == 0 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": FilingForbiddenMsg})
			return
		}

		tracker, err := db.GetFilingTracker(filingID)
		if err != nil || tracker == nil {
			filingLogger.Log("DEBUG", fmt.Sprintf("RequireFilingAccess: filing not found: %s - %v", filingID, err))
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": FilingNotFoundMsg})
			return
		}

		if !clientCodeAllowed(tracker.ClientCode, allowedCodes) {
			filingLogger.Log("WARNING", fmt.Sprintf("RequireFilingAccess: filing clientCode %s not in user's client codes %v (filing_id=%s)",
				tracker.ClientCode, allowedCodes, filingID))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": FilingForbiddenMsg})
			return
		}

		c.Set("filingClientCode", tracker.ClientCode)
		c.Next()
	}
}

func getAllowedClientCodes(c *gin.Context) ([]string, error) {
	clientIDVal, exists := c.Get("clientId")
	if !exists {
		return nil, fmt.Errorf("clientId not in context (Authenticate may not have run)")
	}
	switch v := clientIDVal.(type) {
	case string:
		if v == "" {
			return nil, nil
		}
		return []string{v}, nil
	case []string:
		return v, nil
	default:
		return nil, fmt.Errorf("clientId has unsupported type %T", clientIDVal)
	}
}

func clientCodeAllowed(filingClientCode string, allowed []string) bool {
	for _, c := range allowed {
		if c == filingClientCode {
			return true
		}
	}
	return false
}
