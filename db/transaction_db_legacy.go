package db

import (
	"fmt"
	"form_pf_backend/models"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var AUM = []string{
	"8", "9",
}

/* ---------- meta ---------- */
func FetchFormFieldMSLegacy() ([]models.FormFieldMSLegacy, error) {
	var rows []models.FormFieldMSLegacy
	err := GetDB().Order("formLineNo ASC").Find(&rows).Error
	return rows, err
}

func FetchCurrentValuesForFiling(filingID string, version int) (map[string]map[string]string, error) {
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
		     WHERE filingid = ? AND version = ? AND isCurrentValue = ?`, filingID, version, true).
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

// HeadVersion is the committed filing version. Zero means that no extraction
// has completed; uncommitted form_data_legacy rows must not become the head.
func HeadVersion(filingID string) (int, error) {
	var head int
	if err := GetDB().Raw(`SELECT currentVersion FROM filing_tracker WHERE id = ?`, filingID).Scan(&head).Error; err != nil {
		return 0, err
	}
	if head > 0 {
		return head, nil
	}
	return 0, nil
}

func AvailableVersions(filingID string) ([]int, error) {
	head, err := HeadVersion(filingID)
	if err != nil {
		return nil, err
	}
	if head == 0 {
		return nil, ErrExtractionNotCompleted
	}
	var versions []int
	err = GetDB().Raw(`SELECT DISTINCT version FROM form_data_legacy WHERE filingId = ? AND version <= ? ORDER BY version ASC`, filingID, head).Scan(&versions).Error
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return []int{1}, nil
	}
	return versions, nil
}

// ResolveWriteVersion accepts an omitted version as the head and rejects any other version.
func ResolveWriteVersion(filingID string, requested int) (int, error) {
	head, err := HeadVersion(filingID)
	if err != nil {
		return 0, err
	}
	if head == 0 {
		return 0, ErrExtractionNotCompleted
	}
	if requested == 0 || requested == head {
		return head, nil
	}
	return 0, ErrVersionNotHead
}

func GetFieldHistoryDB(filingID, fundSeriesID, fieldNo string) ([]models.FieldHistoryRow, error) {
	var rows []models.FieldHistoryRow

	err := GetDB().Raw(`
		SELECT 
			f.fundSeriesId, f.fieldNo, f.formLineNo,
			ffml.lineText,
			f.value, f.sourceFiles, f.isCurrentValue,
			f.version, f.isEdit,
			f.createdAt, u.userName as createdBy, 
			f.updatedAt, f.updatedBy as updatedById, u2.userName as updatedBy
		FROM form_data_legacy f
		left join users u on u.id = f.createdBy 
		left join users u2 on u2.id = f.updatedBy
		left join form_fields_ms_legacy ffml  on f.formLineNo = ffml.formLineNo 
		WHERE f.filingid = ? 
		  AND f.fundSeriesId = ? 
		  AND f.fieldNo = ? 
		ORDER BY f.version DESC, f.createdAt DESC`,
		filingID, fundSeriesID, fieldNo).
		Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	return rows, nil
}

func FetchSection1a(filingId string) ([]models.FormDatum, error) {
	head, err := HeadVersion(filingId)
	if err != nil {
		return nil, err
	}
	var rows []models.FormDatum
	err = GetDB().
		Table("form_data_legacy").
		Select("*").
		Where("filingId = ? AND fundSeriesId = ? AND isCurrentValue = ? AND version = ?", filingId, "SECTION-1A", "1", head).
		Find(&rows).Error

	if err != nil {
		return nil, err
	}
	return rows, nil
}

func GetAUM(filingid string) ([]models.FormDataLegacy, error) {
	var rows []models.FormDataLegacy
	head, err := HeadVersion(filingid)
	if err != nil {
		return rows, err
	}
	if err := sqlDb.Where("filingid = ? AND fieldNo= ? AND isCurrentValue= ? AND version = ?", filingid, "3", 1, head).Find(&rows).Error; err != nil {
		return rows, err
	}
	return rows, nil
}

func getAUMType(tx *gorm.DB, filingID, fundID, formLineNo string, version int) (string, string, error) {
	onceFundCache.Do(loadFundCache)
	b, ok := fundCache[fundID]
	if !ok {
		return "", "", gorm.ErrRecordNotFound
	}

	// Determine fund type letter based on Fundtype
	var fundTypeLetter string
	switch strings.ToLower(b.Fundtype) {
	case "hedge":
		fundTypeLetter = "a"
	case "liquidity":
		fundTypeLetter = "b"
	case "private":
		fundTypeLetter = "c"
	case "real estate":
		fundTypeLetter = "d"
	case "securitized":
		fundTypeLetter = "e"
	case "vc":
		fundTypeLetter = "f"
	case "other":
		fundTypeLetter = "g"
	case "nota":
		fundTypeLetter = "h"
	default:
		return "", "", fmt.Errorf("unknown fund type: %s", b.Fundtype)
	}

	// Map formLineNo -> AUM suffix
	formLineToSuffix := map[string]string{
		"8": "Regulatory_assets_under_management",
		"9": "Net_assets_under_management",
	}

	suffix, ok := formLineToSuffix[formLineNo]
	if !ok {
		return "", "", fmt.Errorf("unsupported AUM line %s", formLineNo)
	}
	line := fmt.Sprintf("3_(%s)_%s", fundTypeLetter, suffix)
	var row models.FormDataLegacy
	if err := tx.Where("filingid = ? AND fundSeriesId = ? AND fieldNo = ? AND formLineNo = ? AND version = ? AND isCurrentValue = ?",
		filingID, "SECTION-1A", "3", line, version, true).First(&row).Error; err != nil {
		return "", "", fmt.Errorf("fetching current Section 1A AUM: %w", err)
	}
	return row.Value, line, nil
}

// updateAUM updates the Section 1A Question 3 aggregate in the caller's
// transaction. It must be used with the associated Question 8/9 write.
func updateAUM(tx *gorm.DB, filingID, fundID, userID string, version int, formLineNo, newVal string, accept bool) error {
	var row models.FormDataLegacy
	aumVal, aumLineNo, err := getAUMType(tx, filingID, fundID, formLineNo, version)
	if err != nil {
		return err
	}

	if err := tx.Model(&models.FormDataLegacy{}).
		Where("filingid = ? AND fundSeriesId = ? AND fieldNo = ? AND isCurrentValue = ? AND version = ? AND formLineNo = ?",
			filingID, fundID, formLineNo, true, version, formLineNo).
		First(&row).Error; err != nil {
		return err
	}

	if err := tx.Model(&models.FormDataLegacy{}).
		Where("filingid = ?  AND fieldNo = ? AND isCurrentValue = ? AND version = ? AND formLineNo = ?",
			filingID, "3", true, version, aumLineNo).
		Update("isCurrentValue", false).Error; err != nil {
		return err
	}

	if accept {
		row.Value = "-" + row.Value
	} else {
		newVal = "-" + newVal
	}

	val, err := addStrings(newVal, row.Value, aumVal)
	if err != nil {
		return err
	}

	rowsToInsert := []models.FormDataLegacy{
		{
			ID:             uuid.New().String(),
			FilingID:       filingID,
			FundSeriesid:   "SECTION-1A",
			FieldNo:        "3",
			FormLineNo:     aumLineNo,
			Value:          strconv.FormatFloat(val, 'f', -1, 64),
			IsCurrentValue: true,
			Version:        version,
			IsEdit:         true,
			CreatedBy:      userID,
			CreatedAt:      time.Now(),
			UpdatedBy:      userID,
		},
	}

	// CreateInBatches sends one INSERT with multiple values
	return tx.CreateInBatches(&rowsToInsert, 200).Error
}

func addStrings(vals ...string) (float64, error) {
	var sum float64
	for _, v := range vals {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, err
		}
		sum += f
	}
	return sum, nil
}
