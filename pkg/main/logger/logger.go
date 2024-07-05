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
	var level = zerolog.Level(zerolog.InfoLevel)
	if strings.EqualFold(config.LogLevel, StrDebug) {
		level = zerolog.Level(zerolog.DebugLevel)
		dbug = true
	}
	if strings.EqualFold(config.LogLevel, "warning") {
		level = zerolog.Level(zerolog.WarnLevel)
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
	var writers []io.Writer

	if !config.LogToFileOnly {
		if config.LogColorize {
			writers = append(writers, zerolog.ConsoleWriter{Out: os.Stdout})
		} else {
			writers = append(writers, os.Stdout)
		}
	}
	logctx := zerolog.New(zerolog.MultiLevelWriter(append(writers, &lumberjack.Logger{
		Filename:   logfile,
		MaxSize:    config.LogFileSize, // megabytes
		MaxBackups: int(config.LogFileCount),
		MaxAge:     28,                 //days
		Compress:   config.LogCompress, // disabled by default
	})...)).Level(level).With().Timestamp()
	if dbug {
		log = logctx.Caller().Logger()
	} else {
		log = logctx.Logger()
	}
}

// LogDynamicany logs a message with dynamic fields. The 'typev' parameter specifies the log level (info, debug, error, fatal, warn, panic). The 'msg' parameter is the log message. The 'fields' parameter is a variadic list of key-value pairs to be logged.
func LogDynamicany(typev string, msg string, fields ...any) {
	var logv *zerolog.Event
	switch typev {
	case "info":
		logv = log.Info()
	case "debug":
		logv = log.Debug()
	case "error":
		logv = log.Error()
	case "fatal":
		logv = log.Fatal()
	case "warn":
		logv = log.Warn()
	case "panic":
		logv = log.Panic()
	default:
		logv = log.Info()
	}
	logv.CallerSkipFrame(1)

	var n string
	for i := range fields {
		switch tt := fields[i].(type) {
		case string:
			if n == "" {
				n = tt
			} else {
				if logZeroValues || tt != "" {
					logv.Str(n, tt)
				}
				n = ""
			}
		case *string:
			if n == "" {
				n = *tt
			} else {
				if logZeroValues || *tt != "" {
					logv.Str(n, *tt)
				}
				n = ""
			}
		case *int:
			if n != "" {
				if logZeroValues || *tt != 0 {
					logv.Int(n, *tt)
				}
				n = ""
			}
		case int:
			if n != "" {
				if logZeroValues || tt != 0 {
					logv.Int(n, tt)
				}
				n = ""
			}
		case *int8:
			if n != "" {
				if logZeroValues || *tt != 0 {
					logv.Int8(n, *tt)
				}
				n = ""
			}
		case int8:
			if n != "" {
				if logZeroValues || tt != 0 {
					logv.Int8(n, tt)
				}
				n = ""
			}
		case *int32:
			if n != "" {
				if logZeroValues || *tt != 0 {
					logv.Int32(n, *tt)
				}
				n = ""
			}
		case int32:
			if n != "" {
				if logZeroValues || tt != 0 {
					logv.Int32(n, tt)
				}
				n = ""
			}
		case *int64:
			if n != "" {
				if logZeroValues || *tt != 0 {
					logv.Int64(n, *tt)
				}
				n = ""
			}
		case int64:
			if n != "" {
				if logZeroValues || tt != 0 {
					logv.Int64(n, tt)
				}
				n = ""
			}
		case uint:
			if n != "" {
				if logZeroValues || tt != 0 {
					logv.Uint(n, tt)
				}
				n = ""
			}
		case *uint:
			if n != "" {
				if logZeroValues || *tt != 0 {
					logv.Uint(n, *tt)
				}
				n = ""
			}
		case uint8:
			if n != "" {
				if logZeroValues || tt != 0 {
					logv.Uint8(n, tt)
				}
				n = ""
			}
		case *uint8:
			if n != "" {
				if logZeroValues || *tt != 0 {
					logv.Uint8(n, *tt)
				}
				n = ""
			}
		case uint16:
			if n != "" {
				if logZeroValues || tt != 0 {
					logv.Uint16(n, tt)
				}
				n = ""
			}
		case *uint16:
			if n != "" {
				if logZeroValues || *tt != 0 {
					logv.Uint16(n, *tt)
				}
				n = ""
			}
		case uint32:
			if n != "" {
				if logZeroValues || tt != 0 {
					logv.Uint32(n, tt)
				}
				n = ""
			}
		case *uint32:
			if n != "" {
				if logZeroValues || *tt != 0 {
					logv.Uint32(n, *tt)
				}
				n = ""
			}
		case bool:
			if n != "" {
				logv.Bool(n, tt)
				n = ""
			}
		case *bool:
			if n != "" {
				logv.Bool(n, *tt)
				n = ""
			}
		case float64:
			if n != "" {
				if logZeroValues || tt != 0 {
					logv.Float64(n, tt)
				}
				n = ""
			}
		case float32:
			if n != "" {
				if logZeroValues || tt != 0 {
					logv.Float32(n, tt)
				}
				n = ""
			}
		case *float64:
			if n != "" {
				if logZeroValues || *tt != 0 {
					logv.Float64(n, *tt)
				}
				n = ""
			}
		case *float32:
			if n != "" {
				if logZeroValues || *tt != 0 {
					logv.Float32(n, *tt)
				}
				n = ""
			}
		case time.Duration:
			if n != "" {
				logv.Str(n, tt.Round(time.Second).String())
				n = ""
			}
		case *time.Duration:
			if n != "" {
				logv.Str(n, tt.Round(time.Second).String())
				n = ""
			}
		case error:
			logv.Err(tt)
			n = ""
		case []string:
			if n != "" {
				if logZeroValues || len(tt) != 0 {
					logv.Strs(n, tt)
				}
				n = ""
			}
		case *[]string:
			if n != "" {
				if logZeroValues || len(*tt) != 0 {
					logv.Strs(n, *tt)
				}
				n = ""
			}
		case []byte:
			if n != "" {
				if logZeroValues || len(tt) != 0 {
					logv.Bytes(n, tt)
				}
				n = ""
			}
		case *[]byte:
			if n != "" {
				if logZeroValues || len(*tt) != 0 {
					logv.Bytes(n, *tt)
				}
				n = ""
			}
		case *bytes.Buffer:
			if n != "" {
				logv.Bytes(n, tt.Bytes())
				n = ""
			}
		default:
			if n != "" {
				logv.Any(n, tt)
				n = ""
			}
		}
	}
	logv.Msg(msg)
}

// GetLogger returns the global zerolog logger instance.
func GetLogger() *zerolog.Logger {
	return &log
}
