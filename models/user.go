package models

import (
	"time"
)

type User struct {
	ID         string    `gorm:"primaryKey;not null"`
	UserName   string    `gorm:"column:userName;not null"`
	Email      string    `gorm:"column:emailAddress;not null"`
	Password   string    `gorm:"column:password;not null"`
	AccessType string    `gorm:"column:accessType;not null"`
	ClientID   uint      `gorm:"column:clientId;not null"`
	Jti        int       `gorm:"column:jwtId;not null"`
	CreatedAt  time.Time `gorm:"column:createdAt;not null"`
	UpdatedBy  string    `gorm:"column:updatedBy"`
	UpdatedAt  time.Time `gorm:"column:updatedAt;not null"`
}

type Session struct {
	ID         uint      `gorm:"primaryKey;autoIncrement"`
	UserID     string    `gorm:"column:userId"`
	ClientID   uint      `gorm:"column:clientCode"`
	LoginTime  time.Time `gorm:"column:loginTime"`
	LogoutTime time.Time `gorm:"column:logoutTime;default:null"`
	Jti        int       `gorm:"column:jwtId;default:null "`
}

func (Session) TableName() string {
	return "session_logs"
}
