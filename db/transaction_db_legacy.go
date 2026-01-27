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

func FetchCurrentValuesForFiling(filingID string) (map[string]map[string]string, error) {
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
func FetchFundSeriesForFiling(filingID string) ([]string, error) {
	var rows []string
	err := GetDB().
		Table("form_data_legacy").
		Select("DISTINCT fundSeriesid").
		Where("filingid = ?", filingID).
		Scan(&rows).Error
	return rows, err
}

func GetComputationlogs(series, clientCode string) (map[string]map[string][2]string, error) {

	var rows []models.CompLogs

	if err := sqlDb.Where("series = ? AND clientCode = ?", series, clientCode).Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[string]map[string][2]string)
	for _, r := range rows {
		if out[r.FundID] == nil {
			out[r.FundID] = make(map[string][2]string)
		}
		if r.ComputationType == nil {
			out[r.FundID][r.FormLineNo] = [2]string{"null", "null"}
			continue
		}

		switch *r.ComputationType {
		case "staticData":
			out[r.FundID][r.FormLineNo] = [2]string{*r.ComputationType, r.StaticData}
		case "InfixExpression":
			out[r.FundID][r.FormLineNo] = [2]string{*r.ComputationType, r.InfixExpression}
		case "filters":
			out[r.FundID][r.FormLineNo] = [2]string{*r.ComputationType, grpFormat(r.GroupOperation)}
		default:
			out[r.FundID][r.FormLineNo] = [2]string{"null", "null"}
			continue
		}

	}

	return out, nil
}

func grpFormat(grpOp string) string {
	return grpOp
}
