package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config defines the configuration options for the logger.
type Config struct {
	// LogLevel sets the minimum enabled logging level. Valid levels are
	// "debug", "info", "warn", and "error".
	LogLevel string

	// LogFileSize is the maximum size in megabytes of the log file before it gets
	// rotated. It defaults to 10 megabytes.
	LogFileSize int

	// LogFileCount is the maximum number of old log files to retain.
	// The default is 5.
	LogFileCount uint8

	// LogCompress determines if the rotated log files should be compressed
	// using gzip. The default is false.
	LogCompress bool

	// LogColorize enables output with colors
	LogColorize bool

	// TimeFormat sets the format for timestamp in logs. Valid formats are
	// "rfc3339", "iso8601", etc. The default is RFC3339.
	TimeFormat string

	// TimeZone sets the time zone to use for timestamps in logs.
	// The default is to use the local time zone.
	TimeZone string

	// LogToFileOnly disables logging to stdout.
	// If true, logs will only be written to the file and not also stdout.
	LogToFileOnly bool

	LogZeroValues bool
}

const (
	logfile      = "./logs/downloader.log"
	errorlogfile = "./logs/errors.log"
	weblogfile   = "./logs/weblogs.log"
)

// errorLevelWriter is a zerolog LevelWriter that only forwards events at
// ErrorLevel or above (Error, Fatal, Panic) to the wrapped writer. It lets the
// same log stream additionally be persisted to an errors-only file. Lower-level
// events are accepted and discarded so they still reach the other writers.
type errorLevelWriter struct {
	w io.Writer
}

func (ew errorLevelWriter) Write(p []byte) (int, error) {
	// Unleveled writes have no level context; drop them from the errors file.
	return len(p), nil
}

func (ew errorLevelWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if level >= zerolog.ErrorLevel {
		return ew.w.Write(p)
	}

	return len(p), nil
}

var (
	logZeroValues bool

	// webLog is a dedicated logger for HTTP request logging (the gin middleware).
	// It writes only to weblogs.log so web access lines stay out of the main
	// application and error logs. Initialized by InitLogger.
	webLog zerolog.Logger

	logMap = map[string]func() *zerolog.Event{
		"info":  log.Info,
		"debug": log.Debug,
		"error": log.Error,
		"fatal": log.Fatal,
		"warn":  log.Warn,
		"panic": log.Panic,
	}
)

// InitLogger initializes the global logger based on the provided Config.
// It sets the log level, output format, rotation options, etc.
func InitLogger(config Config) {
	initializePools()

	if config.LogFileSize == 0 {
		config.LogFileSize = 10
	}

	logZeroValues = config.LogZeroValues
	if config.LogFileCount == 0 {
		config.LogFileCount = 5
	}

	switch config.TimeFormat {
	case "rfc3339":
		timeFormat = time.RFC3339Nano
	case "iso8601":
		timeFormat = "2006-01-02T15:04:05.000Z0700"
	case "rfc1123":
		timeFormat = time.RFC1123
	case "rfc822":
		timeFormat = time.RFC822
	case "rfc850":
		timeFormat = time.RFC850
	case "":
		timeFormat = time.RFC3339Nano
	default:
		timeFormat = config.TimeFormat
	}

	var dbug bool

	level := zerolog.InfoLevel
	if strings.EqualFold(config.LogLevel, StrDebug) {
		level = zerolog.DebugLevel
		dbug = true
	}

	if strings.EqualFold(config.LogLevel, "warning") {
		level = zerolog.WarnLevel
	}

	if config.TimeZone != "" {
		switch {
		case strings.EqualFold(config.TimeZone, "local"):
			timeZone = *time.Local
		case strings.EqualFold(config.TimeZone, "utc"):
			timeZone = *time.UTC
		default:
			timeZone2, err := time.LoadLocation(config.TimeZone)
			if err == nil {
				timeZone = *timeZone2
			}
		}
	}

	var writers io.Writer

	if !config.LogToFileOnly {
		if config.LogColorize {
			writers = zerolog.ConsoleWriter{Out: os.Stdout}
		} else {
			writers = os.Stdout
		}
	} else {
		writers = io.Discard
	}

	// Gate verbosity via zerolog's atomic global level rather than baking the
	// level into the logger instance. This lets SetLevel change verbosity at
	// runtime (including enabling debug) without rebuilding the logger or racing
	// with concurrent logging.
	zerolog.SetGlobalLevel(level)

	// Dedicated errors-only file. Receives the same formatted lines as the main
	// log, but the level writer keeps only Error/Fatal/Panic entries.
	errorFile := errorLevelWriter{w: &lumberjack.Logger{
		Filename:   errorlogfile,
		MaxSize:    config.LogFileSize, // megabytes
		MaxBackups: int(config.LogFileCount),
		MaxAge:     28,                 // days
		Compress:   config.LogCompress, // disabled by default
	}}

	logctx := zerolog.New(zerolog.MultiLevelWriter(writers, &lumberjack.Logger{
		Filename:   logfile,
		MaxSize:    config.LogFileSize, // megabytes
		MaxBackups: int(config.LogFileCount),
		MaxAge:     28,                 // days
		Compress:   config.LogCompress, // disabled by default
	}, errorFile)).With().Timestamp()
	if dbug {
		log = logctx.Caller().Logger()
	} else {
		log = logctx.Logger()
	}

	// Web request logging goes to its own file only (not stdout, downloader.log,
	// or errors.log).
	webLog = zerolog.New(&lumberjack.Logger{
		Filename:   weblogfile,
		MaxSize:    config.LogFileSize, // megabytes
		MaxBackups: int(config.LogFileCount),
		MaxAge:     28,                 // days
		Compress:   config.LogCompress, // disabled by default
	}).With().Timestamp().Logger()
}

// SetLevel changes the active logging verbosity at runtime without a restart.
// Accepted values (case-insensitive): "debug", "info", "warn"/"warning",
// "error". Unknown values are ignored. Safe to call concurrently with logging
// because it uses zerolog's atomic global level.
func SetLevel(level string) {
	switch {
	case strings.EqualFold(level, StrDebug):
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case strings.EqualFold(level, "info"):
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case strings.EqualFold(level, "warn"), strings.EqualFold(level, "warning"):
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case strings.EqualFold(level, "error"):
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	}
}

// GetLevel returns the current global logging level as a lowercase string
// (e.g. "debug", "info", "warn", "error").
func GetLevel() string {
	return zerolog.GlobalLevel().String()
}

// Logtype returns a zerolog.Event with the specified log level. If the log level is not recognized,
// it defaults to log.Info(). The 'skip' parameter specifies the number of stack frames to skip when
// determining the caller location for the log event.
func Logtype(typev string, skip int) *zerolog.Event {
	logFunc, exists := logMap[typev]
	if exists {
		if skip == 0 {
			return logFunc()
		}

		return logFunc().CallerSkipFrame(skip)
	}

	if skip == 0 {
		return log.Info()
	}

	return log.Info().CallerSkipFrame(skip)
}
