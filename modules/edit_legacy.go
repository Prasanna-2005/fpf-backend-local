package modules

import (
	"encoding/json"
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

type group struct {
	parent   models.FormFieldMSLegacy
	children []models.FormFieldMSLegacy
}

// OrderedMap is a simple ordered map implementation for JSON marshaling
type OrderedMap struct {
	keys   []string
	values map[string]interface{}
}

var expTableColumnHeaders = map[string][]string{
	"1_b": {
		"Legal name",
		"SEC 801-Number",
		"NFA ID Number, if any",
		"Large trader ID, if any",
		"Large trader ID suffix, if any",
	},
	"4": {
		"Question number",
		"Description",
	},
}

var xmlTagAUM = map[string]string{
	"3_(a)": "HEDGE",
	"3_(b)": "LIQUIDITY",
	"3_(c)": "PRIVATE",
	"3_(d)": "REALESTATE",
	"3_(e)": "SECUREASSET",
	"3_(f)": "VC",
	"3_(g)": "OTHER",
	"3_(h)": "NONPRIVATE",
}

var expTableMetaData = map[string]map[string]string{
	"1_b": {
		"Legal name":                     "text",
		"SEC 801-Number":                 "text",
		"NFA ID Number, if any":          "text",
		"Large trader ID, if any":        "text",
		"Large trader ID suffix, if any": "text",
	},
	"4": {
		"Question number": "text",
		"Description":     "text",
	},
}

func NewOrderedMap() *OrderedMap {
	return &OrderedMap{
		keys:   make([]string, 0),
		values: make(map[string]interface{}),
	}
}

func (om *OrderedMap) Set(key string, value interface{}) {
	if _, exists := om.values[key]; !exists {
		om.keys = append(om.keys, key)
	}
	om.values[key] = value
}

func (om *OrderedMap) MarshalJSON() ([]byte, error) {
	result := "{"
	for i, k := range om.keys {
		if i > 0 {
			result += ","
		}
		keyJSON, _ := json.Marshal(k)
		valJSON, _ := json.Marshal(om.values[k])
		result += string(keyJSON) + ":" + string(valJSON)
	}
	result += "}"
	return []byte(result), nil
}

func optionSlice(dataType, formLineNo string) []string {
	if dataType == "bool" {
		return []string{"YES", "NO"}
	}
	if dataType == "mcq" {
		var opt []string
		for options, _ := range mcqlist[formLineNo] {
			opt = append(opt, options)
		}
		return opt
	}
	return nil
}

var mcqlist = map[string]map[string]string{
	"0_A": {"Submit your first filing on Form PF": "PF-INIT", "Submit an annual update": "PF-UPDATING", "Amend a previously submitted filing": "PF-AMEND", "Submit a final filing": "PF-FINAL", "Request a temporary hardship exemption": "EXEMPTION"},
	"0_B": {"Submit your first filing on Form PF for the [1st, 2nd, 3rd, 4th] quarter": "PF-INIT", "Submit a quarterly update (including fourth quarter updates)for the [1st, 2nd, 3rd, 4th] quarter, ": "PF-UPDATING", "Amend a previously submitted filing for the [1st, 2nd, 3rd, 4th] quarter": "PF-AMEND", "Transition to annual reporting": "PF-TRANS", "Submit a final filing": "PF-FINAL", "Request a temporary hardship exemption": "EXEMPTION"},
}

func headingFor(fieldNo string) string {
	switch fieldNo {
	case "1", "2":
		return "sec1a:Item A. Information about you"
	case "3":
		return "sec1a:Item B. Information about assets of private funds that you advise"
	case "4":
		return "sec1a:Item C. Miscellaneous"
	case "5", "6", "7":
		return "sec1b:Item A. Reporting fund identifying information"
	case "8", "9", "10", "11", "12", "13", "14", "15", "16":
		return "sec1b:Item B. Assets, financing and investor concentration"
	case "17":
		return "sec1b:Item C. Reporting fund performance"
	case "18":
		return "sec1c:Item A. Reporting fund identifying information"
	case "19", "20", "21", "22", "23", "24", "25":
		return "sec1c:Item B. Certain information regarding the reporting fund"

	default:
		return ""
	}
}

// BuildResponselegacy loads one filing version. tracker is the row already
// read by RequireFilingAccess; a nil tracker falls back to HeadVersion.
func BuildResponselegacy(filingID string, requested int, tracker *models.FilingTracker) (*models.FilingResponselegacy, error) {
	meta, err := db.FetchFormFieldMSLegacy()
	if err != nil {
		return nil, err
	}

	var head int
	if tracker != nil && tracker.CurrentVersion > 0 {
		head = tracker.CurrentVersion
	} else {
		head, err = db.HeadVersion(filingID)
		if err != nil {
			return nil, err
		}
	}
	if head == 0 {
		return nil, db.ErrExtractionNotCompleted
	}

	versions := db.VersionList(head)
	loadVersion := head
	if requested > 0 {
		if requested > head {
			return nil, db.ErrVersionNotFound
		}
		loadVersion = requested
	}

	valueMap, err := db.FetchCurrentValuesForFiling(filingID, loadVersion)
	if err != nil {
		return nil, err
	}
	fsIDs := make([]string, 0, len(valueMap))
	for fundID := range valueMap {
		fsIDs = append(fsIDs, fundID)
	}
	if len(fsIDs) == 0 {
		return &models.FilingResponselegacy{Funds: []models.Fundlegacy{}},
			fmt.Errorf("extraction is still in process")
	}
	fundMap, err := db.GetFundMap(fsIDs)
	if err != nil {
		return nil, err
	}

	// 1. group meta rows by fieldNo
	fieldMap := make(map[string]*group)
	for _, m := range meta {
		if m.FormLineItemNo == m.FieldNo {
			if g := fieldMap[m.FieldNo]; g == nil {
				fieldMap[m.FieldNo] = &group{parent: m}
			} else {
				g.parent = m
			}
			continue
		}
		if fieldMap[m.FieldNo] == nil {
			fieldMap[m.FieldNo] = &group{}
		}
		fieldMap[m.FieldNo].children = append(fieldMap[m.FieldNo].children, m)
	}

	// 2. build response
	resp := &models.FilingResponselegacy{
		Funds:             make([]models.Fundlegacy, 0, len(fsIDs)),
		CurrentVersion:    head,
		Version:           loadVersion,
		AvailableVersions: versions,
	}

	for _, fsID := range fsIDs {
		fundRec := fundMap[fsID]
		fund := models.Fundlegacy{
			FundID:   fundRec.FundID,
			FundName: fundRec.FundName,
		}

		// pick the value sub-map for this fund-series
		vm := valueMap[fsID]
		if vm == nil { // no data yet
			vm = make(map[string]string)
		}
		seen := make(map[string]bool)
		for _, m := range meta {
			// skip if we already emitted this fieldNo
			fieldNoInt, _ := strconv.Atoi(m.FieldNo)
			if (fieldNoInt > 17 && fundRec.FundType != "HEDGE") || (fieldNoInt < 5 && fundRec.FundType != "ALL") {
				continue
			}
			if fieldNoInt > 4 && fundRec.FundType == "ALL" {
				continue
			}

			if seen[m.FieldNo] {
				continue
			}
			seen[m.FieldNo] = true

			g := fieldMap[m.FieldNo]

			if g.parent.IsTableData {
				fund.FundData = append(fund.FundData,
					buildTableFieldItemGroup(m.FieldNo, g, vm))
				continue
			}
			if g.parent.IsMCQ {
				fund.FundData = append(fund.FundData, buildMCQ(m.FieldNo, g, vm))
				continue
			}
			if g.parent.DataType == "exp_table" {
				fund.FundData = append(fund.FundData, buildExpandableTable(m.FieldNo, g, vm))
				continue
			}

			// 2. normal text / bool (options) block
			fund.FundData = append(fund.FundData,
				buildFieldItemGroup(g, vm))
		}
		resp.Funds = append(resp.Funds, fund)
	}

	// 3. sort funds by name
	sort.Sort(byFundName(resp.Funds))

	// 4. sort field_data inside every fund by field_no
	for i := range resp.Funds {
		sort.Sort(byFieldNo(resp.Funds[i].FundData))
	}

	return resp, nil
}

func buildExpandableTableLine(l models.FormFieldMSLegacy, vm map[string]string) models.LineDetail {
	return models.LineDetail{
		LineItemNo:      l.FormLineItemNo,
		HasLineItemText: l.LineText != "",
		LineItemType:    l.DataType,
		LineText:        l.LineText,
		TableMetadata:   tableMetaDataFromStringMap(expTableMetaData[l.FormLineItemNo]),
		TableHeaders:    expTableColumnHeaders[l.FormLineItemNo],
		Value:           vm[l.FormLineItemNo],
		TableData:       nil,
		RequireDate:     l.RequiresDate,
		DateValue:       "",
		DisclaimerText:  l.DisclaimerText,
	}
}

func tableMetaDataFromStringMap(m map[string]string) map[string]models.TableColumnMetadata {
	if m == nil {
		return nil
	}
	out := make(map[string]models.TableColumnMetadata, len(m))
	for k, v := range m {
		out[k] = models.TableColumnMetadata{DataType: v, DisplayAffix: ""}
	}
	return out
}

func buildExpandableTable(fieldNo string, g *group, vm map[string]string) models.FieldItem {
	par := g.parent
	return models.FieldItem{
		FieldNo:          fieldNo,
		HasFieldItemText: par.LineText != "",
		FieldItemType:    par.DataType,
		HasTableData:     false,
		TableMetadata:    tableMetaDataFromStringMap(expTableMetaData[fieldNo]),
		FieldText:        par.LineText,
		FieldHeading:     headingFor(fieldNo),
		Value:            vm[fieldNo],
		TableHeaders:     expTableColumnHeaders[fieldNo],
		FieldData:        []models.LineDetail{},
		Confirmed:        false,
		Editing:          true,
		IsMultipleAdd:    par.IsMultipleAdd,
		DisclaimerText:   par.DisclaimerText,
	}
}

// table builder
func buildTableFieldItemGroup(fieldNo string, g *group, vm map[string]string) models.FieldItem {
	par := g.parent
	if par.FormLineItemNo == "" {
		for _, v := range g.children {
			if v.FormLineItemNo == fieldNo {
				par = v
				break
			}
		}
	}

	// Group children by row-key
	rowMap := make(map[string]map[string]string)
	tableMetadata := make(map[string]models.TableColumnMetadata)
	for _, v := range g.children {

		if v.FormLineItemNo == fieldNo {
			continue
		}
		tokens := strings.SplitN(v.FormLineItemNo, "_", 3)
		if len(tokens) <= 2 {
			continue
		}
		rowKey, colKey := tokens[1], tokens[2]
		if rowMap[rowKey] == nil {
			rowMap[rowKey] = make(map[string]string)
		}
		rowMap[rowKey][colKey] = vm[v.FormLineItemNo]
		tableMetadata[colKey] = models.TableColumnMetadata{DataType: v.DataType, DisplayAffix: v.DisplayAffix}
	}

	// Stable row order
	rowKeys := make([]string, 0, len(rowMap))
	for k := range rowMap {
		rowKeys = append(rowKeys, k)
	}
	sort.Strings(rowKeys)

	// Convert to ordered maps
	tableData := make([]*OrderedMap, 0, len(rowMap))
	for _, rk := range rowKeys {
		var label string
		for _, v := range g.children {
			if strings.HasPrefix(v.FormLineItemNo, fieldNo+"_"+rk+"_") {
				label = v.LineText
				break
			}
		}

		// Create ordered map with s.name and s.text first
		obj := NewOrderedMap()
		obj.Set("s.name", rk)
		obj.Set("s.text", rk+" "+label)

		// Add remaining columns in sorted order
		colKeys := make([]string, 0, len(rowMap[rk]))
		for ck := range rowMap[rk] {
			colKeys = append(colKeys, ck)
		}
		sort.Strings(colKeys)

		for _, ck := range colKeys {
			obj.Set(ck, rowMap[rk][ck])
		}

		tableData = append(tableData, obj)
	}
	return models.FieldItem{
		FieldNo:          fieldNo,
		HasFieldItemText: par.LineText != "",
		FieldItemType:    par.DataType,
		HasTableData:     true,
		TableMetadata:    tableMetadata,
		FieldText:        par.LineText,
		FieldHeading:     headingFor(fieldNo),
		Value:            vm[fieldNo],
		FieldData:        []models.LineDetail{},
		TableData:        tableData,
		Confirmed:        false,
		Editing:          true,
		IsMultipleAdd:    par.IsMultipleAdd,
		DisclaimerText:   par.DisclaimerText,
	}
}

// builder for text / bool
func buildFieldItemGroup(g *group, vm map[string]string) models.FieldItem {
	par := g.parent

	fItem := models.FieldItem{
		FieldNo:          par.FieldNo,
		HasFieldItemText: par.LineText != "",
		FieldItemType:    par.DataType,
		HasTableData:     par.IsTableData,
		FieldText:        par.LineText,
		FieldHeading:     headingFor(par.FieldNo),
		Value:            vm[par.FormLineItemNo],
		Confirmed:        false,
		Editing:          true,
		IsMultipleAdd:    par.IsMultipleAdd,
		DisclaimerText:   par.DisclaimerText,
		DisplayAffix:     par.DisplayAffix,
		Options:          optionSlice(par.DataType, par.FieldNo),
	}

	fItem.FieldData = make([]models.LineDetail, 0, len(g.children))
	for _, l := range g.children {
		switch l.DataType {
		case "exp_table":
			fItem.FieldData = append(fItem.FieldData, buildExpandableTableLine(l, vm))

		default:
			fItem.FieldData = append(fItem.FieldData, models.LineDetail{
				LineItemNo:      l.FormLineItemNo,
				HasLineItemText: l.LineText != "",
				LineItemType:    l.DataType,
				LineText:        l.LineText,
				Value:           vm[l.FormLineItemNo],
				Options:         optionSlice(l.DataType, l.FormLineItemNo),
				TableData:       nil,
				RequireDate:     l.RequiresDate,
				DateValue:       "",
				DisclaimerText:  l.DisclaimerText,
				DisplayAffix:    l.DisplayAffix,
			})

		}

	}

	return fItem
}

func buildMCQ(fieldNo string, g *group, vm map[string]string) models.FieldItem {
	par := g.parent

	return models.FieldItem{
		FieldNo:          fieldNo,
		HasFieldItemText: par.LineText != "",
		FieldItemType:    par.DataType,
		HasTableData:     false,
		FieldText:        par.LineText,
		FieldHeading:     headingFor(fieldNo),
		Value:            vm[fieldNo],         // currently ticked choice
		Options:          envChoices(fieldNo), // all choices in env order
		Confirmed:        false,
		Editing:          true,
		IsMultipleAdd:    par.IsMultipleAdd,
		DisclaimerText:   par.DisclaimerText,
	}
}

func envChoices(fieldNo string) []string {
	raw := os.Getenv(fieldNo + "_MCQ_OPTIONS")

	if raw == "" {
		return nil
	}
	var choices []string
	err := json.Unmarshal([]byte(raw), &choices)
	if err != nil {
		logger.Log("ERROR", err.Error())
	}
	return choices
}

type FundInfo struct {
	FundID   string `json:"fund_id"`
	FundName string `json:"fund_name"`
	FundType string `json:"fund_type"`
}

type TagInfo struct {
	FundID        string      `json:"fund_id"`
	XMLTags       string      `json:"xmltags"`
	Value         interface{} `json:"value"`
	QuestionNo    string      `json:"qno,omitempty"`
	QuestionSubNo string      `json:"qSubno,omitempty"`
}

type SimpleResponse struct {
	FundInfo      []FundInfo  `json:"fund_info"`
	TagInfoSec1a  [][]TagInfo `json:"tags_info_sec1a"`
	TagsInfoSec1b [][]TagInfo `json:"tags_info_sec1b"`
	TagInfoSec1c  [][]TagInfo `json:"tags_info_sec1c"`
}

var reportingPeriod = map[string]string{
	"A": "A",
	"B": "Q",
}

func ValueFor(value, qtype, qno, qsubno, xmlTag string) string {
	if xmlTag == "StrategyType_StrategyChecked" {
		vals := qsubno[strings.Index(qsubno, "(")+1] - 'a' + 1
		val := int(vals)
		return strconv.Itoa(val)
	}
	xmlOpt := map[string]string{
		"Single primary strategy": "S",
		"Multi-strategy":          "M",
		"0%":                      "0",
		"less than 10%":           "less than 10",
		"10-25%":                  "10-25",
		"26-50%":                  "26-50",
		"51-75%":                  "51-75",
		"76-99%":                  "76-99",
		"100% or more":            "100 or more",
	}
	if qtype == "mcq" {
		xmlVal, ok := xmlOpt[value]
		if !ok {
			logger.Log("WARNING", fmt.Sprintf("Invalid options present for %v option:%v", qno, value))
			xmlVal = "NULL"
		}
		return xmlVal
	}
	return value
}

func TransformToSimpleResponse(filingID string) (*SimpleResponse, error) {

	rows, err := db.FetchFormDataByFiling(filingID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch form data: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no data found for filingID %s", filingID)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].FundName != rows[j].FundName {
			return rows[i].FundName < rows[j].FundName
		}
		if rows[i].FieldNo != rows[j].FieldNo {
			return rows[i].FieldNo < rows[j].FieldNo
		}
		return rows[i].FormLineNo < rows[j].FormLineNo
	})

	fundMap := make(map[string]FundInfo)
	tagMap1b := make(map[string][]TagInfo)
	tagMap1c := make(map[string][]TagInfo)
	tagMap1a := make(map[string][]TagInfo)

	for _, r := range rows {
		qno, _ := strconv.Atoi(r.FieldNo)
		if r.Value == nil || *r.Value == "" {
			continue
		}
		if _, ok := fundMap[r.FundID]; !ok {
			fundMap[r.FundID] = FundInfo{
				FundID:   r.FundID,
				FundName: r.FundName,
				FundType: r.FundType,
			}
		}
		if qno > 17 {
			if r.FormLineNo == "18_a" {
				continue
			}
			if qno == 20 && *r.Value == "true" {
				tagMap1c[r.FundID] = append(tagMap1c[r.FundID], TagInfo{
					FundID:  r.FundID,
					XMLTags: "StrategyType",
					Value:   ValueFor(*r.Value, r.LineItemType, r.FieldNo, r.FormLineNo, r.XmlTags),
				}, TagInfo{
					FundID:  r.FundID,
					XMLTags: "StrategyChecked",
					Value:   "true",
				})
			} else if qno == 22 || qno == 23 {
				tagMap1c[r.FundID] = append(tagMap1c[r.FundID], TagInfo{
					FundID:     r.FundID,
					XMLTags:    string(r.XmlTags),
					Value:      ValueFor(*r.Value, r.LineItemType, r.FieldNo, r.FormLineNo, r.XmlTags),
					QuestionNo: r.FieldNo,
				})
			} else {
				tagMap1c[r.FundID] = append(tagMap1c[r.FundID], TagInfo{
					FundID:  r.FundID,
					XMLTags: string(r.XmlTags),
					Value:   ValueFor(*r.Value, r.LineItemType, r.FieldNo, r.FormLineNo, r.XmlTags),
				})
			}
		} else if qno < 5 {
			if r.LineItemType == "exp_table" {
				xmlTags := strings.Split(r.XmlTags, ",")
				var rows [][]string
				if err := json.Unmarshal([]byte(*r.Value), &rows); err != nil {
					logger.Log("ERROR", err.Error())
				}

				for _, row := range rows {
					for j, val := range row {
						tagMap1a[r.FundID] = append(tagMap1a[r.FundID], TagInfo{
							FundID:        r.FundID,
							XMLTags:       string(xmlTags[j]),
							Value:         val,
							QuestionNo:    r.FieldNo,
							QuestionSubNo: r.FormLineNo,
						})

					}
				}

			} else if qno == 3 {
				var aumtype string
				for key, val := range xmlTagAUM {
					if strings.Contains(r.FormLineNo, key) {
						aumtype = val
					}
				}
				tagMap1a[r.FundID] = append(tagMap1a[r.FundID], TagInfo{
					FundID:        r.FundID,
					XMLTags:       string(r.XmlTags),
					Value:         r.Value,
					QuestionNo:    r.FieldNo,
					QuestionSubNo: r.FormLineNo,
				})
				tagMap1a[r.FundID] = append(tagMap1a[r.FundID], TagInfo{
					FundID:        r.FundID,
					XMLTags:       "FundType",
					Value:         aumtype,
					QuestionNo:    r.FieldNo,
					QuestionSubNo: r.FormLineNo,
				})
			} else if qno == 0 {
				if r.FormLineNo == "0_C" {
					tagMap1a[r.FundID] = append(tagMap1a[r.FundID], TagInfo{
						FundID:        r.FundID,
						XMLTags:       string(r.XmlTags),
						Value:         r.Value,
						QuestionNo:    r.FieldNo,
						QuestionSubNo: r.FormLineNo,
					})
				} else if r.FormLineNo != "0" {
					tagMap1a[r.FundID] = append(tagMap1a[r.FundID], TagInfo{
						FundID:        r.FundID,
						XMLTags:       "ReportingPeriodType",
						Value:         reportingPeriod[strings.Split(r.FormLineNo, "_")[1]],
						QuestionNo:    r.FieldNo,
						QuestionSubNo: r.FormLineNo,
					})
					if r.Value != nil {
						tagMap1a[r.FundID] = append(tagMap1a[r.FundID], TagInfo{
							FundID:        r.FundID,
							XMLTags:       "FilingType",
							Value:         mcqlist[r.FormLineNo][*r.Value],
							QuestionNo:    r.FieldNo,
							QuestionSubNo: r.FormLineNo,
						})
					}
				}
			} else {
				tagMap1a[r.FundID] = append(tagMap1a[r.FundID], TagInfo{
					FundID:        r.FundID,
					XMLTags:       string(r.XmlTags),
					Value:         r.Value,
					QuestionNo:    r.FieldNo,
					QuestionSubNo: r.FormLineNo,
				})
			}

		} else {
			if *r.Value == "NA" && qno == 17 {
				continue
			}

			value := r.Value
			if string(r.XmlTags) == "CostBasedLiabilitiesAmount" && r.Value != nil {
				if v, err := strconv.ParseFloat(*r.Value, 64); err == nil && v != 0 {
					negated := strconv.FormatFloat(-math.Abs(v), 'f', -1, 64)
					//accounting for very small values
					if negated == "-0" || negated == "-0.0" {
						negated = "0"
					}
					value = &negated
				}
			}
			if string(r.XmlTags) == "NFAFundCFTCComplianceFlag" {
				if *value == "true" {
					*value = "Y"
				} else {
					*value = "N"
				}
			}
			tagMap1b[r.FundID] = append(tagMap1b[r.FundID], TagInfo{
				FundID:  r.FundID,
				XMLTags: string(r.XmlTags),
				Value:   value,
			})
		}
	}

	fundSlc := make([]FundInfo, 0, len(fundMap))
	tagSlc1a := make([][]TagInfo, 0, len(tagMap1a))
	tagSlc1b := make([][]TagInfo, 0, len(tagMap1b))
	tagSlc1c := make([][]TagInfo, 0, len(tagMap1c))

	for _, r := range rows {
		if fi, ok := fundMap[r.FundID]; ok {
			if r.FundID != "SECTION-1A" {
				fundSlc = append(fundSlc, fi)
			}
			if _, ok := tagMap1b[r.FundID]; ok {
				tagSlc1b = append(tagSlc1b, tagMap1b[r.FundID])
			}
			if _, ok := tagMap1c[r.FundID]; ok {
				tagSlc1c = append(tagSlc1c, tagMap1c[r.FundID])
			}
			if _, ok := tagMap1a[r.FundID]; ok {
				tagSlc1a = append(tagSlc1a, tagMap1a[r.FundID])
			}
			delete(fundMap, r.FundID)
		}
	}

	return &SimpleResponse{
		FundInfo:      fundSlc,
		TagInfoSec1a:  tagSlc1a,
		TagsInfoSec1b: tagSlc1b,
		TagInfoSec1c:  tagSlc1c,
	}, nil
}
