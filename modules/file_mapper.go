package modules

import (
	"errors"
	"fmt"
	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/brandenc40/romannumeral"
)

var (
	dateRe      = regexp.MustCompile(`(\d{4})[.\-\/\\](0[1-9]|1[0-2])[.\-\/\\](0[1-9]|[12][0-9]|3[01])`)
	romanRe     = regexp.MustCompile(`\b[MDCLXVI]+\b`)
	fundRomanRe = regexp.MustCompile(`[MDCLXVI]+`)
	q1to5       = regexp.MustCompile(`^Q[1-5]$`)
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
	if len(excels) == 0 {
		return nil, errors.New("no Excel files found")
	}
	candidates, unmapped := shortCodesFromFiles(excels, funds)
	// fmt.Println(unmapped)
	// for key, files := range candidates {
	// 	for _, file := range files {
	// 		logger.Log("DEBUG", fmt.Sprintf("\n key:%v files:%vhi\n", key, file))
	// 	}
	// }
	if len(candidates) == 0 {
		return nil, errors.New("no recognisable short-codes in file names")
	}
	var scoped []models.FegFileIdentification
	for _, fund := range funds {
		if _, ok := candidates[strings.ToUpper(fund.ShortCode)]; ok {
			// fmt.Printf("scoped fundname:%vhi,sc:%vhi", fund.FundName, fund.ShortCode)
			scoped = append(scoped, fund)
		}
	}
	if len(scoped) == 0 {
		return nil, errors.New("upload files do not match any known fund")
	}
	series := decideSeries(scoped)
	if series == "" {
		return nil, errors.New("upload contains funds from multiple series – please upload single series at a time")
	}
	mapped := FindFileType(candidates, funds, series)

	// type result struct {
	// 	detail models.MappedResp
	// 	ok     bool
	// }
	// jobs := make(chan job, len(excels))
	// fmt.Printf("job:%vhi\n", jobs)
	// res := make(chan result, len(excels))
	// workers := 16
	// if len(excels) < workers {
	// 	workers = len(excels)
	// }
	// var wg sync.WaitGroup
	// for i := 0; i < workers; i++ {
	// 	wg.Add(1)
	// 	go func() {
	// 		defer wg.Done()
	// 		for j := range jobs {
	// 			detail, ok := matchFile(j.name, scoped)
	// 			detail.FileName = j.name
	// 			detail.FilePath = j.path
	// 			res <- result{detail, ok}
	// 		}
	// 	}()
	// }
	// for _, p := range excels {
	// 	jobs <- job{p, filepath.Base(p)}
	// }
	// close(jobs)
	// wg.Wait()
	// close(res)
	// var mapped, unmapped []models.FileMappingDetail
	// for r := range res {
	// 	if r.ok {
	// 		mapped = append(mapped, r.detail)
	// 	} else {
	// 		unmapped = append(unmapped, r.detail)
	// 	}
	// }
	msg := "File mappings updated successfully"
	if len(mapped) == 0 {
		msg = "Invalid file format for one or more files"
	} else if len(unmapped) > 0 {
		msg = "Partial success"
	}
	// toResp := func(src []models.FileMappingDetail) []models.MappedResp {
	// 	out := make([]models.MappedResp, len(src))
	// 	for i, v := range src {
	// 		out[i] = models.MappedResp{
	// 			FileName:  v.FileName,
	// 			FundName:  v.FundName,
	// 			FileType:  v.FileType,
	// 			ShortCode: v.ShortCode,
	// 			Path:      strings.TrimPrefix(v.FilePath, os.Getenv("STORAGE_PATH")),
	// 		}
	// 	}
	// 	return out
	// }

	return &models.FileIdentificationResponse{
		Message:       msg,
		FilingName:    req.FilingName,
		FileMappings:  mapped,
		UnmappedFiles: unmapped,
		FundDetails:   toFundResp(funds, series),
	}, nil
}

func findExcelFiles(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(p string, i os.FileInfo, e error) error {
		if e != nil || i.IsDir() {
			return e
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".xlsx" || ext == ".xls" || ext == ".pdf" {
			out = append(out, p)
		}
		return nil
	})
	return out, err
}

func FindFileType(fundMap map[string][]string, funds []models.FegFileIdentification, series string) []models.MappedResp {
	// seen := make(map[string]bool) // key = shortCode+"|"+filePath
	var out []models.MappedResp

	// longest code first to avoid FSAE vs FSAE2 overlap
	sort.Slice(funds, func(i, j int) bool {
		return len(funds[i].ShortCode) > len(funds[j].ShortCode)
	})

	for _, fund := range funds {
		files, ok := fundMap[fund.ShortCode]
		if !ok {
			continue
		}
		for _, file := range files {

			var resp models.MappedResp
			resp.FundName = fund.FundName
			resp.ShortCode = fund.ShortCode
			resp.FileName = filepath.Base(file)
			resp.Path = strings.TrimPrefix(file, os.Getenv("STORAGE_PATH"))

			if q1to5.MatchString(fund.FileType) {
				quarter, loc := matchQuarterByFilename(resp.FileName)
				resp.FileType = "Q" + strconv.Itoa(quarter)
				if loc != -1 && resp.FileType == fund.FileType {
					fmt.Printf("fndname:%vhi\n,filename:%vhi\n", fund.FundName, resp.FileName)

					out = append(out, resp)
				}

			} else {
				if matchByidentifier(strings.ToLower(resp.FileName), strings.ToLower(fund.FileType)) {
					resp.FileType = fund.FileType
				}
				out = append(out, resp)
			}

		}

	}
	return out
}

func matchByidentifier(filename string, filetype string) bool {
	flg := true
	if filetype == "na" {
		fmt.Println("hi got into NA")
		return true
	}
	identifiers := strings.Split(filetype, "_")
	for _, id := range identifiers {
		if !strings.Contains(filename, id) {
			fmt.Printf("filename:%vhi,id:%vhi\n", filename, id)
			flg = false
		}
	}
	return flg
}

func matchFile(name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
	series := decideSeries(funds)
	if series == "" {
		return models.FileMappingDetail{}, false
	}
	switch series {
	case "Private Investors":
		return matchPrivateInvestors(name, funds)
	case "Investment Services":
		return matchInvestmentServices(name, funds)
	default:
		return models.FileMappingDetail{}, false
	}
}

func decideSeries(funds []models.FegFileIdentification) string {
	var concrete []string
	for _, f := range funds {
		if strings.TrimSpace(f.Series) != "ALL" {
			concrete = append(concrete, strings.TrimSpace(f.Series))
		}
	}
	if len(concrete) == 0 {
		return "" // no anchor series
	}
	first := concrete[0]
	for _, s := range concrete[1:] {
		if s != first {
			return "" // mixed
		}
	}
	return first // uniform concrete series
}

func matchPrivateInvestors(name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
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
	}
	return out, fundOk && qCell != -1
}

func matchInvestmentServices(name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
	fund, ok := matchFundByFilename(name, funds)
	if !ok {
		return models.FileMappingDetail{}, false
	}
	fileType := classifyInvestmentFile(name)
	if fileType == "unknown" {
		return models.FileMappingDetail{}, false
	}
	return models.FileMappingDetail{
		FileName:    name,
		FundName:    fund.FundName,
		ShortCode:   fund.ShortCode,
		MatchMethod: "filename",
		FileType:    fileType,
	}, true
}

func classifyInvestmentFile(name string) string {
	ln := strings.ToLower(name)
	switch {
	case strings.Contains(ln, "nav") && strings.Contains(ln, "tb"):
		return "nav_tb"
	case strings.Contains(ln, "investors") && strings.Contains(ln, "capital register"):
		return "capital_register"
	case strings.Contains(ln, "nav") && strings.Contains(ln, "report"):
		return "nav_report"
	default:
		return "unknown"
	}
}

func matchFundByFilename(name string, funds []models.FegFileIdentification) (models.FileMappingDetail, bool) {
	ln := strings.ToLower(normalizeRomans(name))
	// 1.  longest code first
	sort.Slice(funds, func(i, j int) bool {
		return len(funds[i].ShortCode) > len(funds[j].ShortCode)
	})
	for _, f := range funds {
		sc := strings.ToLower(normalizeRomans(f.ShortCode))
		if strings.Contains(ln, sc) || strings.Contains(ln, strings.ToLower(f.ShortCode)) || strings.Contains(ln, strings.ToLower(f.FundName)) {
			return models.FileMappingDetail{
				MapID:       f.ID,
				FundName:    f.FundName,
				ShortCode:   f.ShortCode,
				MatchMethod: "filename",
			}, true
		}
	}
	return models.FileMappingDetail{}, false
}

func matchQuarterByFilename(name string) (int, int) {
	re := regexp.MustCompile(`(?:19|20)\d{2}[.\-\/\\](0?[1-9]|1[0-2])`)
	if loc := re.FindStringIndex(name); loc != nil {
		mmStr := name[loc[0]+5 : loc[0]+7]
		if mm, _ := strconv.Atoi(mmStr); mm >= 1 && mm <= 12 {
			return (mm-1)/3 + 1, loc[0]
		}
	}
	return -1, -1
}

func toFundResp(funds []models.FegFileIdentification, series string) []models.MappedResp {
	var out []models.MappedResp
	for _, f := range funds {
		if strings.EqualFold(strings.TrimSpace(f.Series), strings.TrimSpace(series)) || f.Series == "ALL" {
			out = append(out, models.MappedResp{
				FundName:  f.FundName,
				FileType:  f.FileType,
				ShortCode: f.ShortCode,
			})
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

func shortCodesFromFiles(paths []string, funds []models.FegFileIdentification) (map[string][]string, []models.MappedResp) {
	out := make(map[string][]string)

	unmapped := []models.MappedResp{}
	// longest first
	sort.Slice(funds, func(i, j int) bool {
		return len(funds[i].ShortCode) > len(funds[j].ShortCode)
	})
	for _, p := range paths {
		flg := false
		file := models.MappedResp{}
		ogBase := strings.ToLower(filepath.Base(p))
		base := strings.ToLower(normalizeRomans(filepath.Base(p)))
		for _, f := range funds {
			ogSc := strings.ToLower(f.ShortCode)
			ogFname := strings.ToLower(f.FundName)
			sc := strings.ToLower(normalizeRomans(f.ShortCode))
			name := strings.ToLower(normalizeRomans(f.FundName))
			// logger.Log("DEBUG", fmt.Sprintf("\nogBase:%v,base%v\n", ogBase, base))
			// logger.Log("DEBUG", fmt.Sprintf("\nogSc:%vhi,ogFname:%vhi,sc:%vhi,name:%vhi\n", ogSc, ogFname, sc, name))
			if strings.Contains(ogBase, ogFname) || strings.Contains(ogBase, ogSc) || strings.Contains(base, sc) || strings.Contains(base, name) {
				k := strings.ToUpper(f.ShortCode)
				out[k] = append(out[k], p)
				flg = true
				break
			}
		}
		if !flg {
			file.FileName = filepath.Base(p)
			file.Path = p
			unmapped = append(unmapped, file)
		}
	}
	return out, unmapped
}
