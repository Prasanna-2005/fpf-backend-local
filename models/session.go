package models

import (
	"time"
)

type OTP struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	UserID    uint      `gorm:"not null;foreignKey:ID;references:users"`
	OTPCode   string    `gorm:"column:OTP;not null"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;not null"`
}
