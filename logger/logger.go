package logger

import (
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Log = logrus.New()

// var Cpuprofiler interface {
// 	Stop()
// }
// var Memprofiler interface {
// 	Stop()
// }

type LoggerConfig struct {
	LogLevel     string
	LogFileSize  int
	LogFileCount int
	LogCompress  bool
}

func InitLogger(config LoggerConfig) {
	src, _ := os.OpenFile(os.DevNull, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	Log.Out = src
	// logPath := "downloader.log"
	Log.SetFormatter(&logrus.TextFormatter{})
	if strings.EqualFold(config.LogLevel, "Debug") {
		// Memprofiler = profile.Start(profile.ProfilePath("."), profile.MemProfile, profile.MemProfileHeap)
		Log.SetLevel(logrus.DebugLevel)
	}
	if strings.EqualFold(config.LogLevel, "Warning") {
		Log.SetLevel(logrus.WarnLevel)
	}

	// logWriter, err := rotatelogs.New(
	// 	logPath,
	// 	rotatelogs.WithLinkName("downloader-current.log"), // Generate a soft link to point to the latest log file
	// 	rotatelogs.WithRotationSize(int64(config.LogFileSize)*1024*1024),
	// 	rotatelogs.WithRotationCount(uint(config.LogFileCount)),
	// 	rotatelogs.WithMaxAge(0),
	// 	rotatelogs.ForceNewFile(),
	// )

	// if err != nil {
	// 	log.Printf("failed to create rotatelogs: %s", err)
	// 	os.Exit(0)
	// } else {
	mw := io.MultiWriter(os.Stdout, &lumberjack.Logger{
		Filename:   "downloader.log",
		MaxSize:    config.LogFileSize, // megabytes
		MaxBackups: config.LogFileCount,
		MaxAge:     28,                 //days
		Compress:   config.LogCompress, // disabled by default
	})
	Log.SetOutput(mw)
	// }
}
