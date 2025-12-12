package models

import (
	"time"
)

// Fund maps to the table that stores fund information.
type Fund struct {
	FundName  string    `gorm:"column:fundName;not null"`
	FundID    string    `gorm:"column:fegFundId;not null"`
	NfaID     string    `gorm:"column:nfaId;not null"`
	Series    string    `gorm:"column:series;not null"`
	FundType  string    `gorm:"column:fundType;not null"`
	CreatedAt time.Time `gorm:"column:createdAt;not null"`
	UpdatedBy string    `gorm:"column:updatedBy"`
	UpdatedAt time.Time `gorm:"column:updatedAt;not null"`
}

// TableName overrides the default table name.
func (Fund) TableName() string {
	return "feg_funds"
}
