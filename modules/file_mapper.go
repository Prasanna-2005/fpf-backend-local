package modules

import (
	"fmt"
	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brandenc40/romannumeral"
)

func IdentifyFiles(req *models.FileIdentificationRequest, cid string) (*models.FileIdentificationResponse, error) {
	funds, err := db.GetAllFegFileIdentifications()
	if err != nil {
		return nil, fmt.Errorf("cannot load funds: %w", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("config error")
	}

	baseDir := filepath.Join(cfg.UploadVolumePath, cid, sanitize(req.FilingName))
	excels, err := findExcelFiles(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot walk dir: %w", err)
	}

	type job struct{ path, name string }
	type result struct {
		detail models.FileMappingDetail
		ok     bool
	}

	jobs := make(chan job, len(excels))
	res := make(chan result, len(excels))

	workers := 16
	if len(excels) < workers {
		workers = len(excels)
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				detail, ok := matchFile(j.name, funds)
				detail.FileName = j.name
				detail.FilePath = j.path
				res <- result{detail, ok}
			}
		}()
	}

	for _, p := range excels {
		jobs <- job{p, filepath.Base(p)}
	}
	close(jobs)
	wg.Wait()
	close(res)

	var mapped, unmapped []models.FileMappingDetail
	for r := range res {
		if r.ok {
			mapped = append(mapped, r.detail)
		} else {
			unmapped = append(unmapped, r.detail)
		}
	}
	// fmt.Println(mapped)
	msg := "File mappings updated successfully"
	if len(mapped) == 0 {
		msg = "Invalid file format for one or more files"
	} else if len(unmapped) > 0 {
		msg = "Partial success"
	}

	toResp := func(src []models.FileMappingDetail) []models.MappedResp {
		out := make([]models.MappedResp, len(src))
		for i, v := range src {
			out[i] = models.MappedResp{
				FileName:  v.FileName,
				FundName:  v.FundName,
				Quarter:   v.Quarter,
				ShortCode: v.ShortCode,
				Path:      strings.TrimPrefix(v.FilePath, os.Getenv("STORAGE_PATH")),
			}
		}

		return out
	}

	return &models.FileIdentificationResponse{
		Message:       msg,
		FilingName:    req.FilingName,
		FileMappings:  toResp(mapped),
		UnmappedFiles: toResp(unmapped),
		FundDetails:   toFundResp(funds),
	}, nil
}

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

func matchFile(name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
	var out models.FileMappingDetail
	fund, fundOk := matchFundByFilename(name, funds)
	quarter, qCell := matchQuarterByFilename(name)
	if fund.ShortCode == "Q15_Q16" {
		quarter = 0
		qCell = 0
	}
	out = models.FileMappingDetail{
		FileName:   name,
		Quarter:    quarter,
		DateCellNo: qCell,
	}
	if fundOk {
		out.FundName = fund.FundName
		out.ShortCode = fund.ShortCode
		out.MatchMethod = "filename"
		// out.MatchScore = 100
	}
	// logger.Log("INFO", fmt.Sprintf("Fund %v got mapped to %v for timeperiod %v", out.FundName, out.FileName, out.Quarter))
	return out, fundOk && qCell != -1
}

func matchFundByFilename(name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
	ln := strings.ToLower(normalizeRomans(name))
	name = strings.ToLower(name)
	for _, f := range funds {
		if strings.Contains(ln, strings.ToLower(f.ShortCode)) || strings.Contains(ln, strings.ToLower(f.FundName)) || strings.Contains(name, strings.ToLower(f.FundName)) || strings.Contains(name, strings.ToLower(f.ShortCode)) {
			return models.FileMappingDetail{
				MapID:       f.ID,
				FundName:    f.FundName,
				ShortCode:   f.ShortCode,
				MatchMethod: "filename",
				// MatchScore:  100,
			}, true

		}
	}
	return models.FileMappingDetail{}, false
}

func matchQuarterByFilename(name string) (int, int) {
	// grab yyyy<sep>MM anywhere; ignore day
	re := regexp.MustCompile(`(?:19|20)\d{2}[.\-\/\\](0[1-9]|1[0-2])`)
	if loc := re.FindStringIndex(name); loc != nil {
		mmStr := name[loc[0]+5 : loc[0]+7] // yyyy. -> MM
		if mm, _ := strconv.Atoi(mmStr); mm >= 1 && mm <= 12 {
			return (mm-1)/3 + 1, loc[0]
		}
	}
	return -1, -1
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

func toFundResp(funds []models.FegFileIdentification) []models.MappedResp {
	out := make([]models.MappedResp, len(funds))
	for i, f := range funds {
		out[i] = models.MappedResp{
			FundName:  f.FundName,
			Quarter:   f.Quarter,
			ShortCode: f.ShortCode,
		}
	}
	return out
}

func normalizeRomans(s string) string {
	return fundRomanRe.ReplaceAllStringFunc(s, func(r string) string {
		if v, err := romannumeral.StringToInt(r); err == nil {
			return strconv.Itoa(v)
		}
		return r
	})
}

var (
	dateRe      = regexp.MustCompile(`(\d{4})[.\-\/\\](0[1-9]|1[0-2])[.\-\/\\](0[1-9]|[12][0-9]|3[01])`)
	romanRe     = regexp.MustCompile(`\b[MDCLXVI]+\b`)
	fundRomanRe = regexp.MustCompile(`[MDCLXVI]+`)
)
