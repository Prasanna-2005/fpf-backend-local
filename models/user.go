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
	JwtId      string    `gorm:"column:jwtId;not null"`
	ClientID   uint      `gorm:"column:clientId;not null"`
	Jti        string    `gorm:"column:jwtId;not null"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedBy  string    `gorm:"column:updatedBy"`
	UpdatedAt  time.Time
}

type Session struct {
	ID         uint      `gorm:"primaryKey;autoIncrement"`
	UserID     string    `gorm:"column:userId"`
	ClientCode string    `gorm:"column:clientCode"`
	LoginTime  time.Time `gorm:"column:loginTime"`
	LogoutTime time.Time `gorm:"column:logoutTime;default:null"`
	Jti        string    `gorm:"column:jwtId;default:null "`
}

type Token struct {
	ID           uint      `gorm:"primaryKey;column:id"`
	UserID       string    `gorm:"not null;column:userId"`
	ClientCode   string    `gorm:"size:20;not null;column:clientCode"`
	AccessToken  string    `gorm:"type:varchar(255);not null;column:accessToken"`
	RefreshToken string    `gorm:"type:varchar(255);not null;column:refreshToken"`
	IsActive     bool      `gorm:"not null;column:isActive"`
	CreatedAt    time.Time `gorm:"not null;column:createdAt"`
	UpdatedAt    time.Time `gorm:"not null;column:updatedAt"`
}

func (Session) TableName() string {
	return "session_logs"
}

func (Token) TableName() string {
	return "session_management"
}
