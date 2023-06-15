package logger

import (
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
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

var (
	Log        zerolog.Logger
	TimeZone   = *time.UTC
	TimeFormat = time.RFC3339Nano
)

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
	var level = int(zerolog.InfoLevel)
	if strings.EqualFold(config.LogLevel, StrDebug) {
		level = int(zerolog.DebugLevel)
	}
	if strings.EqualFold(config.LogLevel, "warning") {
		level = int(zerolog.WarnLevel)
	}
	if config.TimeZone != "" {
		if strings.EqualFold(config.TimeZone, "local") {
			TimeZone = *time.Local
		} else if strings.EqualFold(config.TimeZone, "utc") {
			TimeZone = *time.UTC
		} else {
			TimeZone2, _ := time.LoadLocation(config.TimeZone)
			TimeZone = *TimeZone2
		}
	}

	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.ErrorStackFieldName = "stack"
	zerolog.DisableSampling(true)
	zerolog.TimeFieldFormat = TimeFormat

	Log = zerolog.New(zerolog.MultiLevelWriter(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: TimeFormat,
	}, &lumberjack.Logger{
		Filename:   "./logs/downloader.log",
		MaxSize:    config.LogFileSize, // megabytes
		MaxBackups: config.LogFileCount,
		MaxAge:     28,                 //days
		Compress:   config.LogCompress, // disabled by default
	})).
		Level(zerolog.Level(level)).
		With().
		Timestamp().Caller().
		Logger()
}
func stack() string {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, 2*len(buf))
	}
}
func Logerror(err error, msg string) {
	LogAnyError(err, msg)
	//Log.Error().Err(err).Msg(msg)
}

func LogerrorStr(err error, str1 string, str2 string, msg string) {
	LogAnyError(err, msg, LoggerValue{Name: str1, Value: str2})
	//Log.Error().Err(err).Str(str1, str2).Msg(msg)
}

type LoggerValue struct {
	Name  string
	Value interface{}
}

func LogAnyError(err error, msg string, vals ...LoggerValue) {
	evt := Log.Error()
	if err != nil {
		evt = evt.Err(err)
	}
	for idx := range vals {
		switch tt := vals[idx].Value.(type) {
		case string:
			if len(tt) >= 1 {
				evt.Str(vals[idx].Name, tt)
			}
		case int:
			if tt != 0 {
				evt.Int(vals[idx].Name, tt)
			}
		case uint:
			if tt != 0 {
				evt.Uint(vals[idx].Name, tt)
			}
		default:
			evt.Any(vals[idx].Name, vals[idx].Value)
		}
	}
	if msg != "" {
		evt.Msg(msg)
	} else {
		evt.Send()
	}
}

func LogAnyInfo(msg string, vals ...LoggerValue) {
	evt := Log.Info()
	for idx := range vals {
		switch tt := vals[idx].Value.(type) {
		case string:
			if len(tt) >= 1 {
				evt.Str(vals[idx].Name, tt)
			}
		case int:
			if tt != 0 {
				evt.Int(vals[idx].Name, tt)
			}
		case uint:
			if tt != 0 {
				evt.Uint(vals[idx].Name, tt)
			}
		default:
			evt.Any(vals[idx].Name, vals[idx].Value)
		}
	}
	if msg != "" {
		evt.Msg(msg)
	} else {
		evt.Send()
	}
}

func LogAnyDebug(msg string, vals ...LoggerValue) {
	evt := Log.Debug()
	for idx := range vals {
		switch tt := vals[idx].Value.(type) {
		case string:
			if len(tt) >= 1 {
				evt.Str(vals[idx].Name, tt)
			}
		case int:
			if tt != 0 {
				evt.Int(vals[idx].Name, tt)
			}
		case []int:
			if len(tt) != 0 {
				evt.Ints(vals[idx].Name, tt)
			}
		case uint:
			if tt != 0 {
				evt.Uint(vals[idx].Name, tt)
			}
		default:
			evt.Any(vals[idx].Name, vals[idx].Value)
		}
	}
	if msg != "" {
		evt.Msg(msg)
	} else {
		evt.Send()
	}
}
