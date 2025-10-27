package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	infoLog   *log.Logger
	warnLog   *log.Logger
	errorLog  *log.Logger
	debugLog  *log.Logger
	logMutex  = &sync.Mutex{}
	debugMode = false
)

type LoggingOptions struct {
	LogDir struct {
		Path     string
		Relative bool
	}
	EnableDebug  bool
	MaxSizeMB    int
	MaxBackups   int
	MaxAgeDays   int
	CompressLogs bool
}

func SetupLogging(cfg LoggingOptions) {
	logDir := cfg.LogDir.Path

	if cfg.LogDir.Relative {
		// Ensure path is relative to working directory
		logDir = strings.TrimPrefix(logDir, string(os.PathSeparator))
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get working directory: %v", err)
		}
		logDir = filepath.Join(cwd, logDir)
	} else if !filepath.IsAbs(logDir) {
		// If not explicitly relative but not absolute, make it absolute
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get working directory: %v", err)
		}
		logDir = filepath.Join(cwd, logDir)
	}

	debugMode = cfg.EnableDebug

	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	newRotateWriter := func(filename string) io.Writer {
		return &lumberjack.Logger{
			Filename:   filepath.Join(logDir, filename),
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   cfg.CompressLogs,
		}
	}

	infoWriter := io.MultiWriter(os.Stdout, newRotateWriter("info.log"))
	warnWriter := io.MultiWriter(os.Stdout, newRotateWriter("warn.log"))
	errorWriter := io.MultiWriter(os.Stderr, newRotateWriter("error.log"))

	infoLog = log.New(infoWriter, "INFO: ", log.Ldate|log.Ltime)
	warnLog = log.New(warnWriter, "WARNING: ", log.Ldate|log.Ltime)
	errorLog = log.New(errorWriter, "ERROR: ", log.Ldate|log.Ltime)

	if cfg.EnableDebug {
		debugWriter := io.MultiWriter(os.Stdout, newRotateWriter("debug.log"))
		debugLog = log.New(debugWriter, "DEBUG: ", log.Ldate|log.Ltime)
	}

	log.SetOutput(infoWriter)
}

func getFuncName(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}
	return filepath.Base(fn.Name())
}

func getFileLine(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown:0"
	}
	return filepath.Base(file) + ":" + fmt.Sprint(line)
}

func Log(level string, format string, v ...interface{}) {
	logMutex.Lock()
	defer logMutex.Unlock()

	message := fmt.Sprintf(format, v...)
	caller := getFuncName(3)

	if level == "DEBUG" && debugMode {
		caller = caller + " " + getFileLine(3)
	}

	logEntry := "[" + caller + "] " + message

	switch level {
	case "INFO":
		infoLog.Println(logEntry)
	case "WARNING":
		warnLog.Println(logEntry)
	case "ERROR":
		errorLog.Println(logEntry)
	case "DEBUG":
		if debugLog != nil && debugMode {
			debugLog.Println(logEntry)
		}
	default:
		infoLog.Println(logEntry)
	}
}

func Info(format string, v ...interface{})  { Log("INFO", format, v...) }
func Warn(format string, v ...interface{})  { Log("WARNING", format, v...) }
func Error(format string, v ...interface{}) { Log("ERROR", format, v...) }
func Debug(format string, v ...interface{}) {
	if debugMode {
		Log("DEBUG", format, v...)
	}
}
