package db

import (
	"form_pf_backend/models"
)

/* ---------- meta ---------- */
func FetchFormFieldMSLegacy() ([]models.FormFieldMSLegacy, error) {
	var rows []models.FormFieldMSLegacy
	err := GetDB().Order("formLineNo ASC").Find(&rows).Error
	return rows, err
}

func FetchCurrentValuesForFiling(filingID uint64) (map[string]map[string]string, error) {
	type skinny struct {
		FundSeriesid string `gorm:"column:fundSeriesid"`
		FieldNo      string `gorm:"column:fieldNo"`
		FormLineNo   string `gorm:"column:formLineNo"`
		Value        string `gorm:"column:value"`
	}
	var rows []skinny

	err := GetDB().
		Raw(`SELECT fundSeriesid, fieldNo, formLineNo, value
		     FROM form_data_legacy
		     WHERE filingid = ? AND isCurrentValue = ?`, filingID, true).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make(map[string]map[string]string)
	for _, r := range rows {
		if out[r.FundSeriesid] == nil {
			out[r.FundSeriesid] = make(map[string]string)
		}
		out[r.FundSeriesid][r.FormLineNo] = r.Value
	}
	return out, nil
}

/* ---------- filing → fund-series ---------- */
func FetchFundSeriesForFiling(filingID uint64) ([]string, error) {
	var rows []string
	err := GetDB().
		Table("form_data_legacy").
		Select("DISTINCT fundSeriesid").
		Where("filingid = ?", filingID).
		Scan(&rows).Error
	return rows, err
}
