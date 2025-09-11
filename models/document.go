package models

import (
	"time"
)

type FileIdentificationRequest struct {
	FilingName string `json:"filing_name" gorm:"-"` // incoming payload only
}

type FileIdentificationResponse struct {
	Message       string       `json:"message,omitempty"        gorm:"-"`
	FilingName    string       `json:"filing_name,omitempty"    gorm:"-"`
	FileMappings  []MappedResp `json:"file_mappings,omitempty"  gorm:"-"`
	UnmappedFiles []string     `json:"unmapped_files,omitempty" gorm:"-"`
	FundDetails   []MappedResp `json:"fund_details,omitempty"  gorm:"-"`
}

type FileMappingDetail struct {
	MapID       int     `json:"Map_ID"  gorm:"column:Map_ID"`
	DateCellNo  int     `json:"date_cell_no"  gorm:"column:dateCellNo"`
	FundCellNo  int     `json:"fund_cell_no"  gorm:"column:fundCellNo"`
	FileName    string  `json:"file_name"     gorm:"column:fileName"`
	FilePath    string  `json:"file_path"     gorm:"column:filePath"`
	FundName    string  `json:"fund_name"     gorm:"column:fundName"`
	ShortCode   string  `json:"short_code"    gorm:"column:shortCode"`
	MatchMethod string  `json:"match_method"  gorm:"column:matchMethod"`
	MatchScore  float64 `json:"match_score"   gorm:"column:matchScore"`
	Quarter     int     `json:"quarter"       gorm:"column:quarterEndDate"`
	EntityFound string  `json:"entity_found"  gorm:"column:entity_found"`
}

type FileMapper struct {
	ID            int       `gorm:"column:id;primaryKey;autoIncrement"`
	FileName      string    `gorm:"column:fileName;not null"`
	FilingPath    string    `gorm:"column:filingPath;not null"`
	FundShortCode string    `gorm:"column:fundShortCode"`
	FilingID      int       `gorm:"column:filingId;not null"` // legacy DB column – keep
	FileMapID     int       `gorm:"column:fileMapId"`
	QuarterNo     int       `gorm:"column:quarterNo"`
	IsUpdated     bool      `gorm:"column:isUpdated;default:false"`
	CreatedAt     time.Time `gorm:"column:createdAt;not null"`
	CreatedBy     string    `gorm:"column:createdBy;not null"`
	UpdatedAt     time.Time `gorm:"column:updatedAt"`
	UpdatedBy     string    `gorm:"column:updatedBy"`

	FegFileIdentificationID uint                  `gorm:"column:feg_file_identification_id"`
	FegFileIdentification   FegFileIdentification `gorm:"foreignKey:FegFileIdentificationID;references:ID"`
}

type FegFileIdentification struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement"`
	ShortCode string    `gorm:"column:shortCode;not null"`
	FundName  string    `gorm:"column:fundName;not null"`
	Quarter   int       `gorm:"column:quarter;not null"`
	Month     string    `gorm:"column:month"`
	Series    string    `gorm:"column:series;not null"`
	CreatedAt time.Time `gorm:"column:createdAt;not null"`
	CreatedBy string    `gorm:"column:createdBy;not null"`
	UpdatedAt time.Time `gorm:"column:updatedAt"`
	UpdatedBy string    `gorm:"column:updatedBy"`
}

func (FileMapper) TableName() string {
	return "filing_mapper"
}

func (FegFileIdentification) TableName() string {
	return "feg_file_identification"
}

type MappedResp struct {
	FileName  string `json:"filename,omitempty"        gorm:"-"`
	ShortCode string `json:"shortcode,omitempty"        gorm:"-"`
	FundName  string `json:"fundname,omitempty"        gorm:"-"`
	Quarter   int    `json:"quarter,omitempty"        gorm:"-"`
	Path      string `json:"filepath,omitempty"        gorm:"-"`
}

type SubmitReq struct {
	UserID       string `json:"user_id"                    binding:"required" gorm:"-"`
	FilingName   string `json:"filing_name"                binding:"required" gorm:"-"`
	NewFiling    bool   `json:"new_filing"                              gorm:"-"`
	Timestamp    string `json:"timestamp"                  binding:"required" gorm:"-"`
	FileMappings []struct {
		Filename  string `json:"filename"  binding:"required" gorm:"-"`
		Fundname  string `json:"fundname"  binding:"required" gorm:"-"`
		Quarter   string `json:"quarter"   binding:"required" gorm:"-"`
		Shortcode string `json:"shortcode" binding:"required" gorm:"-"`
		Filepath  string `json:"filepath"  binding:"required" gorm:"-"`
	} `json:"file_mappings" binding:"required,dive,min=1" gorm:"-"`
}
