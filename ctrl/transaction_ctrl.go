package ctrl

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type AcceptPayload struct {
	FieldNo    string `json:"field_no" binding:"required"`
	UserID     string `json:"user_id"  binding:"required"`
	FilingID   string `json:"filing_id" binding:"required"`
	FundID     string `json:"fund_id"`
	Timestamp  string `json:"timestamp" binding:"required"`
	SeriesName string `json:"series_name"`
	Data       struct {
		SubFieldNo string `json:"sub_field_no" binding:"required"`
		Value      string `json:"value"`
	} `json:"data"`
}

func AcceptItemValue(c *gin.Context) {
	filingID := c.Param("filing_id")
	if filingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "No filing found for the provided filing_id"})
		return
	}

	var payload AcceptPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid data format for the provided data field"})
		return
	}

	if payload.FilingID != filingID {
		c.JSON(http.StatusBadRequest, gin.H{"message": "No filing found for the provided filing_id"})
		return
	}

	if _, err := time.Parse(time.RFC3339Nano, payload.Timestamp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid data format for the provided data field"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Changes accepted successfully"})
}

package ctrl

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func MarkApproved(c *gin.Context) {
	// Hard-coded path to the XML file you want to return
	const xmlPath = `C:\Users\ANANDHAKUMAR\Downloads\qtech\FEG Private Investors\XML & Excel File\FEG Private Investors 2f0cfc75-a070-49f9-9033-ff60f90f84c6.xml`

	// Open the file
	f, err := os.Open(filepath.Clean(xmlPath))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Sever Error. Please reach out to your admin"})
		return
	}
	defer f.Close()

	// Stream the file back with correct XML content-type
	c.Header("Content-Type", "application/xml")
	http.ServeContent(c.Writer, c.Request, filepath.Base(xmlPath), /*modTime*/ time.Time{}, f)
}

package ctrl

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// MarkApproved streams the static XML file on success.
// RBAC middleware takes care of returning 401 when the caller is not authorized.
func MarkApproved(c *gin.Context) {
	const xmlPath = `C:\Users\ANANDHAKUMAR\Downloads\qtech\FEG Private Investors\XML & Excel File\FEG Private Investors 2f0cfc75-a070-49f9-9033-ff60f90f84c6.xml`

	f, err := os.Open(filepath.Clean(xmlPath))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": ServerError,
		})
		return
	}
	defer f.Close()

	// On success, send the XML file directly
	c.Header("Content-Type", "application/xml")
	http.ServeContent(c.Writer, c.Request, filepath.Base(xmlPath), time.Time{}, f)
}