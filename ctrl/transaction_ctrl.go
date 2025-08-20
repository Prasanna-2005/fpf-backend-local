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
		c.JSON(http.StatusBadRequest, gin.H{"message": "FILING_NOT_FOUND"})
		return
	}

	var payload AcceptPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "INVALID_FORMAT"})
		return
	}

	// Ensure the filing_id in the path and body match
	if payload.FilingID != filingID {
		c.JSON(http.StatusBadRequest, gin.H{"message": "FILING_NOT_FOUND"})
		return
	}

	// Validate timestamp layout
	if _, err := time.Parse(time.RFC3339Nano, payload.Timestamp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "INVALID_FORMAT"})
		return
	}

	// Dummy success
	c.JSON(http.StatusOK, gin.H{"message": "Changes accepted successfully"})
}
