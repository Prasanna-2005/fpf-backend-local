package ctrl

import (
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type latestChangeItem struct {
	FundSeriesId string    `json:fundSeriesId`
	FormLineNo   string    `json:"formLineNo"`
	FieldNo      string    `json:"fieldNo"`
	Value        string    `json:"value"`
	CreatedAt    time.Time `json:"created_at"`
	CreatedBy    string    `json:"created_by"`
}

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
		ID           string    `json:"id"`
		FilingName   string    `json:"filing_name"`
		PreparedBy   string    `json:"prepared_by"`
		IsSubmitted  bool      `json:"is_submitted"`
		IsDeleted    bool      `json:"is_deleted"`
		ModifiedBy   string    `json:"modified_by"`
		ModifiedOn   time.Time `json:"modified_on"`
		FilingStatus string    `json:"filingStatus"`
	}

	list := make([]respFiling, 0, len(*filingsPtr))
	for _, f := range *filingsPtr {
		modifiedOn := f.CreatedAt
		if f.UpdatedAt.IsZero() && f.UpdatedAt.After(f.CreatedAt) {
			modifiedOn = f.UpdatedAt
		}
		list = append(list, respFiling{
			ID:           f.Id,
			FilingName:   f.FilingName,
			IsSubmitted:  f.IsSubmitted,
			IsDeleted:    f.IsDeleted,
			PreparedBy:   userMap[f.CreatedBy],
			ModifiedOn:   modifiedOn,
			ModifiedBy:   userMap[f.UpdatedBy],
			FilingStatus: f.FilingStatus,
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

func GetRelatedFilings(c *gin.Context) {
	filingID := c.Param("filing_id")
	if strings.TrimSpace(filingID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "filing_id is required"})
		return
	}

	userID := c.GetString("userId")
	current, err := db.GetFilingTracker(filingID)
	if err != nil || current == nil {
		if err != nil {
			logger.Log("ERROR", fmt.Sprintf("GetRelatedFilings tracker: user_id=%s filing=%s: %v", userID, filingID, err))
		} else {
			logger.Log("ERROR", fmt.Sprintf("GetRelatedFilings: user_id=%s filing tracker missing for %s", userID, filingID))
		}
		c.JSON(http.StatusNotFound, gin.H{"message": FilingNotFound})
		return
	}

	series := strings.TrimSpace(current.Series)
	if series == "" {
		c.JSON(http.StatusOK, gin.H{"data": []any{}, "company": current.Series, "current_version": current.CurrentVersion})
		return
	}

	related, err := db.GetRelatedFilingsBySeries(filingID, series, current.ClientCode)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("GetRelatedFilings: user_id=%s filing=%s: %v", userID, filingID, err))
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	type relatedFiling struct {
		ID           string    `json:"id"`
		FilingName   string    `json:"filing_name"`
		Series       string    `json:"series"`
		ModifiedOn   time.Time `json:"modified_on"`
		FilingStatus string    `json:"filingStatus"`
	}

	list := make([]relatedFiling, 0, len(related))
	for _, f := range related {
		if strings.EqualFold(strings.TrimSpace(f.FilingStatus), "Archived") {
			continue
		}
		modifiedOn := f.CreatedAt
		if !f.UpdatedAt.IsZero() && f.UpdatedAt.After(f.CreatedAt) {
			modifiedOn = f.UpdatedAt
		}
		list = append(list, relatedFiling{
			ID:           f.Id,
			FilingName:   f.FilingName,
			Series:       f.Series,
			ModifiedOn:   modifiedOn,
			FilingStatus: f.FilingStatus,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": list, "company": current.Series, "current_version": current.CurrentVersion})
}

func GetFilingVersions(c *gin.Context) {
	filingID := c.Param("filing_id")
	if strings.TrimSpace(filingID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "filing_id is required"})
		return
	}

	userID := c.GetString("userId")
	current, err := db.GetFilingTracker(filingID)
	if err != nil || current == nil {
		if err != nil {
			logger.Log("ERROR", fmt.Sprintf("GetFilingVersions tracker: user_id=%s filing=%s: %v", userID, filingID, err))
		} else {
			logger.Log("ERROR", fmt.Sprintf("GetFilingVersions: user_id=%s filing tracker missing for %s", userID, filingID))
		}
		c.JSON(http.StatusNotFound, gin.H{"message": FilingNotFound})
		return
	}

	versions, err := db.AvailableVersions(filingID)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("GetFilingVersions: user_id=%s filing=%s: %v", userID, filingID, err))
		c.JSON(http.StatusOK, gin.H{
			"data":            []any{},
			"company":         current.Series,
			"current_version": current.CurrentVersion,
		})
		return
	}

	head, err := db.HeadVersion(filingID)
	if err != nil {
		head = current.CurrentVersion
	}

	type filingVersion struct {
		Version   int  `json:"version"`
		IsCurrent bool `json:"is_current"`
	}

	list := make([]filingVersion, 0, len(versions))
	for _, v := range versions {
		list = append(list, filingVersion{Version: v, IsCurrent: v == head})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":            list,
		"company":         current.Series,
		"current_version": head,
	})
}

func GetLatestChanges(c *gin.Context) {
	var req struct {
		FilingID string `json:"filing_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log("INFO", fmt.Sprintf("Internal server error:%v", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"message": "Internal Server error"})
		return
	}

	userId := c.GetString("userId")
	if userId == "" {
		logger.Log("ERROR", "Empty string present in claims.ID for userId")
	}
	items, updatedAt := db.GetSubmissionLatestChanges(req.FilingID, userId)

	// Map ORM model → response shape
	response := make([]latestChangeItem, len(items))
	for i, item := range items {
		response[i] = latestChangeItem{
			FundSeriesId: item.FundSeriesid,
			FormLineNo:   item.FormLineNo,
			FieldNo:      item.FieldNo,
			Value:        item.Value,
			CreatedAt:    item.CreatedAt,
			CreatedBy:    item.Username,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"filing_id":       req.FilingID,
		"count":           len(response),
		"cache_refreshed": updatedAt,
		"changes":         response,
	})
}
