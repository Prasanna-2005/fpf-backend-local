package modules

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
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
var FileTypeConv = map[string]string{
	"capital_register":     "Capital Register",
	"nav_tb":               "Trial Balance",
	"nav_report":           "Performance report",
	"Unfunded_Commitments": "Unfunded_Commitments",
	"NA":                   "NA",
	"Q3":                   "Q3",
	"Q2":                   "Q2",
	"Q1":                   "Q1",
}

type Payload struct {
	PDFPaths []string `json:"pdf_paths"`
	ExcelDir string   `json:"excel_dir"`
}

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
	excels, pdf, err := findFiles(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot walk dir: %w", err)
	}
	if len(pdf) != 0 {
		excelDir := filepath.Join(baseDir, "convPdf")
		pl := Payload{PDFPaths: pdf, ExcelDir: excelDir}
		payload, _ := json.Marshal(pl)

		scriptPath := "modules/pymupdf_table_extraction.py"
		cmd := exec.Command("python", scriptPath)
		cmd.Stdin = bytes.NewReader(payload)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil { // blocks until completion
			if exitErr, ok := err.(*exec.ExitError); ok {
				fmt.Errorf("table extractor failed with exit code %d", exitErr.ExitCode())
			}
			fmt.Errorf("table extractor failed: %w", err)
		}

		for _, pdfPath := range pdf {
			convExcelPath := filepath.Join(excelDir, strings.TrimSuffix(filepath.Base(pdfPath), ".pdf")+".xlsx")
			excels = append(excels, convExcelPath)
		}
	}

	if len(excels) == 0 {
		return nil, errors.New("no Excel files found")
	}
	candidates, unmapped := shortCodesFromFiles(excels, funds)
	fmt.Println(unmapped)
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
	// fmt.Println("shortcide identification")
	// for k, v := range candidates {
	// 	fmt.Printf("fund:%vhi,files:%v\n", k, v)
	// }

	mapped, failedFiles := FindFileType(candidates, funds, series)
	unmapped = append(unmapped, failedFiles...)

	msg := "File mappings updated successfully"
	if len(mapped) == 0 {
		msg = "Invalid file format for one or more files"
	} else if len(unmapped) > 0 {
		msg = "Partial success"
	}
	return &models.FileIdentificationResponse{
		Message:       msg,
		Series:        series,
		FilingName:    req.FilingName,
		FileMappings:  mapped,
		UnmappedFiles: unmapped,
		FundDetails:   toFundResp(funds, series),
	}, nil
}

func findFiles(root string) ([]string, []string, error) {
	var out, pdf []string
	err := filepath.Walk(root, func(p string, i os.FileInfo, e error) error {
		if e != nil || i.IsDir() {
			return e
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".xlsx" || ext == ".xls" {
			out = append(out, p)
		}
		if ext == ".pdf" {
			pdf = append(pdf, p)
		}
		return nil
	})
	return out, pdf, err
}

func FindFileType(fundMap map[string][]string, funds []models.FegFileIdentification, series string) ([]models.MappedResp, []models.MappedResp) {
	var out []models.MappedResp
	var unmapped []models.MappedResp
	var mappedFilePath []string
	var allFiles []string
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
			if !slices.Contains(allFiles, file) {
				allFiles = append(allFiles, file)
			}

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
					mappedFilePath = append(mappedFilePath, file)
				}

			} else if matchByidentifier(strings.ToLower(resp.FileName), strings.ToLower(fund.FileType)) {
				fmt.Printf("\nfname:%vhi,fileType:%vhi,conv:%vhi\n", resp.FileName, resp.FileType, FileTypeConv[fund.FileType])
				resp.FileType = FileTypeConv[fund.FileType]
				out = append(out, resp)
				mappedFilePath = append(mappedFilePath, file)
			}
		}
	}
	for _, file := range allFiles {
		if !slices.Contains(mappedFilePath, file) {
			var unmap models.MappedResp
			unmap.FileName = filepath.Base(file)
			unmap.Path = strings.TrimPrefix(file, os.Getenv("STORAGE_PATH"))
			unmapped = append(unmapped, unmap)
		}
	}
	return out, unmapped
}

func matchByidentifier(filename string, filetype string) bool {
	flg := true
	if filetype == "na" {
		// fmt.Println("hi got into NA")
		return true
	}
	identifiers := strings.Split(filetype, "_")
	if filetype == "nav_tb" {
		for _, id := range strings.Split(FileTypeConv[filetype], " ") {
			if !strings.Contains(filename, strings.ToLower(id)) {
				flg = false
			}
		}
		if flg {
			return flg
		}
	}
	flg = true
	// Build a single regex:  token1 .* token2 .* … .* tokenN
	parts := make([]string, len(identifiers))
	for i, tok := range identifiers {
		parts[i] = regexp.QuoteMeta(strings.ToLower(tok))
	}
	pattern := strings.Join(parts, ".*")

	re, err := regexp.Compile("(?i)" + pattern) // (?i) = case-insensitive
	if err != nil {
		return false
	}

	return re.MatchString(strings.ToLower(filename))
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
				FileType:  FileTypeConv[f.FileType],
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
			file.Path = strings.TrimPrefix(p, os.Getenv("STORAGE_PATH"))
			unmapped = append(unmapped, file)
		}
	}
	return out, unmapped
}
