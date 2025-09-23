package ctrl

import (
	"form_pf_backend/modules"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

func GetFilingResponse(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("filing_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid filingId"})
		return
	}

	resp, err := modules.BuildResponse(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}



func GetApprovedFilingResponse(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("filing_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid filingId"})
		return
	}

	resp, err := modules.TransformToSimpleResponse(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}