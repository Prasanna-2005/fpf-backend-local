package ctrl

import (
	"errors"
	"form_pf_backend/db"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type reviewerResponse struct {
	UserID     string    `json:"user_id"`
	UserName   string    `json:"user_name"`
	ReviewedAt time.Time `json:"reviewed_at"`
}

type questionReviewResponse struct {
	FieldNo      string             `json:"field_no"`
	ReviewedByMe bool               `json:"reviewed_by_me"`
	Reviewers    []reviewerResponse `json:"reviewers"`
}

type reviewRequest struct {
	FundID  string `json:"fund_id"`
	FieldNo string `json:"field_no"`
}

func ListQuestionReviews(c *gin.Context) {
	filingID := c.Param("filing_id")
	fundID := strings.TrimSpace(c.Query("fundSeriesid"))
	if filingID == "" || fundID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "filing_id and fundSeriesid are required"})
		return
	}
	userID, ok := authUserID(c)
	if !ok {
		return
	}
	if !ensureReviewTarget(c, filingID, fundID, "", false) {
		return
	}

	rows, err := db.ListQuestionReviews(filingID, fundID)
	if err != nil {
		logger.Log("ERROR", "ListQuestionReviews: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reviews": groupQuestionReviews(rows, userID)})
}

func CreateQuestionReview(c *gin.Context) {
	filingID := c.Param("filing_id")
	var req reviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}
	req.FundID = strings.TrimSpace(req.FundID)
	req.FieldNo = strings.TrimSpace(req.FieldNo)
	if filingID == "" || req.FundID == "" || req.FieldNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "fund_id and field_no are required"})
		return
	}
	userID, ok := authUserID(c)
	if !ok {
		return
	}
	if !ensureReviewTarget(c, filingID, req.FundID, req.FieldNo, true) {
		return
	}

	created, err := db.CreateQuestionReview(filingID, req.FundID, req.FieldNo, userID)
	if err != nil {
		logger.Log("ERROR", "CreateQuestionReview: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	item, err := questionReviewItem(filingID, req.FundID, req.FieldNo, userID)
	if err != nil {
		logger.Log("ERROR", "CreateQuestionReview list: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, item)
}

func DeleteQuestionReview(c *gin.Context) {
	filingID := c.Param("filing_id")
	var req reviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}
	req.FundID = strings.TrimSpace(req.FundID)
	req.FieldNo = strings.TrimSpace(req.FieldNo)
	if filingID == "" || req.FundID == "" || req.FieldNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "fund_id and field_no are required"})
		return
	}
	userID, ok := authUserID(c)
	if !ok {
		return
	}
	if !ensureReviewTarget(c, filingID, req.FundID, req.FieldNo, true) {
		return
	}

	err := db.DeleteQuestionReview(filingID, req.FundID, req.FieldNo, userID)
	if errors.Is(err, db.ErrReviewNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"message": "you have not reviewed this question"})
		return
	}
	if err != nil {
		logger.Log("ERROR", "DeleteQuestionReview: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}

	item, err := questionReviewItem(filingID, req.FundID, req.FieldNo, userID)
	if err != nil {
		logger.Log("ERROR", "DeleteQuestionReview list: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	c.JSON(http.StatusOK, item)
}

func authUserID(c *gin.Context) (string, bool) {
	raw, exists := c.Get("userId")
	userID, _ := raw.(string)
	if !exists || strings.TrimSpace(userID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return "", false
	}
	return userID, true
}

func ensureReviewTarget(c *gin.Context, filingID, fundID, fieldNo string, requireField bool) bool {
	onFiling, err := db.FundOnFiling(filingID, fundID)
	if err != nil {
		logger.Log("ERROR", "FundOnFiling: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return false
	}
	if !onFiling {
		c.JSON(http.StatusNotFound, gin.H{"message": "fund not found on filing"})
		return false
	}
	if !requireField {
		return true
	}
	exists, err := db.QuestionFieldExists(fieldNo)
	if err != nil {
		logger.Log("ERROR", "QuestionFieldExists: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return false
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"message": "question not found"})
		return false
	}
	return true
}

func questionReviewItem(filingID, fundID, fieldNo, userID string) (questionReviewResponse, error) {
	rows, err := db.ListQuestionReviews(filingID, fundID)
	if err != nil {
		return questionReviewResponse{}, err
	}
	grouped := groupQuestionReviews(rows, userID)
	for _, item := range grouped {
		if item.FieldNo == fieldNo {
			return item, nil
		}
	}
	return questionReviewResponse{
		FieldNo:      fieldNo,
		ReviewedByMe: false,
		Reviewers:    []reviewerResponse{},
	}, nil
}

func groupQuestionReviews(rows []db.QuestionReviewRecord, userID string) []questionReviewResponse {
	order := make([]string, 0)
	byField := make(map[string]*questionReviewResponse)
	for _, row := range rows {
		item := byField[row.FieldNo]
		if item == nil {
			item = &questionReviewResponse{
				FieldNo:   row.FieldNo,
				Reviewers: []reviewerResponse{},
			}
			byField[row.FieldNo] = item
			order = append(order, row.FieldNo)
		}
		if row.CreatedBy == userID {
			item.ReviewedByMe = true
		}
		name := strings.TrimSpace(row.UserName)
		if name == "" {
			name = row.CreatedBy
		}
		item.Reviewers = append(item.Reviewers, reviewerResponse{
			UserID:     row.CreatedBy,
			UserName:   name,
			ReviewedAt: row.CreatedAt,
		})
	}
	out := make([]questionReviewResponse, 0, len(order))
	for _, fieldNo := range order {
		out = append(out, *byField[fieldNo])
	}
	return out
}
