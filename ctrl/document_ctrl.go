package ctrl

import (
	"fmt"
	"form_pf_backend/modules"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// UploadDoc returns a gin.HandlerFunc (fresh + re-upload).
func UploadDoc(volumeRoot string) gin.HandlerFunc {
	_ = os.MkdirAll(volumeRoot, 0755)

	return func(c *gin.Context) {
		req, ok := modules.ValidateRequest(c)
		if !ok {
			return
		}

		clientID, _ := c.Get("clientId")
		filingDir, ok := modules.ResolveFilingDir(c, volumeRoot, fmt.Sprintf("%v", clientID), req)
		if !ok {
			return
		}

		if err := modules.SaveFiles(req.Files, filingDir); err != nil {
			c.JSON(http.StatusInternalServerError,
				gin.H{"message": "Internal Server Error. Please reach out to your admin"})
			return
		}

		filingID := filepath.Base(filingDir)[0:strings.Index(filepath.Base(filingDir), "+")]
		c.JSON(http.StatusOK, gin.H{
			"message":   "Successfully uploaded",
			"filing_id": filingID,
		})
	}
}
