package ctrl

import (
	"form_pf_backend/db"
	"form_pf_backend/models"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const FileNotFnd = "Filing id is doesn't exist"

func GetFilings(c *gin.Context) {
	filingsPtr, err := db.GetAllFilings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	userList, err := db.GetAllUser()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	// build id → userName map  (User.ID is string)
	userMap := make(map[string]string, len(*userList))
	for _, u := range *userList {
		userMap[u.ID] = u.UserName
	}

	// response DTO – numeric IDs removed from JSON
	type respFiling struct {
		ID          int       `json:"id"`
		FilingName  string    `json:"filing_name"`
		IsSubmitted bool      `json:"is_submitted"`
		IsDeleted   bool      `json:"is_deleted"`
		CreatedAt   time.Time `json:"created_at"`
		CreatedBy   string    `json:"created_by"` // <-- string (username)
		UpdatedAt   time.Time `json:"updated_at,omitempty"`
		UpdatedBy   string    `json:"updated_by,omitempty"` // <-- string (username)
	}

	list := make([]respFiling, 0, len(*filingsPtr))
	for _, f := range *filingsPtr {
		list = append(list, respFiling{
			ID:          f.Id,
			FilingName:  f.FilingName,
			IsSubmitted: f.IsSubmitted,
			IsDeleted:   f.IsDeleted,
			CreatedAt:   f.CreatedAt,
			CreatedBy:   userMap[strconv.Itoa(f.CreatedBy)],
			UpdatedAt:   f.UpdatedAt,
			UpdatedBy:   userMap[strconv.Itoa(f.UpdatedBy)],
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": list})
}
func DerivedStatus(fs *models.FilingStatus, ft *models.FilingTracker) string {
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
	if err != nil {
		logger.Log("ERROR", "filing_status not found: "+err.Error())
		c.JSON(http.StatusNotFound, gin.H{"message": FileNotFnd})
		return
	}

	fts, err := db.GetFilingTracker(filingID)
	if err != nil {
		logger.Log("ERROR", "filing_tracker not found: "+err.Error())
		c.JSON(http.StatusNotFound, gin.H{"message": FileNotFnd})
		return
	}

	state := DerivedStatus(fss, fts)

	c.JSON(http.StatusOK, gin.H{
		"filing_id": filingID,
		"state":     state,
	})
}
