package models

import "time"

type FormField struct {
	ID             uint64     `gorm:"column:id;primaryKey;autoIncrement"`
	FieldNo        string     `gorm:"column:fieldNo;type:varchar(255);not null"`
	FormLineNo     string     `gorm:"column:formLineNo;type:varchar(255);not null"`
	LineText       string     `gorm:"column:lineText;type:text;not null"`
	DataType       string     `gorm:"column:dataType;type:varchar(255);not null"`
	IsMultipleAdd  bool       `gorm:"column:isMultipleAdd;not null;default:0"`
	IsMSQ          bool       `gorm:"column:isMSQ;not null;default:0"`
	IsMCQ          bool       `gorm:"column:isMCQ;not null;default:0"`
	RequiresDate   bool       `gorm:"column:requiresDate;not null;default:0"`
	IsTableData    bool       `gorm:"column:isTableData;not null;default:0"`
	DisclaimerText *string    `gorm:"column:disclaimerText;type:text"` // nullable
	CreatedAt      time.Time  `gorm:"column:createdAt;not null;autoCreateTime"`
	CreatedBy      int64      `gorm:"column:createdBy;not null"`
	UpdatedAt      *time.Time `gorm:"column:updatedAt;autoUpdateTime"` // nullable
	UpdatedBy      *int64     `gorm:"column:updatedBy"`                // nullable
}

type FormDatum struct {
	ID             uint64     `gorm:"column:id;primaryKey;autoIncrement"`
	FilingID       uint64     `gorm:"column:filingld;not null"` // typo matches screenshot
	FundSeriesID   string     `gorm:"column:fundSeriesId;type:varchar(50);not null"`
	FieldNo        string     `gorm:"column:feldNo;type:varchar(255);not null"` // typo matches screenshot
	FormLineNo     string     `gorm:"column:formLineNo;type:varchar(255);not null"`
	Value          *string    `gorm:"column:value;type:varchar(255)"` // nullable
	CreatedAt      time.Time  `gorm:"column:createdAt;not null;autoCreateTime"`
	CreatedBy      uint64     `gorm:"column:createdBy;not null"`
	IsCurrentValue bool       `gorm:"column:isCurrentValue;not null;default:1"`
	UpdatedAt      *time.Time `gorm:"column:updatedAt;autoUpdateTime"` // nullable
	UpdatedBy      *int64     `gorm:"column:updatedBy"`                // nullable
}

func (FormDatum) TableName() string { return "formpf.form_data" }

func (FormField) TableName() string { return "formpf.form_fields_ms" }
