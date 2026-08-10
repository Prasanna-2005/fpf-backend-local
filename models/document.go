package models

import (
	"time"
)

type FileIdentificationRequest struct {
	FilingName string `json:"filing_name" gorm:"-"` // incoming payload only
	Filing     *bool  `json:"filing" gorm:"-"`
}

type FileIdentificationResponse struct {
	Message       string       `json:"message,omitempty"        gorm:"-"`
	Series        string       `json:"series,omitempty"        gorm:"-"`
	FilingName    string       `json:"filing_name,omitempty"    gorm:"-"`
	FileMappings  []MappedResp `json:"file_mappings,omitempty"  gorm:"-"`
	UnmappedFiles []MappedResp `json:"unmapped_files," `
	FundDetails   []MappedResp `json:"fund_details,omitempty"  gorm:"-"`
}

type FileMappingDetail struct {
	MapID       string `json:"Map_ID"  gorm:"column:Map_ID"`
	DateCellNo  int    `json:"date_cell_no"  gorm:"column:dateCellNo"`
	FundCellNo  int    `json:"fund_cell_no"  gorm:"column:fundCellNo"`
	FileName    string `json:"file_name"     gorm:"column:fileName"`
	FilePath    string `json:"file_path"     gorm:"column:filePath"`
	FundName    string `json:"fund_name"     gorm:"column:fundName"`
	ShortCode   string `json:"short_code"    gorm:"column:shortCode"`
	MatchMethod string `json:"match_method"  gorm:"column:matchMethod"`
	// MatchScore  float64 `json:"match_score"   gorm:"column:matchScore"`
	Quarter  int    `json:"quarter"       gorm:"column:quarterEndDate"`
	FileType string `json:"file_type,omitempty" gorm:"column:fileType;default:NULL"`
}

type FileMapper struct {
	ID            string    `gorm:"column:id;primaryKey;autoIncrement"`
	FileName      string    `gorm:"column:fileName;not null"`
	FilingPath    string    `gorm:"column:filingPath;not null"`
	FundShortCode string    `gorm:"column:fundShortCode"`
	FilingID      string    `gorm:"column:filingId;not null"` // legacy DB column – keep
	FileMapID     string    `gorm:"column:fileMapId"`
	Quarter       string    `gorm:"column:quarterNo;default:NULL"`
	Period        string    `gorm:"column:period;default:NULL"`
	IsUpdated     bool      `gorm:"column:isUpdated;default:false"`
	CreatedAt     time.Time `gorm:"column:createdAt;not null"`
	CreatedBy     string    `gorm:"column:createdBy;not null"`
	UpdatedAt     time.Time `gorm:"column:updatedAt;default:NULL"`
	UpdatedBy     string    `gorm:"column:updatedBy;default:NULL"`

	// FegFileIdentificationID uint                  `gorm:"column:feg_file_identification_id"`
	// FegFileIdentification   FegFileIdentification `gorm:"foreignKey:FegFileIdentificationID;references:ID"`
}

type SubmitReq struct {
	Series       string       `json:"series"      binding:"required" gorm:"-"`
	UserID       string       `json:"user_id"      binding:"required" gorm:"-"`
	FilingName   string       `json:"filing_name"  binding:"required" gorm:"-"`
	NewFiling    bool         `json:"new_filing"                gorm:"-"`
	Timestamp    string       `json:"timestamp"    binding:"required" gorm:"-"`
	FileMappings []MappedResp `json:"file_mappings" binding:"min=1,dive" gorm:"-"`
}
type FegFileIdentification struct {
	ID        string    `gorm:"column:id;primaryKey;autoIncrement"`
	ShortCode string    `gorm:"column:shortCode;not null"`
	FundName  string    `gorm:"column:fundName;not null"`
	FileType  string    `gorm:"column:filetype;not null"`
	Quarter   int       `gorm:"column:quarter;not null"`
	Period    *string   `gorm:"column:period"`
	Series    string    `gorm:"column:series;not null"`
	CreatedAt time.Time `gorm:"column:createdAt;not null"`
	CreatedBy string    `gorm:"column:createdBy;not null"`
	UpdatedAt time.Time `gorm:"column:updatedAt"`
	UpdatedBy string    `gorm:"column:updatedBy"`
}

func (FileMapper) TableName() string {
	return "file_mapper"
}

func (FegFileIdentification) TableName() string {
	return "feg_file_identification"
}

type MappedResp struct {
	FileName    string `json:"filename,omitempty"`
	VolFileName string `json:"volfilename,omitempty"`
	ShortCode   string `json:"shortcode,omitempty"`
	FundName    string `json:"fundname,omitempty"`
	FileType    string `json:"quarter"`
	Period      string `json:"period,omitempty"`
	Path        string `json:"filepath,"`
}
