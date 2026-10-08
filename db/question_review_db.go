package db

import (
	"errors"
	"form_pf_backend/models"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrQuestionNotFound = errors.New("question not found")
	ErrFundNotOnFiling  = errors.New("fund not found on filing")
	ErrReviewNotFound   = errors.New("review not found")
)

// QuestionReviewRecord is one reviewer row joined with the user's display name.
type QuestionReviewRecord struct {
	FieldNo   string    `gorm:"column:fieldNo"`
	CreatedBy string    `gorm:"column:createdBy"`
	UserName  string    `gorm:"column:userName"`
	CreatedAt time.Time `gorm:"column:createdAt"`
}

func QuestionFieldExists(fieldNo string) (bool, error) {
	var count int64
	err := GetDB().
		Table("form_fields_ms_legacy").
		Where("fieldNo = ?", fieldNo).
		Limit(1).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func FundOnFiling(filingID, fundID string) (bool, error) {
	ids, err := FetchFundSeriesForFiling(filingID)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		if id == fundID {
			return true, nil
		}
	}
	return false, nil
}

func ListQuestionReviews(filingID, fundID string) ([]QuestionReviewRecord, error) {
	var rows []QuestionReviewRecord
	err := GetDB().
		Table("question_reviews qr").
		Select("qr.fieldNo, qr.createdBy, users.userName, qr.createdAt").
		Joins("LEFT JOIN users ON users.id = qr.createdBy").
		Where("qr.filingId = ? AND qr.fundSeriesId = ?", filingID, fundID).
		Order("qr.fieldNo, qr.createdAt").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []QuestionReviewRecord{}
	}
	return rows, nil
}

// CreateQuestionReview inserts the current user's review.
// created is false when that user already has a row for the question.
func CreateQuestionReview(filingID, fundID, fieldNo, userID string) (bool, error) {
	row := models.QuestionReview{
		ID:           uuid.NewString(),
		FilingID:     filingID,
		FundSeriesID: fundID,
		FieldNo:      fieldNo,
		CreatedBy:    userID,
		CreatedAt:    time.Now(),
	}
	err := GetDB().Create(&row).Error
	if err == nil {
		return true, nil
	}
	if isDuplicateReview(err) {
		return false, nil
	}
	return false, err
}

// DeleteQuestionReview removes only the given user's review of the question.
func DeleteQuestionReview(filingID, fundID, fieldNo, userID string) error {
	res := GetDB().
		Where("filingId = ? AND fundSeriesId = ? AND fieldNo = ? AND createdBy = ?",
			filingID, fundID, fieldNo, userID).
		Delete(&models.QuestionReview{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrReviewNotFound
	}
	return nil
}

func isDuplicateReview(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
