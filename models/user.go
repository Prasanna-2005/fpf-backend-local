package models

import (
	"time"
)

type User struct {
	ID           uint      `gorm:"primaryKey;not null"`
	UserName     string    `gorm:"column:userName;not null"`
	EmailAddress string    `gorm:"column:emailAddress;not null"`
	Password     string    `gorm:"column:password;not null"`
	AccessType   string    `gorm:"column:accessType;not null"`
	ClientID     uint      `gorm:"column:clientId;not null"`
	ClientCode   string    `gorm:"column:clientCode;not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedBy    string    `gorm:"column:updatedBy"`
	UpdatedAt    time.Time
}
