package logger

import (
	"bytes"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config defines the configuration options for the logger
type Config struct {
	// LogLevel sets the minimum enabled logging level. Valid levels are
	// "debug", "info", "warn", and "error".
	LogLevel string

	// LogFileSize is the maximum size in megabytes of the log file before it gets
	// rotated. It defaults to 10 megabytes.
	LogFileSize int

	// LogFileCount is the maximum number of old log files to retain.
	// The default is 5.
	LogFileCount int

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
}

// InitLogger initializes the global logger based on the provided Config.
// It sets the log level, output format, rotation options, etc.
func InitLogger(config Config) {
	if config.LogFileSize == 0 {
		config.LogFileSize = 10
	}
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
	var level = int(zerolog.InfoLevel)
	if strings.EqualFold(config.LogLevel, StrDebug) {
		level = int(zerolog.DebugLevel)
	}
	if strings.EqualFold(config.LogLevel, "warning") {
		level = int(zerolog.WarnLevel)
	}
	if config.TimeZone != "" {
		if strings.EqualFold(config.TimeZone, "local") {
			timeZone = *time.Local
		} else if strings.EqualFold(config.TimeZone, "utc") {
			timeZone = *time.UTC
		} else {
			timeZone2, err := time.LoadLocation(config.TimeZone)
			if err == nil {
				timeZone = *timeZone2
			}
		}
	}

	Log = zerolog.New(zerolog.Logger{}).
		Level(zerolog.Level(level)).
		With().
		Timestamp().Caller().
		Logger()
	filelogger := lumberjack.Logger{
		Filename:   "./logs/downloader.log",
		MaxSize:    config.LogFileSize, // megabytes
		MaxBackups: config.LogFileCount,
		MaxAge:     28,                 //days
		Compress:   config.LogCompress, // disabled by default
	}
	if config.LogToFileOnly {
		Log = Log.Output(&filelogger)
		return
	}
	if config.LogColorize {
		Log = Log.Output(zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: os.Stdout}, &filelogger))
		return
	}
	Log = Log.Output(zerolog.MultiLevelWriter(os.Stdout, &filelogger))
}

type LogField struct {
	Name  string
	Value any
}

// NewLogField creates a new LogField struct with the given name and value.
// LogField is used to represent a key-value pair that can be attached
// to a log entry.
func NewLogField(name string, value any) LogField {
	return LogField{Name: name, Value: value}
}

// NewLogFieldValue creates a new LogField with the given value.
func NewLogFieldValue(value any) LogField {
	return LogField{Value: value}
}

// LogDynamicSlice logs a message with dynamic fields and static fields.
// It allows specifying the log level, message, static fields, and dynamic fields.
// The static fields are logged on every call, while the dynamic fields can vary per call.
// For typev use one of the following: "info", "debug", "error", "fatal", "warn", "panic"
func LogDynamicSlice(typev string, msg string, staticfields []LogField, fields ...LogField) {
	var logv *zerolog.Event
	switch typev {
	case "info":
		logv = Log.Info()
	case "debug":
		logv = Log.Debug()
	case "error":
		logv = Log.Error()
	case "fatal":
		logv = Log.Fatal()
	case "warn":
		logv = Log.Warn()
	case "panic":
		logv = Log.Panic()
	default:
		logv = Log.Info()
	}
	logv.CallerSkipFrame(1)
	for idx := range fields {
		if fields[idx].Value == nil {
			continue
		}
		addlogvalue(logv, &fields[idx])
	}
	for idx := range staticfields {
		if staticfields[idx].Value == nil {
			continue
		}
		addlogvalue(logv, &staticfields[idx])
	}
	logv.Msg(msg)
}

// addlogvalue adds the provided LogField value to the given zerolog Event.
// It inspects the dynamic type of the LogField Value and calls the appropriate
// zerolog Event method like Str, Int, etc.
func addlogvalue(logv *zerolog.Event, field *LogField) {
	switch tt := field.Value.(type) {
	case string:
		if tt == "" {
			break
		}
		logv.Str(field.Name, tt)
	case *string:
		if *tt == "" {
			break
		}
		logv.Str(field.Name, *tt)
	case *int:
		if *tt == 0 {
			break
		}
		logv.Int(field.Name, *tt)
	case int:
		if tt == 0 {
			break
		}
		logv.Int(field.Name, tt)
	case *int64:
		if *tt == 0 {
			break
		}
		logv.Int64(field.Name, *tt)
	case int64:
		if tt == 0 {
			break
		}
		logv.Int64(field.Name, tt)
	case uint:
		if tt == 0 {
			break
		}
		logv.Uint(field.Name, tt)
	case *uint:
		if *tt == 0 {
			break
		}
		logv.Uint(field.Name, *tt)
	case bool:
		logv.Bool(field.Name, tt)
	case *bool:
		logv.Bool(field.Name, *tt)
	case float64:
		logv.Float64(field.Name, tt)
	case float32:
		logv.Float32(field.Name, tt)
	case *float64:
		logv.Float64(field.Name, *tt)
	case *float32:
		logv.Float32(field.Name, *tt)
	case time.Duration:
		logv.Str(field.Name, tt.Round(time.Second).String())
	case error:
		logv.Err(tt)
	case *bytes.Buffer:
		logv.Bytes(field.Name, tt.Bytes())
	default:
		logv.Any(field.Name, tt)
	}
}

// LogDynamic logs a message with dynamic log level and arbitrary fields.
// It takes the log level type as the first argument, followed by the log message,
// and finally any number of LogField structs containing the key-value pairs.
// It allows logging with different levels and custom fields in a flexible way.
// For typev use one of the following: "info", "debug", "error", "fatal", "warn", "panic"
func LogDynamic(typev string, msg string, fields ...LogField) {
	var logv *zerolog.Event
	switch typev {
	case "info":
		logv = Log.Info()
	case "debug":
		logv = Log.Debug()
	case "error":
		logv = Log.Error()
	case "fatal":
		logv = Log.Fatal()
	case "warn":
		logv = Log.Warn()
	case "panic":
		logv = Log.Panic()
	default:
		logv = Log.Info()
	}
	logv.CallerSkipFrame(1)
	for idx := range fields {
		if fields[idx].Value == nil {
			continue
		}
		addlogvalue(logv, &fields[idx])
	}
	logv.Msg(msg)
}
