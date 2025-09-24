package models

import (
	"time"

	"gorm.io/datatypes"
)

// FormField  (table: form_fields_ms)
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
	// UI helpers
	HasFieldItemText bool           `gorm:"column:has_field_item_text"`
	HasTableData     bool           `gorm:"column:has_table_data"`
	FieldHeading     string         `gorm:"column:field_heading"`
	Options          datatypes.JSON `gorm:"column:options"`
	RequireDate      bool           `gorm:"column:require_date"`
	XmlTags          datatypes.JSON `gorm:"column:xmlTags"`
}

// FormDatum  (table: form_data)  – NEW nullable column added
type FormDatum struct {
	ID               uint64         `gorm:"column:id;primaryKey"                           json:"id"`
	FilingID         uint64         `gorm:"column:filingId;not null"                       json:"filing_id"`
	FieldNo          string         `gorm:"column:fieldNo;type:varchar(255);not null"      json:"field_no"`
	FormLineNo       string         `gorm:"column:formLineNo;type:varchar(255);not null"   json:"form_line_no"`
	Value            *string        `gorm:"column:value;type:varchar(255)"                 json:"value,omitempty"`
	IsMultipleAdd    bool           `gorm:"column:isMultipleAdd"                            json:"is_multiple_add"`
	CreatedAt        time.Time      `gorm:"column:createdAt;not null;autoCreateTime"       json:"created_at"`
	CreatedBy        uint64         `gorm:"column:createdBy;not null"                      json:"created_by"`
	UpdatedAt        *time.Time     `gorm:"column:updatedAt;autoUpdateTime"                json:"updated_at,omitempty"`
	UpdatedBy        *int64         `gorm:"column:updatedBy"                               json:"updated_by,omitempty"`
	IsCurrentValue   bool           `gorm:"column:isCurrentValue"                          json:"is_current_value"`
	FundID           string         `gorm:"column:fundSeriesId"                            json:"fund_series_id"`
	FundName         string         `gorm:"column:fundName"                                json:"fund_name"`
	FundType         string         `gorm:"column:fundType"                                json:"fund_type"`
	LineItemType     string         `gorm:"column:dataType"                                json:"line_item_type"`
	LineText         string         `gorm:"column:lineText"                                json:"line_text"`
	HasFieldItemText bool           `gorm:"column:has_field_item_text"                     json:"has_field_item_text"`
	HasTableData     bool           `gorm:"column:isTableData"                          json:"has_table_data"`
	FieldHeading     string         `gorm:"column:fieldHeading"                            json:"field_heading"`
	Options          datatypes.JSON `gorm:"column:options"                                 json:"options,omitempty"`
	RequireDate      bool           `gorm:"column:requiresDate"                             json:"require_date"`
	DisclaimerText   string         `gorm:"column:disclaimerText"                         json:"disclaimer_text,omitempty"`
	TableData        datatypes.JSON `gorm:"column:table_data"                              json:"table_data,omitempty"`
	DateValue        datatypes.JSON `gorm:"column:date"                              json:"date_value,omitempty"`
	IsMSQ            bool           `gorm:"column:isMSQ"                                   json:"is_msq"`
	IsMCQ            bool           `gorm:"column:isMCQ"                                   json:"is_mcq"`
	XmlTags          datatypes.JSON `gorm:"column:xmlTags"                                 json:"xml_tags,omitempty"`
	Date             *string        `gorm:"column:date"                                    json:"date,omitempty"`
}
type ResponseField struct {
	FieldNo          string             `json:"field_no"`
	HasFieldItemText bool               `json:"has_field_item_text"`
	FieldItemType    string             `json:"field_item_type,omitempty"`
	HasTableData     bool               `json:"has_table_data"`
	FieldText        string             `json:"field_text"`
	FieldHeading     string             `json:"field_heading"`
	Value            interface{}        `json:"value"`
	FieldData        []ResponseLineItem `json:"field_data"`
	Confirmed        bool               `json:"confirmed"`
	Editing          bool               `json:"editing"`
	IsMultipleAdd    bool               `json:"is_multiple_add"`
	DisclaimerText   string             `json:"disclaimer_text"` // <-- NEW
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

func (FormField) TableName() string { return "formpf.form_fields_ms" }
func (FormDatum) TableName() string { return "formpf.form_data" }
