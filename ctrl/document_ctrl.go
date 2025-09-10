// ctrl/document_ctrl.go
package ctrl

import (
	"fmt"
	"net/http"

	"form_pf_backend/models"
	"form_pf_backend/modules"

	"github.com/gin-gonic/gin"
)

// UploadDoc handles document upload requests with optimized flow
func UploadDoc(volumeRoot string) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Log("DEBUG", fmt.Sprintf("UploadDoc invoked, volumeRoot=%s", volumeRoot))

		// Pipeline approach: each step returns early on failure
		if !ensureUploadDirectory(c, volumeRoot) {
			return
		}

		req, ok := modules.ValidateAndBind(c)
		if !ok {
			return // ValidateAndBind already handled the response
		}

		logger.Log("INFO", fmt.Sprintf("Validated request: user_id=%s, new_filing=%t, filing_name=%s",
			req.UserID, req.NewFiling, req.FilingName))

		if err := modules.ProcessUpload(c, volumeRoot, req); err != nil {
			logger.Log("ERROR", fmt.Sprintf("ProcessUpload failed: %v", err))
			return // ProcessUpload already handled the response
		}

		logger.Log("INFO", fmt.Sprintf("Upload successful for filing_name=%s", req.FilingName))
	}
}

// ensureUploadDirectory ensures the upload directory exists
func ensureUploadDirectory(c *gin.Context, volumeRoot string) bool {
	if err := modules.EnsureVolumeDir(volumeRoot); err != nil {
		logger.Log("ERROR", fmt.Sprintf("EnsureVolumeDir failed: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"message": "cannot create upload directory"})
		return false
	}
	logger.Log("DEBUG", "Upload volume directory OK")
	return true
}

// MapFiles is now a standard gin.HandlerFunc
func MapFiles(c *gin.Context) {
	var req models.FileIdentificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request"})
		return
	}

	resp, err := modules.IdentifyFiles(&req)
	if err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": ServerError})
		return
	}
	c.JSON(http.StatusOK, resp)
}
