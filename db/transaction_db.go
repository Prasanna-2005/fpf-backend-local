package db

import (
	"form_pf_backend/models"
	"sync"

	"gorm.io/gorm"
)

/* ---------- cached fund header map ---------- */
var (
	fundCache     map[string]fundBrief
	onceFundCache sync.Once
)

type fundBrief struct {
	FundID   string
	FundName string
}

func loadFundCache() {
	all, _ := GetAllFunds()
	m := make(map[string]fundBrief, len(all))
	for _, f := range all {
		m[f.FundID] = fundBrief{FundID: f.FundID, FundName: f.FundName}
	}
	fundCache = m
}

// GetFundBrief returns fundID & fundName for a fundSeriesId
func GetFundBrief(seriesID string) (fundID, fundName string, err error) {
	onceFundCache.Do(loadFundCache)
	b, ok := fundCache[seriesID]
	if !ok {
		return "", "", gorm.ErrRecordNotFound
	}
	return b.FundID, b.FundName, nil
}

func FetchFormDataByFiling(filingID uint64) ([]models.FormDatum, error) {
	var rows []models.FormDatum
	err := GetDB().
		Table("form_data d"). // alias declared here
		Select(`d.createdAt, d.createdBy, d.updatedAt, d.updatedBy, d.value,
		        f.fegFundId, f.fundName, f.fundType,
		        fld.fieldNo, fld.formLineNo, fld.lineText, fld.dataType,
		        fld.isMultipleAdd, fld.isMSQ, fld.isMCQ, fld.requiresDate,
		        fld.isTableData, fld.disclaimerText, fld.xmlTags`).
		Joins("JOIN feg_funds f ON f.fegFundId = d.fundSeriesId").
		Joins("JOIN form_fields_ms fld ON fld.fieldNo = d.fieldNo AND fld.formLineNo = d.formLineNo").
		Where("d.filingId = ? AND d.isCurrentValue = 1", filingID).
		Limit(400).
		Find(&rows).Error
	return rows, err
}
