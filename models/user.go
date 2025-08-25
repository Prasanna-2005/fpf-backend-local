package models

import (
	"time"
)

type User struct {
	ID         string    `gorm:"primaryKey;autoIncrement"`
	UserName   string    `gorm:"column:userName;not null"`
	Email      string    `gorm:"column:emailAddress;not null"`
	Password   string    `gorm:"column:password;not null"`
	AccessType string    `gorm:"column:accessType;not null"`
	ClientID   int       `gorm:"column:clientId;not null"`
	Jti        int       `gorm:"column:jwtId;not null"`
	CreatedAt  time.Time `gorm:"column:createdAt;not null"`
	UpdatedBy  string    `gorm:"column:updatedBy"`
	UpdatedAt  time.Time `gorm:"column:updatedAt;not null"`
}

type Session struct {
	ID         uint      `gorm:"primaryKey;autoIncrement"`
	UserID     string    `gorm:"column:userId"`
	ClientID   int       `gorm:"column:clientCode"`
	LoginTime  time.Time `gorm:"column:loginTime"`
	LogoutTime time.Time `gorm:"column:logoutTime;default:null"`
	Jti        int       `gorm:"column:jwtId;default:null "`
}

func (Session) TableName() string {
	return "session_logs"
}

type FilingTracker struct {
	ID          uint   `gorm:"primaryKey;autoIncrement"`
	FilingName  string `gorm:"column:filingName;not null"`
	ClientCode  string `gorm:"column:clientCode;not null"`
	UploadCount int    `gorm:"column:uploadCount;not null;default:0"`
	IsSubmitted bool   `gorm:"column:isSubmitted;not null;default:false"`
	IsDeleted   bool   `gorm:"column:isDeleted;not null;default:false"`
	// UserID      string    `gorm:"column:userId;not null"`    // FK  users.id
	CreatedBy string    `gorm:"column:createdBy;not null"` // FK  users.userName
	UpdatedBy string    `gorm:"column:updatedBy"`
	CreatedAt time.Time `gorm:"column:createdAt;not null;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updatedAt;not null;autoUpdateTime"`
}

func (FilingTracker) TableName() string {
	return "filing_tracker"
}
