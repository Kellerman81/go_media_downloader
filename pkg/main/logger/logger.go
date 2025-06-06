package logger

import (
	"bytes"
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

const logfile = "./logs/downloader.log"

var logZeroValues bool

// InitLogger initializes the global logger based on the provided Config.
// It sets the log level, output format, rotation options, etc.
func InitLogger(config Config) {
	PlAddBuffer.Init(5, func(b *AddBuffer) {
		b.Grow(800)
	}, func(b *AddBuffer) bool {
		b.Reset()
		return b.Cap() > 1000
	})
	PlBuffer.Init(5, func(b *bytes.Buffer) {
		b.Grow(800)
	}, func(b *bytes.Buffer) bool {
		b.Reset()
		return b.Cap() > 1000
	})
	PLArrAny.Init(5, func(a *Arrany) { a.Arr = make([]any, 0, 20) }, func(a *Arrany) bool {
		clear(a.Arr)
		if cap(a.Arr) > 200 {
			return true
		}
		a.Arr = a.Arr[:0]
		return false
	})
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
	logctx := zerolog.New(zerolog.MultiLevelWriter(writers, &lumberjack.Logger{
		Filename:   logfile,
		MaxSize:    config.LogFileSize, // megabytes
		MaxBackups: int(config.LogFileCount),
		MaxAge:     28,                 // days
		Compress:   config.LogCompress, // disabled by default
	})).Level(level).With().Timestamp()
	if dbug {
		log = logctx.Caller().Logger()
	} else {
		log = logctx.Logger()
	}
}

// LogDynamicany0 logs a message with no dynamic fields.
// The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic).
// The 'msg' parameter is the log message.
func LogDynamicany0(typev, msg string) {
	Logtype(typev, 1).Msg(msg)
}

// LogDynamicany1UInt logs a message with a dynamic uint field.
// The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic).
// The 'msg' parameter is the log message.
// The 'field1' parameter is the field name for the dynamic uint field.
// The 'value1' parameter is the value for the dynamic uint field.
func LogDynamicany1UInt(typev, msg, field1 string, value1 uint) {
	Logtype(typev, 1).Uint(field1, value1).Msg(msg)
}

// LogDynamicany1String logs a message with a dynamic string field.
// The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic).
// The 'msg' parameter is the log message.
// The 'field1' parameter is the field name for the dynamic string field.
// The 'value1' parameter is the value for the dynamic string field.
func LogDynamicany1String(typev, msg, field1, value1 string) {
	Logtype(typev, 1).Str(field1, value1).Msg(msg)
}

// LogDynamicany1Int logs a message with a dynamic int field.
// The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic).
// The 'msg' parameter is the log message.
// The 'field1' parameter is the field name for the dynamic int field.
// The 'value1' parameter is the value for the dynamic int field.
func LogDynamicany1Int(typev, msg, field1 string, value1 int) {
	Logtype(typev, 1).Int(field1, value1).Msg(msg)
}

// LogDynamicany1IntErr logs a message with a dynamic int field and an error.
// The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic).
// The 'msg' parameter is the log message.
// The 'err' parameter is the error to be logged.
// The 'field1' parameter is the field name for the dynamic int field.
// The 'value1' parameter is the value for the dynamic int field.
func LogDynamicany1IntErr(typev, msg string, err error, field1 string, value1 int) {
	Logtype(typev, 1).Int(field1, value1).Err(err).Msg(msg)
}

// LogDynamicany1UIntErr logs a message with a dynamic uint field and an error.
// The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic).
// The 'msg' parameter is the log message.
// The 'err' parameter is the error to be logged.
// The 'field1' parameter is the field name for the dynamic uint field.
// The 'value1' parameter is the value for the dynamic uint field.
func LogDynamicany1UIntErr(typev, msg string, err error, field1 string, value1 uint) {
	Logtype(typev, 1).Uint(field1, value1).Err(err).Msg(msg)
}

// LogDynamicany1StringErr logs a message with a dynamic string field and an error.
// The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic).
// The 'msg' parameter is the log message.
// The 'err' parameter is the error to be logged.
// The 'field1' parameter is the field name for the dynamic string field.
// The 'value1' parameter is the value for the dynamic string field.
func LogDynamicany1StringErr(typev, msg string, err error, field1, value1 string) {
	Logtype(typev, 1).Str(field1, value1).Err(err).Msg(msg)
}

// LogDynamicanyErr logs a message with an error.
// The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic).
// The 'msg' parameter is the log message.
// The 'err' parameter is the error to be logged.
func LogDynamicanyErr(typev, msg string, err error) {
	Logtype(typev, 1).Err(err).Msg(msg)
}

// LogDynamicany2StrAny logs a message with two dynamic fields: a string and any type.
// The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic).
// The 'msg' parameter is the log message.
// The 'field1' and 'field2' parameters are the field names for the dynamic fields.
// The 'value1' parameter is the value for the string field.
// The 'value2' parameter is the value for the any type field.
func LogDynamicany2StrAny(typev, msg, field1, value1, field2 string, value2 any) {
	logv := Logtype(typev, 1).Str(field1, value1)
	logvalue(logv, field2, value2)
	logv.Msg(msg)
}

// LogDynamicany2Str logs a message with two dynamic string fields.
// The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic).
// The 'msg' parameter is the log message.
// The 'field1' and 'field2' parameters are the field names for the dynamic string fields.
// The 'value1' and 'value2' parameters are the values for the dynamic string fields.
func LogDynamicany2Str(typev, msg, field1, value1, field2, value2 string) {
	Logtype(typev, 1).Str(field1, value1).Str(field2, value2).Msg(msg)
}

// LogDynamicany2Int logs a message with two dynamic integer fields.
// The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic).
// The 'msg' parameter is the log message.
// The 'field1' and 'field2' parameters are the field names for the dynamic integer fields.
// The 'value1' and 'value2' parameters are the values for the dynamic integer fields.
func LogDynamicany2Int(typev, msg, field1 string, value1 int, field2 string, value2 int) {
	Logtype(typev, 1).Int(field1, value1).Int(field2, value2).Msg(msg)
}

// LogDynamicany3StrIntInt logs a message with three dynamic fields: a string, an integer, and another integer.
// The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic).
// The 'msg' parameter is the log message.
// The 'field1', 'field2', and 'field3' parameters are the field names for the dynamic fields.
// The 'value1', 'value2', and 'value3' parameters are the values for the dynamic fields.
func LogDynamicany3StrIntInt(
	typev, msg, field1 string,
	value1 string,
	field2 string,
	value2 int,
	field3 string,
	value3 int,
) {
	Logtype(typev, 1).Str(field1, value1).Int(field2, value2).Int(field3, value3).Msg(msg)
}

// LogDynamicany logs a message with dynamic fields. The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic). The 'msg' parameter is the log message. The 'fields' parameter is a variadic list of key-value pairs to be logged.
func LogDynamicany(typev, msg string, fields ...any) {
	logv := Logtype(typev, 1)

	var n string
	for idx := range fields {
		switch tt := fields[idx].(type) {
		case string:
			if n == "" {
				n = tt
				continue
			}
		case *string:
			if n == "" {
				n = *tt
				continue
			}
		case error:
			logv.Err(tt)
			n = ""
			continue
		}
		if n == "" {
			continue
		}
		logvalue(logv, n, fields[idx])
		n = ""
	}
	logv.Msg(msg)
}

var logMap = map[string]func() *zerolog.Event{
	"info":  log.Info,
	"debug": log.Debug,
	"error": log.Error,
	"fatal": log.Fatal,
	"warn":  log.Warn,
	"panic": log.Panic,
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

// logvalue logs the provided value with the given name to the provided zerolog event.
// It handles various Go types, including strings, integers, floats, booleans, time.Duration,
// errors, slices of strings, and byte slices. It also respects the logZeroValues flag,
// which determines whether zero values should be logged or not.
func logvalue(logv *zerolog.Event, n string, val any) {
	if n == "" || val == nil {
		return
	}
	switch tt := val.(type) {
	case string:
		if logZeroValues || tt != "" {
			logv.Str(n, tt)
		}
	case *string:
		if logZeroValues || (tt != nil && *tt != "") {
			logv.Str(n, *tt)
		}
	case *int:
		if logZeroValues || (tt != nil && *tt != 0) {
			logv.Int(n, *tt)
		}
	case int:
		if logZeroValues || tt != 0 {
			logv.Int(n, tt)
		}
	case *int8:
		if logZeroValues || (tt != nil && *tt != 0) {
			logv.Int8(n, *tt)
		}
	case int8:
		if logZeroValues || tt != 0 {
			logv.Int8(n, tt)
		}
	case *int32:
		if logZeroValues || (tt != nil && *tt != 0) {
			logv.Int32(n, *tt)
		}
	case int32:
		if logZeroValues || tt != 0 {
			logv.Int32(n, tt)
		}
	case *int64:
		if logZeroValues || (tt != nil && *tt != 0) {
			logv.Int64(n, *tt)
		}
	case int64:
		if logZeroValues || tt != 0 {
			logv.Int64(n, tt)
		}
	case uint:
		if logZeroValues || tt != 0 {
			logv.Uint(n, tt)
		}
	case *uint:
		if logZeroValues || (tt != nil && *tt != 0) {
			logv.Uint(n, *tt)
		}
	case uint8:
		if logZeroValues || tt != 0 {
			logv.Uint8(n, tt)
		}
	case *uint8:
		if logZeroValues || (tt != nil && *tt != 0) {
			logv.Uint8(n, *tt)
		}
	case uint16:
		if logZeroValues || tt != 0 {
			logv.Uint16(n, tt)
		}
	case *uint16:
		if logZeroValues || (tt != nil && *tt != 0) {
			logv.Uint16(n, *tt)
		}
	case uint32:
		if logZeroValues || tt != 0 {
			logv.Uint32(n, tt)
		}
	case *uint32:
		if logZeroValues || (tt != nil && *tt != 0) {
			logv.Uint32(n, *tt)
		}
	case bool:
		logv.Bool(n, tt)
	case *bool:
		logv.Bool(n, *tt)
	case float64:
		if logZeroValues || tt != 0 {
			logv.Float64(n, tt)
		}
	case float32:
		if logZeroValues || tt != 0 {
			logv.Float32(n, tt)
		}
	case *float64:
		if logZeroValues || (tt != nil && *tt != 0) {
			logv.Float64(n, *tt)
		}
	case *float32:
		if logZeroValues || (tt != nil && *tt != 0) {
			logv.Float32(n, *tt)
		}
	case time.Duration:
		logv.Str(n, tt.Round(time.Second).String())

	case *time.Duration:
		if tt != nil {
			logv.Str(n, tt.Round(time.Second).String())
		}
	case error:
		logv.Err(tt)
	case []string:
		if logZeroValues || len(tt) != 0 {
			logv.Strs(n, tt)
		}
	case *[]string:
		if logZeroValues || (tt != nil && len(*tt) != 0) {
			logv.Strs(n, *tt)
		}
	case []byte:
		if logZeroValues || len(tt) != 0 {
			logv.Bytes(n, tt)
		}
	case *[]byte:
		if logZeroValues || (tt != nil && len(*tt) != 0) {
			logv.Bytes(n, *tt)
		}
	case *bytes.Buffer:
		logv.Bytes(n, tt.Bytes())

	default:
		logv.Any(n, tt)
	}
}
