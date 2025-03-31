package utilities

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	infoLog  *log.Logger
	warnLog  *log.Logger
	errorLog *log.Logger
	logMutex = &sync.Mutex{}

	infoFile  *os.File
	warnFile  *os.File
	errorFile *os.File
)

func SetupLogging(logDir string) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	infoFile = openLogFile(filepath.Join(logDir, "info.log"))
	warnFile = openLogFile(filepath.Join(logDir, "warn.log"))
	errorFile = openLogFile(filepath.Join(logDir, "error.log"))

	infoWriter := io.MultiWriter(os.Stdout, infoFile)
	warnWriter := io.MultiWriter(os.Stdout, warnFile)
	errorWriter := io.MultiWriter(os.Stderr, errorFile)

	infoLog = log.New(infoWriter, "INFO: ", log.Ldate|log.Ltime)
	warnLog = log.New(warnWriter, "WARNING: ", log.Ldate|log.Ltime)
	errorLog = log.New(errorWriter, "ERROR: ", log.Ldate|log.Ltime)

	//Override Go's  default log
	log.SetOutput(infoWriter)
}

func openLogFile(path string) *os.File {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	return file
}

func getCallerInfo() string {
	pc, _, _, ok := runtime.Caller(3)
	if !ok {
		return "unknown"
	}
	return runtime.FuncForPC(pc).Name()
}

func Log(level string, format string, v ...interface{}) {
	logMutex.Lock()
	defer logMutex.Unlock()

	message := fmt.Sprintf(format, v...)
	logEntry := fmt.Sprintf("[%s]: %s", getCallerInfo(), message)

	switch level {
	case "INFO":
		infoLog.Println(logEntry)
	case "WARNING":
		warnLog.Println(logEntry)
	case "ERROR":
		errorLog.Println(logEntry)
	default:
		infoLog.Println(logEntry)
	}
}

// FlushLogs ensures that all log files flush their buffered data to disk.
func FlushLogs() {
	if infoFile != nil {
		if err := infoFile.Sync(); err != nil {
			log.Printf("failed to sync infoFile: %v", err)
		}
	}
	if warnFile != nil {
		if err := warnFile.Sync(); err != nil {
			log.Printf("failed to sync warnFile: %v", err)
		}
	}
	if errorFile != nil {
		if err := errorFile.Sync(); err != nil {
			log.Printf("failed to sync errorFile: %v", err)
		}
	}
}

func Info(format string, v ...interface{}) {
	Log("INFO", format, v...)
}
func Warn(format string, v ...interface{}) {
	Log("WARNING", format, v...)
}
func Error(format string, v ...interface{}) {
	Log("ERROR", format, v...)
}
