package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	// File formatter
	fileFlags := log.Ldate | log.Ltime | log.Lshortfile
	fileLogger := log.New(file, "", fileFlags)

	// Console formatter
	consoleLogger := log.New(os.Stdout, "", log.LstdFlags)

	return &Logger{
		fileLogger:    fileLogger,
		consoleLogger: consoleLogger,
	}
}

func (l *Logger) log(level string, message string) {
	// File logging with package, file, line number
	l.fileLogger.Printf("%s | %s", level, message)

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

	// Set timezone to IST for console
	loc := time.FixedZone("IST", 5*3600+30*60)
	timestamp := time.Now().In(loc).Format("02-01-2006 15:04:05 MST-0700")
	fmt.Printf("%s%s |%s | %s%s\n", color, level, timestamp, message, ColorReset)
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
