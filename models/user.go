package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	UserName     string    `gorm:"not null"`
	EmailAddress string    `gorm:"not null"`
	Password     string    `gorm:"not null"`
	AccessType   string    `gorm:"not null"`
	ClientCode   string    `gorm:"not null"`
	OTPHash      string    `gorm:"not null"`
	OTPExpiry    time.Time `gorm:"not null"`
}
