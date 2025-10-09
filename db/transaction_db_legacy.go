package db

import "form_pf_backend/models"

/* ---------- meta ---------- */
func FetchFormFieldMSLegacy() ([]models.FormFieldMSLegacy, error) {
	var rows []models.FormFieldMSLegacy
	err := GetDB().Order("formLineNo ASC").Find(&rows).Error
	return rows, err
}

/* ---------- value store ---------- */
func FetchCurrentValue(fundSeriesID uint64, formLineNo string) (string, error) {
	type res struct{ Value string }
	var r res

	// flagged row first
	if err := GetDB().
		Model(&models.FormDataLegacy{}).
		Select("value").
		Where("fundSeriesid = ? AND formLineNo = ? AND isCurrentValue = ?", fundSeriesID, formLineNo, true).
		Scan(&r).Error; err == nil && r.Value != "" {
		return r.Value, nil
	}

	// newest row fallback
	if err := GetDB().
		Model(&models.FormDataLegacy{}).
		Select("value").
		Where("fundSeriesid = ? AND formLineNo = ?", fundSeriesID, formLineNo).
		Order("updatedAt DESC").
		Limit(1).
		Scan(&r).Error; err != nil {
		return "", err
	}
	return r.Value, nil
}

/* ---------- filing → fund-series ---------- */
func FetchFundSeriesForFiling(filingID uint64) ([]uint64, error) {
	type row struct{ FundSeriesid uint64 }
	var rows []row
	err := GetDB().
		Table("form_data_legacy").
		Select("DISTINCT fundSeriesid").
		Where("filingid = ?", filingID).
		Scan(&rows).Error

	out := make([]uint64, len(rows))
	for i, r := range rows {
		out[i] = r.FundSeriesid
	}
	return out, err
}
