package models

import "time"

// QuestionReview is one user's review of one question.
// It is independent of form_data_legacy value versions.
type QuestionReview struct {
	ID           string    `gorm:"primaryKey;column:id;type:char(36)"`
	FilingID     string    `gorm:"column:filingId;type:varchar(64);not null"`
	FundSeriesID string    `gorm:"column:fundSeriesId;type:varchar(64);not null"`
	FieldNo      string    `gorm:"column:fieldNo;type:varchar(64);not null"`
	CreatedBy    string    `gorm:"column:createdBy;type:varchar(64);not null"`
	CreatedAt    time.Time `gorm:"column:createdAt;not null"`
}

func (QuestionReview) TableName() string { return "question_reviews" }
