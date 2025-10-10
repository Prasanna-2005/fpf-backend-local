package db

import (
	"fmt"
	"form_pf_backend/models"
)

/* ---------- meta ---------- */
func FetchFormFieldMSLegacy() ([]models.FormFieldMSLegacy, error) {
	var rows []models.FormFieldMSLegacy
	err := GetDB().Order("formLineNo ASC").Find(&rows).Error
	return rows, err
}

/* ---------- value store ---------- */
// FetchCurrentValuesForFiling returns map[formLineNo] → value
// for every row flagged isCurrentValue = 1 (or latest if none flagged).
func FetchCurrentValuesForFiling(filingID uint64) (map[string]string, error) {
	type pair struct {
		FormLineNo string
		Value      string
	}
	var rows []pair

	// preferred rows first
	if err := GetDB().
		Model(&models.FormDataLegacy{}).
		Select("formLineNo", "value").
		Where("filingid = ? AND isCurrentValue = ?", filingID, true).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	// fallback: newest row per line item for anything missing
	missing := make([]string, 0, 4) // cheap buffer
	out := make(map[string]string, len(rows))
	for _, p := range rows {
		out[p.FormLineNo] = p.Value
	}

	// discover missing keys
	if err := GetDB().
		Model(&models.FormDataLegacy{}).
		Select("DISTINCT formLineNo").
		Where("filingid = ?", filingID).
		Scan(&missing).Error; err != nil {
		return nil, err
	}

	for _, line := range missing {
		if _, ok := out[line]; ok {
			continue // already have the flagged row
		}
		var fallback pair
		_ = GetDB().
			Model(&models.FormDataLegacy{}).
			Select("formLineNo", "value").
			Where("filingid = ? AND formLineNo = ?", filingID, line).
			Order("updatedAt DESC").
			Limit(1).
			Scan(&fallback).Error
		out[line] = fallback.Value
	}
	fmt.Println(out)
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
	fmt.Println(rows)
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r
	}
	fmt.Println(out)
	return out, err
}
