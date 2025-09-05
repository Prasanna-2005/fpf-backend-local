package modules

import (
	"fmt"
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

type DocumentProcessingService struct {
	logger *utility.Logger
}

func NewDocumentProcessingService() *DocumentProcessingService {
	return &DocumentProcessingService{
		logger: utility.LoggerFunc(), // <-- your logger
	}
}

// ----------  main entry point  ----------
func (s *DocumentProcessingService) IdentifyFiles(req *models.FileIdentificationRequest) (
	*models.FileIdentificationResponse, error) {

	s.logger.Log("INFO", fmt.Sprintf("start identification: %s / %s", req.ClientID, req.FilingName))

	funds, err := db.GetAllFegFileIdentifications()
	if err != nil {
		s.logger.Log("ERROR", "cannot load funds: "+err.Error())
		return nil, fmt.Errorf("cannot load funds: %w", err)
	}
	headers, err := db.GetAllFegFileIdentifications()
	if err != nil {
		s.logger.Log("ERROR", "cannot load headers: "+err.Error())
		return nil, fmt.Errorf("cannot load headers: %w", err)
	}

	baseDir := filepath.Join("../data", req.ClientID, req.FilingName)
	excels, err := s.findExcelFiles(baseDir)
	if err != nil {
		s.logger.Log("ERROR", "cannot walk dir: "+err.Error())
		return nil, fmt.Errorf("cannot walk dir: %w", err)
	}
	s.logger.Log("INFO", fmt.Sprintf("found %d Excel files", len(excels)))

	var mapped []models.FileMappingDetail
	var unmapped []string
	var dbRows []models.FileMapper

	for _, path := range excels {
		name := filepath.Base(path)
		detail, ok := s.matchFile(path, name, funds)
		if !ok {
			unmapped = append(unmapped, name)
			s.logger.Log("WARNING", "unmapped file: "+name)
			continue
		}
		detail.FileName = name
		detail.FilePath = path
		mapped = append(mapped, detail)

		headerID, err := s.findOrCreateHeader(detail.FundName, detail.QuarterEndDate, headers)
		if err != nil {
			s.logger.Log("ERROR", "header create: "+err.Error())
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
		s.logger.Log("DEBUG", fmt.Sprintf("headerID: %v", headerID))
		s.logger.Log("INFO", fmt.Sprintf("mapped %s -> %s (%.1f%%)", name, detail.FundName, detail.MatchScore))
	}

	if len(dbRows) > 0 {
		if err := db.BatchSaveFileMappings(dbRows); err != nil {
			s.logger.Log("ERROR", "batch insert: "+err.Error())
			return nil, fmt.Errorf("batch insert: %w", err)
		}
		s.logger.Log("INFO", fmt.Sprintf("saved %d mappings to DB", len(dbRows)))
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
func (s *DocumentProcessingService) findExcelFiles(root string) ([]string, error) {
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

func (s *DocumentProcessingService) matchFile(path, name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
	if d, ok := s.matchByFilename(name, funds); ok {
		logger.Log("DEBUG", fmt.Sprintf("name: %v, ok: %v", name, ok))
		return d, true
	}
	return s.matchByExcelContent(path, name, funds)
}

// func (s *DocumentProcessingService) matchByFilename(name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
// 	parts := strings.Split(strings.TrimSuffix(name, filepath.Ext(name)), "_")
// 	if len(parts) < 3 {
// 		return models.FileMappingDetail{}, false
// 	}
// 	candidate := strings.ToLower(parts[2])
// 	logger.Log("DEBUG", fmt.Sprintf("candidate:%v", candidate))
// 	for _, f := range funds {
// 		logger.Log("DEBUG", fmt.Sprintf("funds: %v", funds))
// 		logger.Log("DEBUG", "Entered for loop")
// 		if f.ShortCode == "" {
// 			logger.Log("DEBUG", "no short code present")
// 			continue
// 		}
// 		score := s.similarity(candidate, strings.ToLower(f.ShortCode))

// 		logger.Log("DEBUG", fmt.Sprintf("shortCode:%v", strings.ToLower(f.ShortCode)))
// 		logger.Log("DEBUG", fmt.Sprintf("score:%v", score))
// 		if score >= 85 {
// 			return models.FileMappingDetail{
// 				FundName:    f.FundName,
// 				ShortCode:   f.ShortCode,
// 				MatchMethod: "filename",
// 				MatchScore:  score,
// 			}, true
// 		}
// 	}
// 	return models.FileMappingDetail{}, false
// }

func (s *DocumentProcessingService) matchByFilename(name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
	parts := strings.Split(strings.TrimSuffix(name, filepath.Ext(name)), "_")
	if len(parts) < 3 { // <-- guard first
		return models.FileMappingDetail{}, false
	}
	candidate := strings.ToLower(parts[2]) // <-- now safe
	s.logger.Log("DEBUG", fmt.Sprintf("candidate:%v", candidate))
	for _, f := range funds {
		if f.ShortCode == "" {
			continue
		}
		score := s.similarity(candidate, strings.ToLower(f.ShortCode))
		if score >= 85 {
			return models.FileMappingDetail{
				FundName:    f.FundName,
				ShortCode:   f.ShortCode,
				MatchMethod: "filename",
				MatchScore:  score,
			}, true
		}
	}
	return models.FileMappingDetail{}, false // explicit failure → triggers Excel scan
}

func (s *DocumentProcessingService) matchByExcelContent(path, name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return models.FileMappingDetail{}, false
	}

	var best models.FileMappingDetail
	for _, sheet := range f.GetSheetList() {
		rows, _ := f.GetRows(sheet)
		for _, row := range rows {
			for ci, cell := range row {
				if !strings.Contains(strings.ToLower(cell), "entity") {
					continue
				}
				val := strings.TrimSpace(s.adjacent(rows, ci))
				if val == "" {
					continue
				}
				for _, fund := range funds {
					nameScore := s.similarity(strings.ToLower(val), strings.ToLower(fund.FundName))
					codeScore := s.similarity(strings.ToLower(val), strings.ToLower(fund.ShortCode))
					if nameScore >= 90 && nameScore > best.MatchScore {
						best = models.FileMappingDetail{FundName: fund.FundName, ShortCode: fund.ShortCode, MatchMethod: "entity", MatchScore: nameScore, EntityFound: val}
					}
					if codeScore >= 90 && codeScore > best.MatchScore {
						best = models.FileMappingDetail{FundName: fund.FundName, ShortCode: fund.ShortCode, MatchMethod: "entity", MatchScore: codeScore, EntityFound: val}
					}
				}
			}
		}
	}
	return best, best.MatchScore > 0
}

func (s *DocumentProcessingService) findOrCreateHeader(fundName string, qdate *time.Time, cache []models.FegFileIdentification) (uint, error) {
	q := s.quarterFromDate(qdate)
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
	s.logger.Log("INFO", fmt.Sprintf("created new FegFileIdentification %s Q%d (id=%d)", fundName, q, newH.ID))
	return newH.ID, nil
}

func (s *DocumentProcessingService) quarterFromDate(t *time.Time) int {
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

func (s *DocumentProcessingService) similarity(a, b string) float64 {
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

func (s *DocumentProcessingService) adjacent(rows [][]string, col int) string {
	if col+1 < len(rows[0]) {
		return rows[0][col+1]
	}
	return ""
}
