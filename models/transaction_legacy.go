package models

import (
	"time"
)

type FormDataLegacy struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement,column:id"`
	FilingID       string    `gorm:"column:filingId"`
	FieldNo        string    `gorm:"column:fieldNo"`
	FundSeriesid   string    `gorm:"column:fundSeriesId"`
	FormLineNo     string    `gorm:"column:formLineNo"`
	Value          string    `gorm:"column:value"`
	CreatedBy      string    `gorm:"column:createdBy"`
	IsCurrentValue bool      `gorm:"column:isCurrentValue"`
	UpdatedAt      time.Time `gorm:"column:updatedAt"`
	UpdatedBy      string    `gorm:"column:updatedBy"`
	CreatedAt      time.Time `gorm:"column:createdAt"`
}

func (FormDataLegacy) TableName() string { return "form_data_legacy" }

type FormFieldMSLegacy struct {
	FieldNo        string `gorm:"column:fieldNo"`
	FormLineItemNo string `gorm:"column:formLineNo"`
	LineText       string `gorm:"column:lineText"`
	DataType       string `gorm:"column:dataType"`
	IsMultipleAdd  bool   `gorm:"column:isMultipleAdd"`
	RequiresDate   bool   `gorm:"column:requiresDate"`
	IsTableData    bool   `gorm:"column:isTableData"`
	DisclaimerText string `gorm:"column:disclaimerText"`
}

func (FormFieldMSLegacy) TableName() string { return "form_fields_ms_legacy" }

// FilingResponse is the top-level envelope sent back to the UI.
type FilingResponselegacy struct {
	Funds []Fundlegacy `json:"funds"`
}

// Fund represents one fund-series block inside the funds array.
type Fundlegacy struct {
	FundID   string      `json:"fund_id"`
	FundName string      `json:"fund_name"`
	FundData []FieldItem `json:"fund_data"`
}

// FieldItem mirrors the JSON block the frontend expects for every field_no.
type FieldItem struct {
	FieldNo          string       `json:"field_no"`
	HasFieldItemText bool         `json:"has_field_item_text"`
	FieldItemType    string       `json:"field_item_type"`
	HasTableData     bool         `json:"has_table_data"`
	TableData        interface{}  `json:"table_data,omitempty"`
	FieldText        string       `json:"field_text"`
	FieldHeading     string       `json:"field_heading"`
	Value            string       `json:"value"`
	FieldData        []LineDetail `json:"field_data"`
	Confirmed        bool         `json:"confirmed"`
	Editing          bool         `json:"editing"`
	IsMultipleAdd    bool         `json:"is_multiple_add"`
	DisclaimerText   string       `json:"disclaimer_text"`
	Options          []string     `json:"options,omitempty"` // filled only when datatype = options
}

// LineDetail is used inside FieldData when the field is broken into sub-lines.
type LineDetail struct {
	LineItemNo     string      `json:"line_item_no,omitempty"`
	LineItemType   string      `json:"line_item_type,omitempty"`
	LineText       string      `json:"line_text,omitempty"`
	Value          string      `json:"value"`
	Options        []string    `json:"options,omitempty"`
	HasTableData   bool        `json:"has_table_data"`
	TableData      interface{} `json:"table_data,omitempty"`
	RequireDate    bool        `json:"require_date,omitempty"`
	DateValue      string      `json:"date_value,omitempty"`
	DisclaimerText string      `json:"disclaimer_text,omitempty"`
}
