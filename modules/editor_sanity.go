package modules

import (
	"encoding/json"
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"strings"
)

var mandateField = []string{
	"0", "1", "2", "3", "4",
}

func MandateFieldCheck(filingId string) ([]string, bool) {
	var missingFields []string
	rows, err := db.FetchSection1a(filingId)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Fetch section 1a err: %v", err.Error()))
		return nil, false
	}

	groupedRows := groupByFieldNo(rows)

	for _, fieldNo := range mandateField {
		fieldRows, ok := groupedRows[fieldNo]
		if !ok && fieldNo != "4" {
			// entire fieldNo block missing from DB
			missingFields = append(missingFields, fmt.Sprintf("fieldNo %s is missing from filing", fieldNo))
			continue
		}
		missing := validateField(fieldNo, fieldRows)
		missingFields = append(missingFields, missing...)

	}
	// fmt.Println(missingFields)
	if len(missingFields) == 0 {
		return missingFields, true
	}
	return missingFields, false
}

func groupByFieldNo(rows []models.FormDatum) (res map[string][]models.FormDatum) {
	res = make(map[string][]models.FormDatum)
	for _, row := range rows {
		if row.FieldNo == "" {
			logger.Log("WARNING", "Skipping row as field number not present.")
			continue
		}
		fieldNo := strings.ToLower(row.FieldNo)
		res[fieldNo] = append(res[fieldNo], row)
	}
	return res
}

// validateField dispatches to the correct section check based on fieldNo.
// Returns a list of missing/invalid field messages.
func validateField(fieldNo string, fieldRows []models.FormDatum) []string {
	// Build a lineNo → FormDatum map for easy sub-field lookup within this fieldNo group

	lineMap := make(map[string]models.FormDatum)
	for _, row := range fieldRows {
		if strings.EqualFold(row.FieldNo, fieldNo) {
			formlineno := strings.ToLower(row.FormLineNo)
			lineMap[formlineno] = row
		}
	}
	switch fieldNo {
	case "0":
		return validateSection0(lineMap)
	case "1":
		return validateSection1(lineMap)
	case "2":
		return validateSection2(lineMap)
	case "3":
		return validateSection3(lineMap)
	case "4":
		return validateSection4(lineMap)
	}
	return nil
}

// --- helpers ---

func isValueFilled(row models.FormDatum) bool {
	return row.Value != nil && strings.TrimSpace(*row.Value) != ""
}

func isFilled(lineMap map[string]models.FormDatum, key string) bool {
	row, ok := lineMap[strings.ToLower(key)]
	return ok && isValueFilled(row)
}

// --- FieldNo wise validators ---

// FiledNo 0: either 0_A or 0_B must be filled; 0_C is compulsory
func validateSection0(lineMap map[string]models.FormDatum) []string {
	var missing []string
	if !isFilled(lineMap, "0_a") && !isFilled(lineMap, "0_b") {
		missing = append(missing, "fieldno 0: either 0_A or 0_B must be filled")
	}
	if !isFilled(lineMap, "0_c") {
		missing = append(missing, "fieldno 0: 0_C (reporting period end date) is compulsory")
	}
	// fmt.Println(lineMap)
	return missing
}

// fieldNo 1: either 1_a path or 1_b path must be filled
//
//	1_a path → 1_a_i (Legal name) and 1_a_ii (CRD Number) are mandatory
//	1_b path → exp_table []string, index 0 and 1 must be non-empty
func validateSection1(lineMap map[string]models.FormDatum) []string {
	var missing []string

	aiFilled := isFilled(lineMap, "1_a_i")
	aiiFilled := isFilled(lineMap, "1_a_ii")
	aPathActive := aiFilled || aiiFilled

	bRow, bExists := lineMap["1_b"]
	bPathActive := bExists && isValueFilled(bRow)

	if !aPathActive && !bPathActive {
		missing = append(missing, "fieldno 1: either 1_a (with 1_a_i & 1_a_ii) or 1_b must be filled")
		return missing
	}

	if aPathActive {
		if !aiFilled {
			missing = append(missing, "fieldno 1: 1_a_i (Legal name) must be filled")
		}
		if !aiiFilled {
			missing = append(missing, "fieldno 1: 1_a_ii (CRD Number) must be filled")
		}
	}

	if bPathActive {
		var bValues []string
		if err := json.Unmarshal([]byte(*bRow.Value), &bValues); err != nil {
			missing = append(missing, "fieldno 1: 1_b has invalid format, expected []string")
		} else if len(bValues) < 2 {
			missing = append(missing, "fieldno 1: 1_b must have at least 2 entries")
		} else {
			if strings.TrimSpace(bValues[0]) == "" {
				missing = append(missing, "fieldno 1: 1_b index 0 must be non-empty")
			}
			if strings.TrimSpace(bValues[1]) == "" {
				missing = append(missing, "fieldno 1: 1_b index 1 must be non-empty")
			}
		}
	}

	return missing
}

// fieldNo 2: either 2_a or 2_b path must be filled
//
//	2_a path → 2_a_i through 2_a_vi all mandatory
//	2_b path → 2_b_vii (date) mandatory
func validateSection2(lineMap map[string]models.FormDatum) []string {
	var missing, aSubFields, bSubFields []string
	aPath := false
	bPath := false

	for key, _ := range lineMap {
		if strings.HasPrefix(key, "2_a_") {
			aSubFields = append(aSubFields, key)
			if isFilled(lineMap, key) {
				aPath = true
			}
		} else if strings.HasPrefix(key, "2_b_") {
			bSubFields = append(bSubFields, key)
			if isFilled(lineMap, key) {
				bPath = true
			}
		}
	}
	if aPath == bPath { // since there is no boolean XOR(golang) != is used
		missing = append(missing, "fieldno 2: either 2_a or 2_b signature block must be filled")
		return missing
	}

	// based on path activity
	if aPath {
		for _, sf := range aSubFields {
			if !isFilled(lineMap, sf) {
				missing = append(missing, fmt.Sprintf("fieldno 2: %s must be filled", sf))
			}
		}
	}

	if bPath {
		for _, sf := range aSubFields {
			if !isFilled(lineMap, sf) {
				missing = append(missing, fmt.Sprintf("fieldno 2: %s must be filled", sf))
			}
		}
	}
	return missing
}

// fieldNo 3: at least one of 3_(a) through 3_(h) sub-rows must be filled
func validateSection3(lineMap map[string]models.FormDatum) []string {
	hasSubFields := false

	for key, row := range lineMap {
		if strings.HasPrefix(key, "3_") && isFilled(lineMap, key) && *row.Value != "0" {
			hasSubFields = true
			return nil
		}
	}

	if !hasSubFields {
		logger.Log("WARNING", "No subfields present for fieldNo 3")
		return nil // section not applicable
	}

	return []string{"fieldno 3: at least one of 3_(a) through 3_(h) assets must be filled"}
}

// fieldNo 4: exp_table stored as [][]string
// Each row's index 1 (description) must be non-empty
func validateSection4(lineMap map[string]models.FormDatum) []string {
	fmt.Println("printing line 4")
	fmt.Println(lineMap)
	row, exists := lineMap["4"]
	if !exists || row.Value == nil || strings.TrimSpace(*row.Value) == "" {
		return nil // section 4 is optional
	}

	var matrix [][]string
	if err := json.Unmarshal([]byte(*row.Value), &matrix); err != nil {
		return []string{"fieldno 4: invalid format, expected [][]string"}
	}

	var missing []string
	for i, inner := range matrix {
		if len(inner) < 2 {
			missing = append(missing, fmt.Sprintf("fieldno 4: row %d must have at least 2 elements", i))
		} else if strings.TrimSpace(inner[1]) == "" {
			missing = append(missing, fmt.Sprintf("fieldno 4: row %d description (index 1) must be non-empty", i))
		}
	}
	return missing
}
