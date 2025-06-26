package utility

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ANSI color codes for console output
const (
	ColorReset  = "\033[0m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorRed    = "\033[31m"
	ColorWhite  = "\033[37m"
	ColorBgRed  = "\033[41m"
)

type Logger struct {
	fileLogger    *log.Logger
	consoleLogger *log.Logger
}

func LoggerFunc() *Logger {
	// Set timezone to IST (UTC+5:30)
	loc := time.FixedZone("IST", 5*3600+30*60)

	// Create logs directory if it doesn't exist
	logDir := "./logs"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		os.Mkdir(logDir, 0755)
	}

	// Create log file with today's date
	today := time.Now().In(loc).Format("2006-01-02")
	logFile := filepath.Join(logDir, today+".log")

	// Ensure directory exists
	os.MkdirAll(filepath.Dir(logFile), 0755)

	// Open log file
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open log file:", err)
	}

	// File formatter - no flags since we'll format manually
	fileLogger := log.New(file, "", 0)

	// Console formatter
	consoleLogger := log.New(os.Stdout, "", 0)

	return &Logger{
		fileLogger:    fileLogger,
		consoleLogger: consoleLogger,
	}
}

func (l *Logger) getCallerInfo() (string, string, int) {
	// Skip 3 frames: runtime.Callers, getCallerInfo, Log, actual caller
	pc, file, line, ok := runtime.Caller(3)
	if !ok {
		return "unknown", "unknown", 0
	}

	// Get function name and extract package
	fn := runtime.FuncForPC(pc)
	var pkg string
	if fn != nil {
		fullName := fn.Name()
		parts := strings.Split(fullName, "/")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			pkgFunc := strings.Split(lastPart, ".")
			if len(pkgFunc) > 0 {
				pkg = pkgFunc[0]
			}
		}
	}

	// Get just the filename without path
	filename := filepath.Base(file)

	return pkg, filename, line
}

func (l *Logger) log(level string, message string) {
	// Get caller information
	pkg, filename, line := l.getCallerInfo()

	// Set timezone to IST
	loc := time.FixedZone("IST", 5*3600+30*60)
	timestamp := time.Now().In(loc)

	// File logging with format: 2025-06-25 05:24:13 UTC | INFO | pkg=utility | file=hash.go | line=63 | message
	fileTimestamp := timestamp.UTC().Format("2006-01-02 15:04:05 UTC")
	fileLogMsg := fmt.Sprintf("%s | %s | pkg=%s | file=%s | line=%d | %s",
		fileTimestamp, level, pkg, filename, line, message)
	l.fileLogger.Println(fileLogMsg)

	// Console logging with colors
	var color string
	switch level {
	case "DEBUG":
		color = ColorReset
	case "INFO":
		color = ColorGreen
	case "WARNING":
		color = ColorYellow
	case "ERROR":
		color = ColorRed
	case "CRITICAL":
		color = ColorWhite + ColorBgRed
	default:
		color = ColorReset
	}

	// Console timestamp in IST
	consoleTimestamp := timestamp.Format("02-01-2006 15:04:05 MST-0700")
	fmt.Printf("%s%s | %s | %s%s\n", color, level, consoleTimestamp, message, ColorReset)
}

func (l *Logger) Log(level, message string) {
	switch level {
	case "DEBUG":
		l.log("DEBUG", message)
	case "INFO":
		l.log("INFO", message)
	case "WARNING":
		l.log("WARNING", message)
	case "ERROR":
		l.log("ERROR", message)
	case "CRITICAL":
		l.log("CRITICAL", message)
	default:
		l.log("INFO", message) // default to INFO if unknown level
	}
}
