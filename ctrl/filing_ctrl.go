package ctrl

import (
	"form_pf_backend/db"
	"form_pf_backend/models"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func GetFilings(c *gin.Context) {
	clientCode := c.Query("clientCode")
	if clientCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "clientCode is required"})
		return
	}
	filingsPtr, err := db.GetFilingsByClientCode(clientCode)
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

	type respFiling struct {
		ID          string    `json:"id"`
		FilingName  string    `json:"filing_name"`
		PreparedBy  string    `json:"prepared_by"`
		IsSubmitted bool      `json:"is_submitted"`
		IsDeleted   bool      `json:"is_deleted"`
		ModifiedBy  string    `json:"modified_by"`
		ModifiedOn  time.Time `json:"modified_on"`
		FilingStatus      string	  `json:"filingStatus"`
	}

	list := make([]respFiling, 0, len(*filingsPtr))
	for _, f := range *filingsPtr {
		modifiedOn := f.CreatedAt
		if f.UpdatedAt.IsZero() && f.UpdatedAt.After(f.CreatedAt) {
			modifiedOn = f.UpdatedAt
		}
		list = append(list, respFiling{
			ID:          f.Id,
			FilingName:  f.FilingName,
			IsSubmitted: f.IsSubmitted,
			IsDeleted:   f.IsDeleted,
			PreparedBy:  userMap[f.CreatedBy],
			ModifiedOn:  modifiedOn,
			ModifiedBy:  userMap[f.UpdatedBy],
			FilingStatus: 	 f.FilingStatus,
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
	case strings.EqualFold(fs.ApprovalStatus, "yes"):
		return "approved"
	case strings.EqualFold(fs.ReviewStatus, "yes"):
		return "in review"
	case strings.TrimSpace(fs.ExtractionStatus) == "" ||
		strings.TrimSpace(fs.ComputationStatus) == "":
		return "draft"
	default:
		return "Not filed"
	}
}

func FilingStatus(c *gin.Context) {
	filingID := c.Param("filing_id")

	fss, err := db.GetFilingStatus(filingID)
	if err != nil {
		logger.Log("ERROR", "filing_status not found: "+err.Error())
		c.JSON(http.StatusNotFound, gin.H{"message": FilingNotFound})
		return
	}

	fts, err := db.GetFilingTracker(filingID)
	if err != nil {
		logger.Log("ERROR", "filing_tracker not found: "+err.Error())
		c.JSON(http.StatusNotFound, gin.H{"message": FilingNotFound})
		return
	}

	state := DerivedStatus(fss, fts)

	c.JSON(http.StatusOK, gin.H{
		"filing_id": filingID,
		"state":     state,
	})
}
