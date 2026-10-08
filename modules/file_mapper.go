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
	"time"

	"github.com/brandenc40/romannumeral"
	"github.com/google/uuid"
)

const marker = "_quantrium_"

var (
	dateRe      = regexp.MustCompile(`(\d{4})[.\-\/\\](0[1-9]|1[0-2])[.\-\/\\](0[1-9]|[12][0-9]|3[01])`)
	romanRe     = regexp.MustCompile(`\b[MDCLXVI]+\b`)
	fundRomanRe = regexp.MustCompile(`[MDCLXVI]+`)
	q1to5       = regexp.MustCompile(`^Q[1-5]$`)
	periodYMD   = regexp.MustCompile(`(19|20)(\d{2})(0[1-9]|1[0-2])([012]\d|3[01])`)
	periodMDY   = regexp.MustCompile(`\b(0?[1-9]|1[0-2])[.\-/](0?[1-9]|[12]\d|3[01])[.\-/](\d{2}|\d{4})\b`)
)

var periodConv = map[string][]string{
	"M": {
		"january",
		"february",
		"march",
		"april",
		"may",
		"june",
		"july",
		"august",
		"september",
		"october",
		"november",
		"december",
	},
	"Q": {
		"Q1", // January, February, March
		"Q2", // April, May, June
		"Q3", // July, August, September
		"Q4", // October, November, December
	},
}

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

var (
	MultiSeriesErr       = errors.New("Upload contains funds from multiple series. Please upload single series at a time")
	ShortCodeErr         = errors.New("invalid file name: no recognizable short-code found; please include a valid short-code")
	SharedFilesUploadErr = errors.New("Common files like investor liquidity/Unfunded commitments are insufficient to identify SPV.Please upload one fund")
	FilingNotFound       = errors.New("Filing not found for given filing Name")
	MappingNotFound      = errors.New("Mapping not found for given filing name")
)

const UnrecognizedFundReason = "The file name does not include a recognized fund short code or fund name, so it could not be matched to a fund."

type fileMatchMiss struct {
	FundName  string
	ShortCode string
	Kind      string
	Detail    string
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

	if req.Filing != nil && !*req.Filing {
		filingId, mappedFileSeries, err := db.GetFilingId(req.FilingName, cid)
		if err != nil {
			logger.Log("ERROR", fmt.Sprintf("Filing tracker error: %v", err.Error()))
			return nil, FilingNotFound
		}
		mapped, err := db.PullFilingMapper(*filingId, *mappedFileSeries)
		if err != nil {
			logger.Log("ERROR", fmt.Sprintf("Mapper error: %v", err.Error()))
			return nil, MappingNotFound
		}

		for _, row := range mapped {
			row.FileType = FileTypeConv[row.FileType]
		}

		return &models.FileIdentificationResponse{
			Message:       "File mappings retrived successfully",
			Series:        *mappedFileSeries,
			FilingName:    req.FilingName,
			FileMappings:  mapped,
			UnmappedFiles: nil,
			FundDetails:   toFundResp(funds, *mappedFileSeries),
		}, nil
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
	// Storage names are "{uuid}_quantrium_{original}". Re-uploads keep every copy,
	// so collapse to the newest file for each original name before mapping.
	excels = DedupeStoredFiles(excels)
	candidates, unmapped := ShortCodesFromFiles(excels, funds)
	// for key, files := range candidates {
	// 	for _, file := range files {
	// 		logger.Log("DEBUG", fmt.Sprintf("\n key:%v files:%vhi\n", key, file))
	// 	}
	// }
	if len(candidates) == 0 {
		if err := RemoveDir(baseDir); err != nil {
			logger.Log("ERROR", fmt.Sprintf("Failed to delete dir:%v", err.Error()))
			return nil, ShortCodeErr
		}
		return nil, ShortCodeErr
	}
	var scoped []models.FegFileIdentification
	for _, fund := range funds {
		if _, ok := candidates[strings.ToUpper(fund.ShortCode)]; ok {
			// fmt.Printf("scoped fundname:%vhi,sc:%vhi", fund.FundName, fund.ShortCode)
			scoped = append(scoped, fund)
		}
	}
	if len(scoped) == 0 {
		return nil, ShortCodeErr
	}
	series := decideSeries(scoped)
	if series == "" || series == "shared" {
		if err := RemoveDir(baseDir); err != nil {
			logger.Log("ERROR", fmt.Sprintf("Failed to delete dir:%v", err.Error()))
			return nil, MultiSeriesErr
		}
		if series == "shared" {
			return nil, SharedFilesUploadErr
		}
		return nil, MultiSeriesErr
	}
	// fmt.Println("shortcide identification")
	// for k, v := range candidates {
	// 	fmt.Printf("fund:%vhi,files:%v\n", k, v)
	// }

	mapped, failedFiles := FindFileType(candidates, funds, series)
	unmapped = append(unmapped, failedFiles...)
	multiversion := getFileVersion(mapped)
	var newfileVer []models.PendingVersion
	if multiversion {
		filingId, mappedFileSeries, err := db.GetFilingId(req.FilingName, cid)
		mappedRec, err := db.PullFilingMapper(*filingId, *mappedFileSeries)
		if err != nil {
			logger.Log("ERROR", fmt.Sprintf("Mapper error: %v", err.Error()))
			return nil, MappingNotFound
		}
		newfileVer = createPendingVersion(mapped, mappedRec)
	}

	msg := "File mappings updated successfully"
	if len(mapped) == 0 {
		msg = "Invalid file format for one or more files"
	} else if len(unmapped) > 0 {
		msg = "Partial success"
	}
	return &models.FileIdentificationResponse{
		Message:         msg,
		Series:          series,
		FilingName:      req.FilingName,
		FileMappings:    mapped,
		UnmappedFiles:   unmapped,
		NewfileVersions: newfileVer,
		FundDetails:     toFundResp(funds, series),
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
func createPendingVersion(mapped, storedMap []models.MappedResp) (pendingVersion []models.PendingVersion) {
	var oldVolFiles []string
	for _, fm := range storedMap {
		if !slices.Contains(oldVolFiles, filepath.Base(fm.Path)) {
			oldVolFiles = append(oldVolFiles, filepath.Base(fm.Path))
		}
	}
	for _, fm := range mapped {
		if !slices.Contains(oldVolFiles, filepath.Base(fm.Path)) {
			newfileVersion := models.PendingVersion{
				ShortCode:      fm.ShortCode,
				Filename:       fm.FileName,
				OldVersion:     fm.Version - 1,
				NewFileVolName: filepath.Base(fm.Path),
				NewVersion:     fm.Version,
				NewFilePath:    fm.Path,
			}
			pendingVersion = append(pendingVersion, newfileVersion)
		}
	}
	return pendingVersion
}

func getFileVersion(mapped []models.MappedResp) bool {
	var multiversion bool
	// Group by storing INDEXES instead of pointers
	groupedFiles := make(map[string][]int)

	for i := range mapped {
		key := fmt.Sprintf("%s-%s", mapped[i].ShortCode, mapped[i].FileType)
		groupedFiles[key] = append(groupedFiles[key], i)
	}

	for _, indexes := range groupedFiles {
		// Sort indexes by timestamp (oldest first)
		sort.Slice(indexes, func(i, j int) bool {
			return mapped[indexes[i]].Timestamp.Before(mapped[indexes[j]].Timestamp)
		})
		// Assign versions using indexes
		for version, idx := range indexes {
			mapped[idx].Version = version + 1
			// mapped[idx].GroupId = grpid
		}
		if len(indexes) > 1 {
			multiversion = true
		}
	}
	return multiversion
}

func getFileUploadTime(filepath string) time.Time {
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		logger.Log("ERROR", err.Error())
		return time.Time{}
	}
	return fileInfo.ModTime()
}

func FindFileType(fundMap map[string][]string, funds []models.FegFileIdentification, series string) ([]models.MappedResp, []models.MappedResp) {
	var out []models.MappedResp
	var unmapped []models.MappedResp
	var mappedFilePath []string
	var allFiles []string
	missesByFile := map[string][]fileMatchMiss{}
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
			fmt.Println(filepath.Base(file))
			if !slices.Contains(allFiles, file) {
				allFiles = append(allFiles, file)
			}

			var resp models.MappedResp
			resp.FundName = fund.FundName
			resp.ShortCode = fund.ShortCode
			resp.FileName = OriginalFileName(file)
			resp.VolFileName = filepath.Base(file)
			resp.Path = strings.TrimPrefix(file, os.Getenv("STORAGE_PATH"))
			resp.Timestamp = getFileUploadTime(file)

			if q1to5.MatchString(fund.FileType) {
				quarter, loc := matchQuarterByFilename(resp.FileName)
				resp.FileType = "Q" + strconv.Itoa(quarter)
				if loc != -1 && resp.FileType == fund.FileType {
					resp.GroupId = fmt.Sprintf("%s-%d", resp.ShortCode, quarter)
					out = append(out, resp)
					mappedFilePath = append(mappedFilePath, file)
				} else if loc == -1 {
					missesByFile[file] = append(missesByFile[file], fileMatchMiss{
						FundName:  fund.FundName,
						ShortCode: fund.ShortCode,
						Kind:      "no_quarter",
					})
				} else {
					missesByFile[file] = append(missesByFile[file], fileMatchMiss{
						FundName:  fund.FundName,
						ShortCode: fund.ShortCode,
						Kind:      "unexpected_quarter",
						Detail:    resp.FileType,
					})
				}

			} else if matchByidentifier(strings.ToLower(resp.FileName), strings.ToLower(fund.FileType)) {
				resp.FileType = FileTypeConv[fund.FileType]
				if fund.Period != nil {
					resp.Period = periodIdentifier(resp.FileName, *fund.Period)
				}
				resp.GroupId = fmt.Sprintf("%s-%s", resp.ShortCode, fund.FileType)
				out = append(out, resp)
				mappedFilePath = append(mappedFilePath, file)
			} else {
				missesByFile[file] = append(missesByFile[file], fileMatchMiss{
					FundName:  fund.FundName,
					ShortCode: fund.ShortCode,
					Kind:      "file_type",
					Detail:    displayFileType(fund.FileType),
				})
			}
		}
	}
	for _, file := range allFiles {
		if !slices.Contains(mappedFilePath, file) {
			var unmap models.MappedResp
			unmap.FileName = OriginalFileName(file)
			unmap.VolFileName = filepath.Base(file)
			unmap.Path = strings.TrimPrefix(file, os.Getenv("STORAGE_PATH"))
			if misses := missesByFile[file]; len(misses) > 0 {
				unmap.FundName = misses[0].FundName
				unmap.ShortCode = misses[0].ShortCode
				unmap.Reason = explainUnmappedFile(misses)
			} else {
				unmap.Reason = explainUnmappedFile(nil)
			}
			unmapped = append(unmapped, unmap)
		}
	}
	return out, unmapped
}

func displayFileType(fileType string) string {
	if label, ok := FileTypeConv[fileType]; ok && label != "" {
		return label
	}
	cleaned := strings.ReplaceAll(strings.TrimSpace(fileType), "_", " ")
	if cleaned == "" {
		return "the expected file type"
	}
	return cleaned
}

func explainUnmappedFile(misses []fileMatchMiss) string {
	if len(misses) == 0 {
		return "The file could not be matched to an expected file type for its fund."
	}

	label := fundMatchLabel(misses)
	noQuarter := false
	var quarters []string
	var types []string
	for _, miss := range misses {
		switch miss.Kind {
		case "no_quarter":
			noQuarter = true
		case "unexpected_quarter":
			if miss.Detail != "" && !slices.Contains(quarters, miss.Detail) {
				quarters = append(quarters, miss.Detail)
			}
		case "file_type":
			detail := miss.Detail
			if detail == "" {
				detail = "the expected file type"
			}
			if !slices.Contains(types, detail) {
				types = append(types, detail)
			}
		}
	}

	if len(quarters) == 1 && !noQuarter && len(types) == 0 {
		return fmt.Sprintf("The file name matches %s and looks like %s, but that quarter is not expected for this fund.", label, quarters[0])
	}
	if len(quarters) > 1 && !noQuarter && len(types) == 0 {
		return fmt.Sprintf("The file name matches %s and looks like %s, but those quarters are not expected for this fund.", label, joinAnd(quarters))
	}

	var problems []string
	if noQuarter {
		problems = append(problems, "it does not include a recognizable reporting quarter (use a year and month such as 2024-03)")
	}
	if len(quarters) == 1 {
		problems = append(problems, fmt.Sprintf("%s is not an expected quarter for this fund", quarters[0]))
	} else if len(quarters) > 1 {
		problems = append(problems, fmt.Sprintf("%s are not expected quarters for this fund", joinAnd(quarters)))
	}
	if len(types) > 0 {
		problems = append(problems, fmt.Sprintf("it does not match an expected file type (%s)", joinOr(types)))
	}
	if len(problems) == 0 {
		return fmt.Sprintf("The file name matches %s, but it could not be matched to an expected file type.", label)
	}
	return fmt.Sprintf("The file name matches %s, but %s.", label, joinAnd(problems))
}

func fundMatchLabel(misses []fileMatchMiss) string {
	var names []string
	code := ""
	for _, miss := range misses {
		if code == "" {
			code = miss.ShortCode
		}
		if miss.FundName != "" && !slices.Contains(names, miss.FundName) {
			names = append(names, miss.FundName)
		}
	}
	name := strings.Join(names, " / ")
	if name == "" {
		name = "the recognized fund"
	}
	if code == "" {
		return name
	}
	return fmt.Sprintf("%s (%s)", name, code)
}

func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

func joinOr(items []string) string {
	switch len(items) {
	case 0:
		return "the expected file type"
	case 1:
		return items[0]
	case 2:
		return items[0] + " or " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", or " + items[len(items)-1]
	}
}

func generateUUIDName(filename string) string {
	return uuid.New().String() + marker + filename
}

// OriginalFileName strips the storage prefix "{uuid}_quantrium_" from a path or file name.
func OriginalFileName(path string) string {
	base := filepath.Base(path)
	if i := strings.Index(base, marker); i >= 0 {
		return base[i+len(marker):]
	}
	return base
}

// DedupeStoredFiles keeps the newest stored copy of each original filename.
func DedupeStoredFiles(paths []string) []string {
	type chosen struct {
		path string
		when time.Time
	}
	best := make(map[string]chosen, len(paths))
	order := make([]string, 0, len(paths))
	for _, p := range paths {
		key := strings.ToLower(OriginalFileName(p))
		when := getFileUploadTime(p)
		prev, ok := best[key]
		if !ok {
			order = append(order, key)
			best[key] = chosen{path: p, when: when}
			continue
		}
		if !when.Before(prev.when) {
			best[key] = chosen{path: p, when: when}
		}
	}
	out := make([]string, 0, len(order))
	for _, key := range order {
		out = append(out, best[key].path)
	}
	return out
}

func getOrgName(filename string) string {
	parts := strings.SplitN(filename, marker, 2)
	if len(parts) != 2 {
		logger.Log("ERROR", fmt.Sprintf("filename %q does not match expected storage format", filename))
		return ""
	}
	return parts[1]
}

func matchByidentifier(filename string, filetype string) bool {
	flg := true
	if filetype == "na" {
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
		return "shared" // no anchor series
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

func periodIdentifier(filename, period string) string {
	var month int
	filename = strings.ToLower(filename)
	if m := periodYMD.FindStringSubmatch(filename); m != nil {
		month, _ = strconv.Atoi(m[3])
	}
	if m := periodMDY.FindStringSubmatch(filename); m != nil {
		month, _ = strconv.Atoi(m[1])
	}
	if period == "Q" {
		return "Q" + strconv.Itoa(month/3)
	}
	return strings.ToLower(time.Month(month).String())
}

func toFundResp(funds []models.FegFileIdentification, series string) []models.MappedResp {
	var out []models.MappedResp
	for _, f := range funds {
		if strings.EqualFold(strings.TrimSpace(f.Series), strings.TrimSpace(series)) || f.Series == "ALL" {
			if f.Period != nil {
				for _, p := range periodConv[*f.Period] {
					out = append(out, models.MappedResp{
						FundName:  f.FundName,
						FileType:  FileTypeConv[f.FileType],
						ShortCode: f.ShortCode,
						Period:    p,
						GroupId:   fmt.Sprintf("%s-%s", f.ShortCode, FileTypeConv[f.FileType]),
					})
				}
			} else {
				out = append(out, models.MappedResp{
					FundName:  f.FundName,
					FileType:  FileTypeConv[f.FileType],
					ShortCode: f.ShortCode,
					GroupId:   fmt.Sprintf("%s-%s", f.ShortCode, FileTypeConv[f.FileType]),
				})
			}
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

func ShortCodesFromFiles(paths []string, funds []models.FegFileIdentification) (map[string][]string, []models.MappedResp) {
	out := make(map[string][]string)

	unmapped := []models.MappedResp{}
	// longest first
	sort.Slice(funds, func(i, j int) bool {
		return len(funds[i].ShortCode) > len(funds[j].ShortCode)
	})
	for _, p := range paths {
		flg := false
		file := models.MappedResp{}
		orgName := OriginalFileName(p)
		ogBase := strings.ToLower(orgName)
		base := strings.ToLower(normalizeRomans(orgName))
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
			file.FileName = orgName
			file.VolFileName = filepath.Base(p)
			file.Path = strings.TrimPrefix(p, os.Getenv("STORAGE_PATH"))
			file.Reason = UnrecognizedFundReason
			unmapped = append(unmapped, file)
		}
	}
	return out, unmapped
}
