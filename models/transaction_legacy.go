package models

import (
	"encoding/json"
	"strings"
	"time"
)

type FormDataLegacy struct {
	ID             string    `gorm:"primaryKey;autoIncrement,column:id"`
	FilingID       string    `gorm:"column:filingId"`
	FieldNo        string    `gorm:"column:fieldNo"`
	FundSeriesid   string    `gorm:"column:fundSeriesId"`
	FormLineNo     string    `gorm:"column:formLineNo"`
	Value          string    `gorm:"column:value"`
	SourceFiles    string    `gorm:"column:sourceFiles"`
	CreatedBy      string    `gorm:"column:createdBy"`
	IsCurrentValue bool      `gorm:"column:isCurrentValue"`
	UpdatedAt      time.Time `gorm:"column:updatedAt"`
	UpdatedBy      string    `gorm:"column:updatedBy"`
	CreatedAt      time.Time `gorm:"column:createdAt"`
}

func (FormDataLegacy) TableName() string { return "form_data_legacy" }

type FieldHistoryRow struct {
	FundSeriesid   string    `gorm:"column:fundSeriesId"`
	FieldNo        string    `gorm:"column:fieldNo"`
	FormLineNo     string    `gorm:"column:formLineNo"`
	LineText       string    `gorm:"column:lineText"`
	Value          string    `gorm:"column:value"`
	SourceFiles    string    `gorm:"column:sourceFiles"`
	IsCurrentValue bool      `gorm:"column:isCurrentValue"`
	CreatedAt      time.Time `gorm:"column:createdAt"`
	CreatedBy      string    `gorm:"column:createdBy"`
	UpdatedAt      time.Time `gorm:"column:updatedAt"`
	UpdatedByID    string    `gorm:"column:updatedById"`
	UpdatedBy      string    `gorm:"column:updatedBy"`
}

// ParseSourceFiles decodes the JSON array stored on form_data_legacy.sourceFiles.
// Invalid JSON returns an empty list and the decode error so callers can log it.
func ParseSourceFiles(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return []string{}, nil
	}
	var files []string
	if err := json.Unmarshal([]byte(raw), &files); err != nil {
		return []string{}, err
	}
	if files == nil {
		return []string{}, nil
	}
	return files, nil
}

type FormFieldMSLegacy struct {
	FieldNo        string `gorm:"column:fieldNo"`
	FormLineItemNo string `gorm:"column:formLineNo"`
	LineText       string `gorm:"column:lineText"`
	DataType       string `gorm:"column:dataType"`
	IsMCQ          bool   `gorm:"column:isMCQ"`
	IsMultipleAdd  bool   `gorm:"column:isMultipleAdd"`
	RequiresDate   bool   `gorm:"column:requiresDate"`
	IsTableData    bool   `gorm:"column:isTableData"`
	DisclaimerText string `gorm:"column:disclaimerText"`
	DisplayAffix   string `gorm:"column:displayAffix"`
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

// TableColumnMetadata holds per-column metadata for table fields.
type TableColumnMetadata struct {
	DataType     string `json:"data_type"`
	DisplayAffix string `json:"display_affix"`
}

// FieldItem mirrors the JSON block the frontend expects for every field_no.
type FieldItem struct {
	FieldNo          string                         `json:"field_no"`
	HasFieldItemText bool                           `json:"has_field_item_text"`
	FieldItemType    string                         `json:"field_item_type"`
	HasTableData     bool                           `json:"has_table_data"`
	TableMetadata    map[string]TableColumnMetadata `json:"table_metadata,omitempty"`
	TableData        interface{}                    `json:"table_data,omitempty"`
	TableHeaders     []string                       `json:"table_headers,omitempty"`
	FieldText        string                         `json:"field_text"`
	FieldHeading     string                         `json:"field_heading"`
	Value            string                         `json:"value" binding:"max=65535"`
	FieldData        []LineDetail                   `json:"field_data"`
	Confirmed        bool                           `json:"confirmed"`
	Editing          bool                           `json:"editing"`
	IsMultipleAdd    bool                           `json:"is_multiple_add"`
	DisclaimerText   string                         `json:"disclaimer_text"`
	DisplayAffix     string                         `json:"display_affix"`
	Options          []string                       `json:"options,omitempty"` // filled only when datatype = options
}

// LineDetail is used inside FieldData when the field is broken into sub-lines.
type LineDetail struct {
	LineItemNo      string                         `json:"line_item_no,omitempty"`
	LineItemType    string                         `json:"line_item_type,omitempty"`
	HasLineItemText bool                           `json:"has_line_item_text"`
	LineText        string                         `json:"line_text,omitempty"`
	Value           string                         `json:"value" binding:"max=65535"`
	Options         []string                       `json:"options,omitempty"`
	HasTableData    bool                           `json:"has_table_data"`
	TableData       interface{}                    `json:"table_data,omitempty"`
	TableMetadata   map[string]TableColumnMetadata `json:"table_metadata,omitempty"`
	TableHeaders    []string                       `json:"table_headers,omitempty"`
	RequireDate     bool                           `json:"require_date,omitempty"`
	DateValue       string                         `json:"date_value,omitempty"`
	DisclaimerText  string                         `json:"disclaimer_text,omitempty"`
	DisplayAffix    string                         `json:"display_affix"`
}

// FilingResponse is the top-level envelope sent back to the UI.
type ComputationLogsLegacy struct {
	Funds []FundlegacyComputation `json:"funds"`
}

// Fund represents one fund-series block inside the funds array.
type FundlegacyComputation struct {
	FundID   string             `json:"fund_id"`
	FundName string             `json:"fund_name"`
	FundComp []FieldComputation `json:"fund_computation"`
}

// FieldItem mirrors the JSON block the frontend expects for every field_no.
type FieldComputation struct {
	FieldNo          string            `json:"field_no"`
	FieldItemType    string            `json:"field_item_type"`
	HasFieldItemText bool              `json:"has_field_item_text"`
	FieldText        string            `json:"field_text"`
	HasTableData     bool              `json:"has_table_data"`
	TableMetadata    map[string]string `json:"table_metadata,omitempty"`
	TableData        interface{}       `json:"table_data,omitempty"`
	Type             string            `json:"computation_type"`
	Formula          string            `json:"computation_formula"`
	FieldData        []LineComputation `json:"field_data"`
}

// LineDetail is used inside FieldData when the field is broken into sub-lines.
type LineComputation struct {
	LineItemNo      string      `json:"line_item_no,omitempty"`
	LineItemType    string      `json:"line_item_type,omitempty"`
	HasLineItemText bool        `json:"has_line_item_text"`
	LineText        string      `json:"line_text,omitempty"`
	Type            string      `json:"computation_type"`
	Formula         string      `json:"computation_formula"`
	HasTableData    bool        `json:"has_table_computation"`
	TableData       interface{} `json:"table_computation,omitempty"`
}

type EditorCache struct {
	FilingID     string    `gorm:"column:filingId"`
	FieldNo      string    `gorm:"column:fieldNo"`
	FundSeriesid string    `gorm:"column:fundSeriesId"`
	FormLineNo   string    `gorm:"column:formLineNo"`
	Value        string    `gorm:"column:value"`
	CreatedBy    string    `gorm:"column:createdBy"`
	CreatedAt    time.Time `gorm:"column:createdAt"`
	Username     string    `gorm:"column:userName"`
}
