package models

import (
	"time"
)

type Client struct {
	ID         uint       `gorm:"primaryKey;autoIncrement;column:id"`
	ClientCode string     `gorm:"column:clientCode;type:varchar(20);not null"`
	CreatedAt  time.Time  `gorm:"column:createdAt;not null"`
	CreatedBy  uint       `gorm:"column:createdBy;not null"` // or string if you store usernames
	UpdatedAt  *time.Time `gorm:"column:updatedAt"`
	UpdatedBy  *uint      `gorm:"column:updatedBy"` // pointer allows NULL
}

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

func (Client) TableName() string {
	return "clients"
}
