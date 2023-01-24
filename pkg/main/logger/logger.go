package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	LogLevel     string
	LogFileSize  int
	LogFileCount int
	LogCompress  bool
	TimeFormat   string
	TimeZone     string
}
type zoneClock time.Time
type zapLogger struct {
	GlobalLogger *zap.Logger
}

type fnlog func(msg string, fields ...zapcore.Field)

var Log zapLogger
var TimeZone = time.UTC
var TimeFormat = time.RFC3339Nano

func MyTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	encodeTimeLayout(t, TimeFormat, enc)
}

func encodeTimeLayout(t time.Time, layout string, enc zapcore.PrimitiveArrayEncoder) {
	type appendTimeEncoder interface {
		AppendTimeLayout(time.Time, string)
	}

	if enc, ok := enc.(appendTimeEncoder); ok {
		enc.AppendTimeLayout(t, layout)
		return
	}

	enc.AppendString(t.Format(layout))
}

func (c zoneClock) Now() time.Time {
	return time.Now().In(TimeZone)
}
func (c zoneClock) NewTicker(d time.Duration) *time.Ticker {
	return &time.Ticker{}
}
func InitLogger(config Config) {
	if config.LogFileSize == 0 {
		config.LogFileSize = 10
	}
	if config.LogFileCount == 0 {
		config.LogFileCount = 5
	}
	switch config.TimeFormat {
	case "rfc3339":
		TimeFormat = time.RFC3339Nano
	case "iso8601":
		TimeFormat = "2006-01-02T15:04:05.000Z0700"
	case "rfc1123":
		TimeFormat = time.RFC1123
	case "rfc822":
		TimeFormat = time.RFC822
	case "rfc850":
		TimeFormat = time.RFC850
	case "":
		TimeFormat = time.RFC3339Nano
	default:
		TimeFormat = config.TimeFormat
	}
	var level zapcore.Level = zap.InfoLevel
	if strings.EqualFold(config.LogLevel, "debug") {
		level = zap.DebugLevel
	}
	if strings.EqualFold(config.LogLevel, "warning") {
		level = zap.WarnLevel
	}
	if config.TimeZone != "" {
		if strings.EqualFold(config.TimeZone, "local") {
			TimeZone = time.Local
		} else if strings.EqualFold(config.TimeZone, "utc") {
			TimeZone = time.UTC
		} else {
			TimeZone, _ = time.LoadLocation(config.TimeZone)
		}
	}

	core := zapcore.NewCore(
		// use NewConsoleEncoder for human readable output
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     MyTimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}),
		// write to stdout as well as log files
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(&lumberjack.Logger{
			Filename:   "./logs/downloader.log",
			MaxSize:    config.LogFileSize, // megabytes
			MaxBackups: config.LogFileCount,
			MaxAge:     28,                 //days
			Compress:   config.LogCompress, // disabled by default
		})),
		zap.NewAtomicLevelAt(level),
	)
	var globalLogger *zap.Logger
	if strings.EqualFold(config.LogLevel, "debug") {
		globalLogger = zap.New(core, zap.WithClock(zoneClock{}), zap.AddCaller(), zap.AddStacktrace(zapcore.DebugLevel), zap.Development())
	} else {
		globalLogger = zap.New(core, zap.WithClock(zoneClock{}))
	}
	zap.ReplaceGlobals(globalLogger)
	Log.GlobalLogger = globalLogger
}

func printlog(fun fnlog, args ...interface{}) {
	fun(fmt.Sprint(args...))
}
func (l *zapLogger) Println(args ...interface{}) {
	printlog(l.GlobalLogger.Info, args...)
}
func (l *zapLogger) Info(args ...interface{}) {
	printlog(l.GlobalLogger.Info, args...)
}
func (l *zapLogger) Infoln(args ...interface{}) {
	printlog(l.GlobalLogger.Info, args...)
}
func (l *zapLogger) Error(args ...interface{}) {
	printlog(l.GlobalLogger.Error, args...)
}
func (l *zapLogger) Errorln(args ...interface{}) {
	printlog(l.GlobalLogger.Error, args...)
}
func (l *zapLogger) Warn(args ...interface{}) {
	printlog(l.GlobalLogger.Warn, args...)
}
func (l *zapLogger) Warning(args ...interface{}) {
	printlog(l.GlobalLogger.Warn, args...)
}
func (l *zapLogger) Warningln(args ...interface{}) {
	printlog(l.GlobalLogger.Warn, args...)
}
func (l *zapLogger) Debug(args ...interface{}) {
	printlog(l.GlobalLogger.Debug, args...)
}
func (l *zapLogger) Fatal(args ...interface{}) {
	printlog(l.GlobalLogger.Fatal, args...)
}
