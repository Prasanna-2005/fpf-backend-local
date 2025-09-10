package models

import (
	"time"
)

type FileIdentificationRequest struct {
	ClientID   string `gorm:"column:client_id"`
	FilingName string `gorm:"column:filing_name"`
}

type FileIdentificationResponse struct {
	Success        bool                `gorm:"column:success"`
	Message        string              `gorm:"column:message"`
	FileMappings   []FileMappingDetail `gorm:"-"` // slice ignored by GORM
	UnmappedFiles  []string            `gorm:"-"`
	ProcessedCount int                 `gorm:"column:processed_count"`
	MappedCount    int                 `gorm:"column:mapped_count"`
}

type FileMappingDetail struct {
	DateCellNo     int     `gorm:"column:dateCellNo"`
	FundCellNo     int     `gorm:"column:fundCellNo"`
	FileName       string  `gorm:"column:fileName"`
	FilePath       string  `gorm:"column:filePath"`
	FundName       string  `gorm:"column:fundName"`
	ShortCode      string  `gorm:"column:shortCode"`
	MatchMethod    string  `gorm:"column:matchMethod"`
	MatchScore     float64 `gorm:"column:matchScore"`
	QuarterEndDate int     `gorm:"column:quarterEndDate"`
	EntityFound    string  `gorm:"column:entity_found"`
}

type FileMapper struct {
	ID            uint      `gorm:"column:id;primaryKey;autoIncrement"`
	FileName      string    `gorm:"column:fileName;not null"`
	FilingPath    string    `gorm:"column:filingPath;not null"`
	FundShortCode string    `gorm:"column:fundShortCode"`
	FilingID      uint      `gorm:"column:filingId;not null"`
	FileMapID     uint      `gorm:"column:fileMapId"`
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
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement"`
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
