package utility

import (
	"form_pf_backend/models"

	"github.com/gin-gonic/gin"
)

// ContextFilingTracker is the gin key set by RequireFilingAccess after it loads the row.
const ContextFilingTracker = "filingTracker"

// FilingTracker returns the filing row already loaded for this request.
func FilingTracker(c *gin.Context) *models.FilingTracker {
	val, ok := c.Get(ContextFilingTracker)
	if !ok {
		return nil
	}
	tracker, _ := val.(*models.FilingTracker)
	return tracker
}
