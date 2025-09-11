package modules

import (
	"fmt"
	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/360EntSecGroup-Skylar/excelize/v2"
	"github.com/texttheater/golang-levenshtein/levenshtein"
)

func IdentifyFiles(req *models.FileIdentificationRequest, uid string) (
	*models.FileIdentificationResponse, error) {

	logger.Log("INFO", fmt.Sprintf("start identification: %s / %s", req.ClientID, req.FilingName))

	funds, err := db.GetAllFegFileIdentifications()
	if err != nil {
		logger.Log("ERROR", "cannot load funds: "+err.Error())
		return nil, fmt.Errorf("cannot load funds: %w", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Log("ERROR", "loadconfig failed")
	}
	baseDir := filepath.Join(cfg.UploadVolumePath, req.ClientID, req.FilingName)
	excels, err := findExcelFiles(baseDir)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("cannot walk dir: %v", err.Error()))
		return nil, fmt.Errorf("cannot walk dir: %w", err)
	}
	logger.Log("INFO", fmt.Sprintf("found %d Excel files", len(excels)))

	var mapped []models.FileMappingDetail
	var unmapped []string
	var dbRows []models.FileMapper

	for _, path := range excels {
		name := filepath.Base(path)
		detail, ok := matchFile(path, name, funds)
		if !ok {
			unmapped = append(unmapped, name)
			logger.Log("WARNING", "unmapped file: "+name)
			continue
		}
		detail.FileName = name
		detail.FilePath = path
		mapped = append(mapped, detail)

		row := models.FileMapper{
			FileName:      detail.FileName,
			FilingPath:    detail.FilePath,
			FundShortCode: detail.ShortCode,
			FileMapID:     detail.MapID,
			QuarterNo:     detail.QuarterEndDate,
			CreatedAt:     time.Now().UTC(),
			CreatedBy:     uid,
		}
		dbRows = append(dbRows, row)
		logger.Log("INFO", fmt.Sprintf("mapped %s -> %s (%.1f%%)", name, detail.FundName, detail.MatchScore))
	}

	if len(dbRows) > 0 {
		if err := db.BatchSaveFileMappings(dbRows); err != nil {
			logger.Log("ERROR", "batch insert: "+err.Error())
			return nil, fmt.Errorf("batch insert: %w", err)
		}
		logger.Log("INFO", fmt.Sprintf("saved %d mappings to DB", len(dbRows)))
	}
	mappedCnt := len(mapped)
	procCnt := len(excels)
	msg := func() string {
		if mappedCnt == 0 {
			return "Invalid file format for one or more files"
		}
		if mappedCnt == procCnt {
			return "File mappings updated successfully"
		}
		return "Partial success"
	}()
	var finalMap []models.MappedResp
	var data models.MappedResp
	for _, j := range mapped {
		data.FileName = j.FileName
		data.FundName = j.FundName
		data.Quarter = j.QuarterEndDate
		data.ShortCode = j.ShortCode
		finalMap = append(finalMap, data)
	}
	return &models.FileIdentificationResponse{
		Message:       msg,
		FilingName:    req.FilingName,
		FileMappings:  finalMap,
		UnmappedFiles: unmapped,
	}, nil
}

// ---------- helpers ----------
func findExcelFiles(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(p string, i os.FileInfo, e error) error {
		if e != nil || i.IsDir() {
			return e
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".xlsx" || ext == ".xls" {
			out = append(out, p)
		}
		return nil
	})
	return out, err
}

func matchFile(path, name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
	// 1. filename first
	fundMatch, fundOk := matchFundByFilename(name, funds)
	quarter, qCell := matchQuarterByFilename(name)
	logger.Log("DEBuG", fmt.Sprintf("qcell: %v,fundOk: %v", fundOk, qCell))
	// 2. excel fallback for missing pieces
	if !fundOk || qCell == -1 {
		excelFund, excelQuarter, excelQCell := matchByExcelContent(path, name, funds)
		logger.Log("DEBUG", fmt.Sprintf("excelQuarter:%v", excelQuarter))
		if !fundOk && excelFund.MatchScore > 0 {
			fundMatch = excelFund
			fundOk = true
		}
		if qCell == -1 && excelQuarter != 0 {
			logger.Log("DEBUG", fmt.Sprintf("qt: %v", excelQuarter))
			quarter, qCell = excelQuarter, excelQCell
		}
	}

	// 3. merge
	out := models.FileMappingDetail{
		FileName: name, FilePath: path,
		QuarterEndDate: quarter, DateCellNo: qCell,
	}
	if fundOk {
		out.FundName = fundMatch.FundName
		out.ShortCode = fundMatch.ShortCode
		out.MatchMethod = fundMatch.MatchMethod
		out.MatchScore = fundMatch.MatchScore
		out.EntityFound = fundMatch.EntityFound
		out.FundCellNo = fundMatch.FundCellNo
	}
	return out, quarter != 0 || fundOk
}

// ---------- filename fund extractor ----------
func matchFundByFilename(name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
	parts := strings.Split(strings.TrimSuffix(name, filepath.Ext(name)), "_")
	logger.Log("DEBUG", fmt.Sprintf("parts: %v", parts))
	for _, part := range parts {
		candidate := strings.ToLower(part)
		logger.Log("DEBUG", fmt.Sprintf("part: %v", candidate))
		for _, f := range funds {
			if f.ShortCode == "" || f.Quarter == 0 {
				continue
			}
			if similarity(candidate, strings.ToLower(f.ShortCode)) >= 85 {
				return models.FileMappingDetail{
					MapID:       f.ID,
					FundName:    f.FundName,
					ShortCode:   f.ShortCode,
					MatchMethod: "filename",
					MatchScore:  similarity(candidate, strings.ToLower(f.ShortCode)),
				}, true
			}
		}
	}
	return models.FileMappingDetail{}, false
}

func matchQuarterByFilename(name string) (int, int) {
	parts := strings.Split(strings.TrimSuffix(name, filepath.Ext(name)), "_")
	for i, p := range parts {
		if t, err := time.Parse("2006.01.02", strings.TrimSpace(p)); err == nil {
			return quarterFromDate(&t), i
		}
	}
	return 0, -1
}

// ---------- excel extractor (returns both pieces) ----------
func matchByExcelContent(path, name string, funds []models.FegFileIdentification) (fund models.FileMappingDetail, quarter int, qCell int) {
	logger.Log("DEBUG", "Inside excel content")
	logger.Log("INFO", fmt.Sprintf("filename: %v", name))

	f, err := excelize.OpenFile(path)
	if err != nil {
		logger.Log("DEBUG", "fileopen error")
		return models.FileMappingDetail{}, 0, -1
	}

	quarter, qCell = matchQuarterFromExcel(f)
	fund, _ = matchFundFromExcel(f, funds)
	return fund, quarter, qCell
}

func matchQuarterFromExcel(f *excelize.File) (int, int) {
	parse := func(raw string) int {
		raw = strings.TrimSpace(raw)
		// normalise separators to "/"
		raw = strings.NewReplacer(".", "/", "-", "/").Replace(raw)

		// try the four common layouts
		for _, l := range []string{"01/02/2006", "1/2/2006", "01/02/06", "1/2/06"} {
			if t, err := time.Parse(l, raw); err == nil {
				// two-digit-year fix: 00-29 → 20xx, 30-99 → 19xx
				if l == "01/02/06" || l == "1/2/06" {
					if t.Year() < 1970 {
						t = t.AddDate(100, 0, 0)
					}
				}
				return quarterFromDate(&t)
			}
		}
		return 0
	}
	for _, sheet := range f.GetSheetList() {
		rows, _ := f.GetRows(sheet)
		for _, row := range rows {
			for ci, cell := range row {
				logger.Log("DEBUG", cell)
				if strings.Contains(strings.ToLower(strings.TrimSpace(cell)), "period to:") &&
					ci+1 < len(row) {
					logger.Log("DEBUG", "period found")
					fmt.Println(row[ci+2])
					fmt.Println(ci + 2)
					fmt.Println(parse(row[ci+2]))
					return parse(row[ci+2]), ci + 2
				}
			}
		}
	}
	return 0, -1
}

func matchFundFromExcel(f *excelize.File, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
	var best models.FileMappingDetail
	for _, sheet := range f.GetSheetList() {
		rows, _ := f.GetRows(sheet)
		for _, row := range rows {
			for ci, cell := range row {
				if cell = strings.TrimSpace(cell); cell == "" {
					continue
				}
				lc := strings.ToLower(cell)
				for _, fund := range funds {
					nScore := similarity(lc, strings.ToLower(fund.FundName))
					cScore := similarity(lc, strings.ToLower(fund.ShortCode))

					if nScore >= 90 && nScore > best.MatchScore {
						best = models.FileMappingDetail{
							MapID:       fund.ID,
							FundName:    fund.FundName,
							ShortCode:   fund.ShortCode,
							MatchMethod: "ExcelContent",
							MatchScore:  nScore,
							EntityFound: cell,
							FundCellNo:  ci,
						}
					}
					if cScore >= 90 && cScore > best.MatchScore {
						best = models.FileMappingDetail{
							MapID:       fund.ID,
							FundName:    fund.FundName,
							ShortCode:   fund.ShortCode,
							MatchMethod: "ExcelContentShort",
							MatchScore:  cScore,
							EntityFound: cell,
							FundCellNo:  ci,
						}
					}
				}
			}
		}
	}
	return best, best.MatchScore > 0
}

func quarterFromDate(t *time.Time) int {
	if t == nil {
		return 0
	}
	switch m := t.Month(); {
	case m <= 3:
		return 1
	case m <= 6:
		return 2
	case m <= 9:
		return 3
	default:
		return 4
	}
}

func similarity(a, b string) float64 {
	if a == b {
		return 100
	}
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	if maxLen == 0 {
		return 100
	}
	d := levenshtein.DistanceForStrings([]rune(a), []rune(b), levenshtein.DefaultOptions)
	return (1 - float64(d)/float64(maxLen)) * 100
}
