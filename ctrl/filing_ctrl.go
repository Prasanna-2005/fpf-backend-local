package ctrl

import (
	"form_pf_backend/db"
	"form_pf_backend/models"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const FileNotFnd = "Filing id is doesn't exist"

func GetFilings(c *gin.Context) {
	filingsPtr, err := db.GetAllFilings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": ServerError,
		})
		return
	}

	// success payload
	type respFiling struct {
		ID          uint      `json:"id"`
		FilingName  string    `json:"filing_name"`
		IsSubmitted bool      `json:"is_submitted"`
		IsDeleted   bool      `json:"is_deleted"`
		CreatedAt   time.Time `json:"created_at"`
		CreatedBy   int       `json:"created_by"`
		UpdatedAt   time.Time `json:"updated_at,omitempty"`
		UpdatedBy   int       `json:"updated_by,omitempty"`
	}

	var list []respFiling
	for _, f := range *filingsPtr {
		list = append(list, respFiling{
			ID:          f.ID,
			FilingName:  f.FilingName,
			IsSubmitted: f.IsSubmitted,
			IsDeleted:   f.IsDeleted,
			CreatedAt:   f.CreatedAt,
			CreatedBy:   f.CreatedBy,
			UpdatedAt:   f.UpdatedAt,
			UpdatedBy:   f.UpdatedBy,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": list})
}

// DerivedStatus now expects single structs, not pointers
func DerivedStatus(fs models.FilingStatus, ft models.FilingTracker) string {
	if ft.IsDeleted {
		return "deleted"
	}

	// archived?
	if !fs.CreatedAt.IsZero() && time.Since(fs.CreatedAt) > 365*24*time.Hour {
		return "archived"
	}

	// priority cascade (case-insensitive)
	switch {
	case strings.ToLower(fs.ApprovalStatus) == "yes":
		return "approved"
	case strings.ToLower(fs.ReviewStatus) == "yes":
		return "in review"
	case strings.ToLower(fs.ExtractionStatus) == "yes" ||
		strings.ToLower(fs.ComputationStatus) == "yes":
		return "draft"
	default:
		return "unknown"
	}
}

func FilingStatus(c *gin.Context) {
	filingID := c.Param("filing_id")

	fss, err := db.GetFilingStatus(filingID)
	if err != nil || len(fss) == 0 {
		logger.Log("ERROR", "filing_status not found: "+err.Error())
		c.JSON(http.StatusNotFound, gin.H{"message": FileNotFnd})
		return
	}

	fts, err := db.GetFilingTracker(filingID)
	if err != nil || len(fts) == 0 {
		logger.Log("ERROR", "filing_tracker not found: "+err.Error())
		c.JSON(http.StatusNotFound, gin.H{"message": FileNotFnd})
		return
	}

	state := DerivedStatus(fss[0], fts[0])

	c.JSON(http.StatusOK, gin.H{
		"filing_id": filingID,
		"state":     state,
	})
}
