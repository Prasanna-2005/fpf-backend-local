package models

import "time"

type FilingTracker struct {
	Id          int       `gorm:"primaryKey;autoIncrement;column:id"`
	FilingName  string    `gorm:"type:varchar(50);not null;column:filingName"`
	ClientCode  string    `gorm:"type:varchar(20);not null;column:clientCode"`
	UploadCount int       `gorm:"not null;column:uploadCount"`
	IsSubmitted bool      `gorm:"not null;column:isSubmitted"`
	IsDeleted   bool      `gorm:"not null;column:isDeleted"`
	CreatedAt   time.Time `gorm:"not null;column:createdAt"`
	CreatedBy   int       `gorm:"not null;column:createdBy"`
	UpdatedAt   time.Time `gorm:"column:updatedAt"`
	UpdatedBy   int       `gorm:"column:updatedBy"`
}

func (FilingTracker) TableName() string {
	return "filing_tracker"
}

type FilingStatus struct {
	Id                uint      `gorm:"column:id;primaryKey;autoIncrement"`
	FilingID          string    `gorm:"column:filing_id"`
	ExtractionStatus  string    `gorm:"column:extraction_status"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
	UpdatedBy         string    `gorm:"column:updated_by"`
	ComputationStatus string    `gorm:"column:computation_status"`
	ReviewStatus      string    `gorm:"column:review_status"`
	FilingStatus      string    `gorm:"column:filing_status"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	CreatedBy         string    `gorm:"column:created_by"`
	ApprovalStatus    string    `gorm:"column:approval_status"`
}

func (FilingStatus) TableName() string {
	return "filing_status"
}
