package video

import (
	"log"
	"os"
	"sync"
)

var (
	logger     *log.Logger
	loggerOnce sync.Once
	debugMode  bool
)

func init() {
	debugMode = os.Getenv("LAZYCUT_DEBUG") == "1"
}

func getLogger() *log.Logger {
	loggerOnce.Do(func() {
		if debugMode {
			logger = log.New(os.Stderr, "[video] ", log.LstdFlags|log.Lshortfile)
		} else {
			logger = log.New(os.Stderr, "[video] ", log.LstdFlags)
		}
	})
	return logger
}

func LogError(format string, args ...interface{}) {
	getLogger().Printf("ERROR: "+format, args...)
}

func LogWarn(format string, args ...interface{}) {
	if debugMode {
		getLogger().Printf("WARN: "+format, args...)
	}
}

func LogDebug(format string, args ...interface{}) {
	if debugMode {
		getLogger().Printf("DEBUG: "+format, args...)
	}
}

func LogInfo(format string, args ...interface{}) {
	if debugMode {
		getLogger().Printf("INFO: "+format, args...)
	}
}

// SetDebugMode enables or disables debug logging
func SetDebugMode(enabled bool) {
	debugMode = enabled
}
