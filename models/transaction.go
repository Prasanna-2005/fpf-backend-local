package models

import (
	"time"

	"gorm.io/datatypes"
)

/* ---------- GORM tables (exact columns you gave) ---------- */
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
	// NEW UI columns
	HasFieldItemText bool           `gorm:"column:has_field_item_text"`
	HasTableData     bool           `gorm:"column:has_table_data"`
	FieldHeading     string         `gorm:"column:field_heading"`
	Options          datatypes.JSON `gorm:"column:options"`
	RequireDate      bool           `gorm:"column:require_date"`
	XmlTags          datatypes.JSON `gorm:"column:xmlTags"`
}

type FormDatum struct {
	ID             uint64     `gorm:"column:id;primaryKey"`
	FilingID       uint64     `gorm:"column:filingId;not null"`
	FieldNo        string     `gorm:"column:fieldNo;type:varchar(255);not null"` // typo matches DB
	FormLineNo     string     `gorm:"column:formLineNo;type:varchar(255);not null"`
	Value          *string    `gorm:"column:value;type:varchar(255)"` // nullable
	IsMultipleAdd  bool       `gorm:"column:isMultipleAdd"`
	CreatedAt      time.Time  `gorm:"column:createdAt;not null;autoCreateTime"`
	CreatedBy      uint64     `gorm:"column:createdBy;not null"`
	UpdatedAt      *time.Time `gorm:"column:updatedAt;autoUpdateTime"` // nullable
	UpdatedBy      *int64     `gorm:"column:updatedBy"`                // nullable
	IsCurrentValue bool       `gorm:"column:isCurrentValue"`
	FundID         string     `gorm:"column:fundSeriesId"`
	FundName       string     `gorm:"column:fundName"`
	FundType       string     `gorm:"column:fundType"`

	LineItemType     string         `gorm:"column:dataType"`
	LineText         string         `gorm:"column:lineText"`
	HasFieldItemText bool           `gorm:"column:has_field_item_text"`
	HasTableData     bool           `gorm:"column:has_table_data"`
	FieldHeading     string         `gorm:"column:field_heading"`
	Options          datatypes.JSON `gorm:"column:options"`
	RequireDate      bool           `gorm:"column:require_date"`
	DisclaimerText   string         `gorm:"column:disclaimer_text"`
	TableData        datatypes.JSON `gorm:"column:table_data"`
	DateValue        datatypes.JSON `gorm:"column:date_value"`
	IsMSQ            bool           `gorm:"column:isMSQ"`
	IsMCQ            bool           `gorm:"column:isMCQ"`
	XmlTags          string 		`gorm:"column:xmlTags"`
}

func (FormField) TableName() string { return "formpf.form_fields_ms" }
func (FormDatum) TableName() string { return "formpf.form_data" }

// type FilingResponse struct {
// 	FundID   string          `json:"fund_id"`
// 	FundName string          `json:"fund_name"` // renamed
// 	FundData []ResponseField `json:"fund_data"` // renamed
// }

type ResponseField struct {
	FieldNo          string             `json:"field_no"`
	HasFieldItemText bool               `json:"has_field_item_text"`
	HasTableData     bool               `json:"has_table_data"`
	FieldText        string             `json:"field_text"`
	FieldHeading     string             `json:"field_heading"`
	Value            interface{}        `json:"value"` // polymorphic
	FieldData        []ResponseLineItem `json:"field_data"`
	Confirmed        bool               `json:"confirmed"` // always false
	Editing          bool               `json:"editing"`   // always true
	IsMultipleAdd    bool               `json:"is_multiple_add"`
}

type ResponseLineItem struct {
	LineItemNo     string                   `json:"line_item_no"`
	LineItemType   string                   `json:"line_item_type"`
	LineText       string                   `json:"line_text"`
	Value          interface{}              `json:"value"` // polymorphic
	Options        []string                 `json:"options"`
	TableData      []map[string]interface{} `json:"table_data"`
	RequireDate    bool                     `json:"require_date"`
	DateValue      interface{}              `json:"date_value"` // polymorphic
	DisclaimerText string                   `json:"disclaimer_text"`
}

type FilingResponse struct {
	Funds []FundEntry `json:"funds"`
}

type FundEntry struct {
	FundID   string          `json:"fund_id"`
	FundName string          `json:"fund_name"`
	FundData []ResponseField `json:"fund_data"`
}
