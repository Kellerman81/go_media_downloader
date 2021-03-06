package logger

import (
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Log = logrus.New()

type LoggerConfig struct {
	LogLevel     string
	LogFileSize  int
	LogFileCount int
	LogCompress  bool
}

func InitLogger(config LoggerConfig) {

	src, _ := os.OpenFile(os.DevNull, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	Log.Out = src
	Log.SetFormatter(&logrus.TextFormatter{})
	if strings.EqualFold(config.LogLevel, "Debug") {
		Log.SetLevel(logrus.DebugLevel)
		Log.SetReportCaller(true)
	}
	if strings.EqualFold(config.LogLevel, "Warning") {
		Log.SetLevel(logrus.WarnLevel)
	}
	if config.LogFileSize == 0 {
		config.LogFileSize = 10
	}
	if config.LogFileCount == 0 {
		config.LogFileCount = 5
	}

	mw := io.MultiWriter(os.Stdout, &lumberjack.Logger{
		Filename:   "./logs/downloader.log",
		MaxSize:    config.LogFileSize, // megabytes
		MaxBackups: config.LogFileCount,
		MaxAge:     28,                 //days
		Compress:   config.LogCompress, // disabled by default
	})
	Log.SetOutput(mw)
}
