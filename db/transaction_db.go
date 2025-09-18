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
	all, _ := GetAllFunds() // existing func
	m := make(map[string]fundBrief, len(all))
	for _, f := range all {
		m[f.ID] = fundBrief{FundID: f.FundID, FundName: f.FundName}
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
		Table("formpf.form_data").    // real table name only
		Select("formpf.form_data.*"). // or simply "form_data.*"
		Joins("JOIN feg_funds f ON f.fegFundId = formpf.form_data.fundSeriesId").
		Joins("JOIN form_fields_ms fld ON fld.fieldNo = formpf.form_data.fieldNo AND fld.formLineNo = formpf.form_data.formLineNo").
		Where("formpf.form_data.filingId = ?", filingID). // typo: filingld → filingId
		Find(&rows).Error
	return rows, err
}
