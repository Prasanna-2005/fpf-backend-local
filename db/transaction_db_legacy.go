package db

import (
	"fmt"
	"form_pf_backend/models"
	"strings"
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
			out[r.FundID][r.FormLineNo] = [2]string{"formula", r.InfixExpression}
		case "filters":
			out[r.FundID][r.FormLineNo] = [2]string{*r.ComputationType, grpFormat(r.GroupOperation)}
		default:
			out[r.FundID][r.FormLineNo] = [2]string{"null", "null"}
			continue
		}

	}

	return out, nil
}

func grpFormat(expression string) string {
	var statement string
	firstOpen := strings.Index(expression, "(")
	firstClose := strings.Index(expression[firstOpen:], ")") + firstOpen
	remaining := expression[firstClose+1:]
	filterOperation := strings.ToLower(expression[:firstOpen])
	filterCondition := expression[firstOpen+1 : firstClose]
	target := remaining

	if filterOperation == "desc" {
		statement = fmt.Sprintf("Top %v percent of column %v ", filterCondition, target)
	} else {
		filterConditionSplit := strings.Split(filterCondition, ":")
		statement = fmt.Sprintf("Filter column %v based on %v column with values %v", target, filterConditionSplit[0], filterConditionSplit[1])
	}
	return statement
}

func GetFieldHistoryDB(filingID, fundSeriesID, fieldNo string) ([]models.FormDataLegacy, error) {
	var rows []models.FormDataLegacy

	err := GetDB().Raw(`
		SELECT 
			f.fundSeriesId, f.fieldNo, f.formLineNo,
			ffml.lineText,
			f.value, f.isCurrentValue,
			f.createdAt, u.userName as createdBy, 
			f.updatedAt, u2.userName as updatedBy
		FROM form_data_legacy f
		left join users u on u.id = f.createdBy 
		left join users u2 on u2.id = f.updatedBy
		left join form_fields_ms_legacy ffml  on f.formLineNo = ffml.formLineNo 
		WHERE f.filingid = ? 
		  AND f.fundSeriesId = ? 
		  AND f.fieldNo = ? 
		ORDER BY createdAt DESC`,
		filingID, fundSeriesID, fieldNo).
		Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	return rows, nil
}
