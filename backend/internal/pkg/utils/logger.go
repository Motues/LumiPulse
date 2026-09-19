package utils

import (
	"fmt"
	"log"
	"os"
	"time"
)

const (
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
	LevelDebug = "debug"
	LevelFatal = "fatal"
)

var logger = log.New(os.Stdout, "", 0)

// Log 输出格式化日志：2026/05/30 22:18:05 [level] message
func Log(level, format string, args ...interface{}) {
	now := time.Now().Format("2006/01/02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	logger.Printf("%s [%s] %s", now, level, msg)
}

func Info(format string, args ...interface{}) {
	Log(LevelInfo, format, args...)
}

func Warn(format string, args ...interface{}) {
	Log(LevelWarn, format, args...)
}

func Error(format string, args ...interface{}) {
	Log(LevelError, format, args...)
}

func Debug(format string, args ...interface{}) {
	Log(LevelDebug, format, args...)
}

// Fatal 输出日志并退出
func Fatal(format string, args ...interface{}) {
	Log(LevelFatal, format, args...)
	os.Exit(1)
}
