package models

import "time"

type FilingTracker struct {
	ID          uint      `gorm:"primaryKey;autoIncrement;column:id"`
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
