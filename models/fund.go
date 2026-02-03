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

type CompLogs struct {
	Id              string  `gorm:"column:id;type:char(36);primaryKey;default:UUID()"`
	FieldNo         string  `gorm:"column:fieldNo;type:varchar(255)"`
	FormLineNo      string  `gorm:"column:formlineNo;type:varchar(255)"`
	ClientCode      string  `gorm:"column:clientCode;type:varchar(255)"`
	Series          string  `gorm:"column:series;type:varchar(255)"`
	FundID          string  `gorm:"column:fundId;type:varchar(255)"`
	ComputationType *string `gorm:"column:computationType;type:varchar(255)"`
	StaticData      string  `gorm:"column:staticData;type:varchar(255)"`
	GroupOperation  string  `gorm:"column:filters;type:varchar(255)"`
	InfixExpression string  `gorm:"column:InfixExpression;type:text"`
}

// TableName overrides the default table name.
func (Fund) TableName() string {
	return "feg_funds"
}

func (CompLogs) TableName() string {
	return "computation_formula"
}
