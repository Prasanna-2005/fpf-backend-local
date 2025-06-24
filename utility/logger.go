package utility

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// ANSI color codes for console output
const (
	ColorReset  = "\033[0m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorRed    = "\033[31m"
	ColorWhite  = "\033[97m"
	ColorBgRed  = "\033[41m"
)

// Logger wraps the standard logger with enhanced functionality
type Logger struct {
	fileLogger    *log.Logger
	consoleLogger *log.Logger
}

// NewLogger creates a new logger instance with file and console output
func NewLogger() (*Logger, error) {
	// Create logs directory if it doesn't exist
	logDir := "./logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file with current date
	logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", time.Now().Format("2006-01-02")))
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Setup timezone (UTC)
	location := time.UTC

	// Create file logger
	fileLogger := log.New(file, "", 0)
	fileLogger.SetOutput(&timestampWriter{
		writer:   file,
		location: location,
		isFile:   true,
	})

	// Create console logger
	consoleLogger := log.New(os.Stdout, "", 0)
	consoleLogger.SetOutput(&timestampWriter{
		writer:   os.Stdout,
		location: location,
		isFile:   false,
	})

	return &Logger{
		fileLogger:    fileLogger,
		consoleLogger: consoleLogger,
	}, nil
}

// timestampWriter adds timestamp and formatting to log output
type timestampWriter struct {
	writer   io.Writer
	location *time.Location
	isFile   bool
}

func (tw *timestampWriter) Write(p []byte) (n int, err error) {
	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05 UTC")

	if tw.isFile {
		formatted := fmt.Sprintf("%s | %s", timestamp, string(p))
		return tw.writer.Write([]byte(formatted))
	}

	// Console output
	formatted := fmt.Sprintf("%s | %s", timestamp, string(p))
	return tw.writer.Write([]byte(formatted))
}

// getCallerInfo returns package, file, and line information
func getCallerInfo() string {
	// Skip 3 frames: getCallerInfo -> log method -> actual caller
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		return "unknown:unknown:0"
	}

	// Extract package and file name
	dir := filepath.Dir(file)
	packageName := filepath.Base(dir)
	fileName := filepath.Base(file)

	return fmt.Sprintf("pkg=%s | file=%s | line=%d", packageName, fileName, line)
}

// Debug logs debug messages
func (l *Logger) Debug(msg string) {
	caller := getCallerInfo()
	l.fileLogger.Printf("DEBUG | %s | %s\n", caller, msg)
	l.consoleLogger.Printf("%sDEBUG%s | %s | %s\n", ColorReset, ColorReset, caller, msg)
}

// Info logs info messages
func (l *Logger) Info(msg string) {
	caller := getCallerInfo()
	l.fileLogger.Printf("INFO | %s | %s\n", caller, msg)
	l.consoleLogger.Printf("%sINFO%s | %s | %s\n", ColorGreen, ColorReset, caller, msg)
}

// Warning logs warning messages
func (l *Logger) Warning(msg string) {
	caller := getCallerInfo()
	l.fileLogger.Printf("WARNING | %s | %s\n", caller, msg)
	l.consoleLogger.Printf("%sWARNING%s | %s | %s\n", ColorYellow, ColorReset, caller, msg)
}

// Error logs error messages
func (l *Logger) Error(msg string) {
	caller := getCallerInfo()
	l.fileLogger.Printf("ERROR | %s | %s\n", caller, msg)
	l.consoleLogger.Printf("%sERROR%s | %s | %s\n", ColorRed, ColorReset, caller, msg)
}

// Critical logs critical messages
func (l *Logger) Critical(msg string) {
	caller := getCallerInfo()
	l.fileLogger.Printf("CRITICAL | %s | %s\n", caller, msg)
	l.consoleLogger.Printf("%s%sCRITICAL%s | %s | %s\n", ColorWhite, ColorBgRed, ColorReset, caller, msg)
}

// Debugf logs formatted debug messages
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}

// Infof logs formatted info messages
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

// Warningf logs formatted warning messages
func (l *Logger) Warningf(format string, args ...interface{}) {
	l.Warning(fmt.Sprintf(format, args...))
}

// Errorf logs formatted error messages
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

// Criticalf logs formatted critical messages
func (l *Logger) Criticalf(format string, args ...interface{}) {
	l.Critical(fmt.Sprintf(format, args...))
}

// Global logger instance for convenience
var globalLogger *Logger

// Init initializes the global logger
func Init() error {
	var err error
	globalLogger, err = NewLogger()
	return err
}

// Convenience functions using global logger
func Debug(msg string) {
	if globalLogger != nil {
		globalLogger.Debug(msg)
	}
}

func Info(msg string) {
	if globalLogger != nil {
		globalLogger.Info(msg)
	}
}

func Warning(msg string) {
	if globalLogger != nil {
		globalLogger.Warning(msg)
	}
}

func Error(msg string) {
	if globalLogger != nil {
		globalLogger.Error(msg)
	}
}

func Critical(msg string) {
	if globalLogger != nil {
		globalLogger.Critical(msg)
	}
}

func Debugf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debugf(format, args...)
	}
}

func Infof(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Infof(format, args...)
	}
}

func Warningf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warningf(format, args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Errorf(format, args...)
	}
}

func Criticalf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Criticalf(format, args...)
	}
}
