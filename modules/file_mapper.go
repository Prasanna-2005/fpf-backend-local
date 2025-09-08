package modules

import (
	"fmt"
	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"form_pf_backend/utility"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/360EntSecGroup-Skylar/excelize/v2"
	"github.com/texttheater/golang-levenshtein/levenshtein"
)

func IdentifyFiles(req *models.FileIdentificationRequest) (
	*models.FileIdentificationResponse, error) {

	logger.Log("INFO", fmt.Sprintf("start identification: %s / %s", req.ClientID, req.FilingName))

	funds, err := db.GetAllFegFileIdentifications()
	if err != nil {
		logger.Log("ERROR", "cannot load funds: "+err.Error())
		return nil, fmt.Errorf("cannot load funds: %w", err)
	}

	cfg, err := config.LoadConfig()
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

		headerID, err := findOrCreateHeader(detail.FundName, detail.QuarterEndDate, headers)
		if err != nil {
			logger.Log("ERROR", "header create: "+err.Error())
			continue
		}

		// dbRows = append(dbRows, models.FileMapper{
		// 	FileName:       name,
		// 	FilePath:       path,
		// 	MatchMethod:    detail.MatchMethod,
		// 	MatchScore:     detail.MatchScore,
		// 	QuarterEndDate: detail.QuarterEndDate,
		// 	EntityFound:    detail.EntityFound,
		// })
		logger.Log("DEBUG", fmt.Sprintf("headerID: %v", headerID))
		logger.Log("INFO", fmt.Sprintf("mapped %s -> %s (%.1f%%)", name, detail.FundName, detail.MatchScore))
	}

	if len(dbRows) > 0 {
		if err := db.BatchSaveFileMappings(dbRows); err != nil {
			logger.Log("ERROR", "batch insert: "+err.Error())
			return nil, fmt.Errorf("batch insert: %w", err)
		}
		logger.Log("INFO", fmt.Sprintf("saved %d mappings to DB", len(dbRows)))
	}

	return &models.FileIdentificationResponse{
		Success:        true,
		Message:        fmt.Sprintf("processed %d, mapped %d", len(excels), len(mapped)),
		FileMappings:   mapped,
		UnmappedFiles:  unmapped,
		ProcessedCount: len(excels),
		MappedCount:    len(mapped),
	}, nil
}

// ----------  helpers  ----------
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
	if d, ok := matchByFilename(name, funds); ok {
		return d, true
	}
	return matchByExcelContent(path, name, funds)
}

func matchByFilename(name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
	parts := strings.Split(strings.TrimSuffix(name, filepath.Ext(name)), "_")
	logger.Log("DEBUG", fmt.Sprintf("parts: %v", parts))
	q, m := utility.ThisYear()
	for _, part := range parts {
		candidate := strings.ToLower(part) // <-- now safe
		logger.Log("DEBUG", fmt.Sprintf("candidate:%v", candidate))
		for _, f := range funds {
			if f.ShortCode == "" || f.Quarter == 0 {
				continue
			}
			score := similarity(candidate, strings.ToLower(f.ShortCode))
			quarterScore := similarity(candidate())
			if score >= 85 {
				return models.FileMappingDetail{
					FundName:    f.FundName,
					ShortCode:   f.ShortCode,
					MatchMethod: "filename",
					MatchScore:  score,
				}, true
			}
		}
	}
	return models.FileMappingDetail{}, false // explicit failure → triggers Excel scan
}

func matchByExcelContent(path, name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
	logger.Log("DEBUG", "Inside excel content")
	f, err := excelize.OpenFile(path)
	if err != nil {
		logger.Log("DEBUG", "fileopen error")

		return models.FileMappingDetail{}, false
	}

	var best models.FileMappingDetail
	for _, sheet := range f.GetSheetList() {
		logger.Log("DEBUG", fmt.Sprintf("sheet: %v", sheet))
		rows, _ := f.GetRows(sheet)

		for _, row := range rows {
			for ci, cell := range row {
				// cell=strings.TrimSpace(cell)
				// if !strings.Contains(strings.ToLower(cell), "entity:") {
				// 	continue
				// }
				// val := strings.TrimSpace(s.adjacent(rows, ci))
				val := strings.TrimSpace(cell)
				if val == "" {
					continue
				}
				for _, fund := range funds {
					nameScore := similarity(strings.ToLower(val), strings.ToLower(fund.FundName))
					codeScore := similarity(strings.ToLower(val), strings.ToLower(fund.ShortCode))
					if nameScore >= 90 && nameScore > best.MatchScore {
						best = models.FileMappingDetail{FundName: fund.FundName, ShortCode: fund.ShortCode, MatchMethod: "ExcelContent", MatchScore: nameScore, EntityFound: val, CellNo: ci}
					}
					if codeScore >= 90 && codeScore > best.MatchScore {
						best = models.FileMappingDetail{FundName: fund.FundName, ShortCode: fund.ShortCode, MatchMethod: "ExcelContentShort", MatchScore: codeScore, EntityFound: val, CellNo: ci}
					}
				}
			}
		}
	}
	return best, best.MatchScore > 0
}

func findOrCreateHeader(fundName string, qdate *time.Time, cache []models.FegFileIdentification) (uint, error) {
	q := quarterFromDate(qdate)
	for _, h := range cache {
		if h.FundName == fundName && h.Quarter == q {
			return h.ID, nil
		}
	}
	newH := models.FegFileIdentification{
		FundName: fundName,
		Quarter:  1,

		Series:    "A",
		CreatedBy: "system",
	}
	if err := db.CreateFegFileIdentification(&newH); err != nil {
		return 0, err
	}
	logger.Log("INFO", fmt.Sprintf("created new FegFileIdentification %s Q%d (id=%d)", fundName, q, newH.ID))
	return newH.ID, nil
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
