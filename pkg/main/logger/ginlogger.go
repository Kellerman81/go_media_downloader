package logger

import (
	"bytes"
	"io"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

type Options struct {
	//
	Name string

	// Custom logger
	Logger *zerolog.Logger

	// FieldsOrder defines the order of fields in output.
	FieldsOrder []string

	// FieldsExclude defines contextual fields to not display in output.
	FieldsExclude []string
}

var (
	NameFieldName       = "name"
	HostnameFieldName   = "hostname"
	ClientIPFieldName   = "client_ip"
	UserAgentFieldName  = "user_agent"
	TimestampFieldName  = zerolog.TimestampFieldName
	DurationFieldName   = "elapsed"
	MethodFieldName     = "method"
	PathFieldName       = "path"
	PayloadFieldName    = "payload"
	RefererFieldName    = "referer"
	statusCodeFieldName = "status_code"
	DataLengthFieldName = "data_length"
	BodyFieldName       = "body"
)

func ErrorLogger() gin.HandlerFunc {
	return ErrorLoggerT(gin.ErrorTypeAny)
}

func ErrorLoggerT(typ gin.ErrorType) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if !c.Writer.Written() {
			json := c.Errors.ByType(typ).JSON()
			if json != nil {
				c.JSON(-1, json)
			}
		}
	}
}

// Logger is a gin middleware which use zerolog.
func GinLogger() gin.HandlerFunc {
	o := Options{
		FieldsOrder: ginDefaultFieldsOrder(),
	}
	return LoggerWithOptions(&o)
}

// LoggerWithOptions is a gin middleware which use zerolog.
func LoggerWithOptions(opt *Options) gin.HandlerFunc {
	// List of fields
	if len(opt.FieldsOrder) == 0 {
		opt.FieldsOrder = ginDefaultFieldsOrder()
	}

	// Logger to use
	if opt.Logger == nil {
		opt.Logger = &log
	}

	//
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return func(ctx *gin.Context) {
		// get zerolog
		z := opt.Logger

		// return if zerolog is disabled
		if z.GetLevel() == zerolog.Disabled {
			ctx.Next()
			return
		}

		// before executing the next handlers
		begin := time.Now()
		path := ctx.Request.URL.Path
		raw := ctx.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		// Get payload from request
		var payload []byte
		if !opt.isExcluded(PayloadFieldName) {
			payload, _ = io.ReadAll(ctx.Request.Body)
			ctx.Request.Body = io.NopCloser(bytes.NewReader(payload))
		}

		// Get a copy of the body
		w := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: ctx.Writer}
		ctx.Writer = w

		// executes the pending handlers
		ctx.Next()

		// after executing the handlers
		duration := time.Since(begin)
		statusCode := ctx.Writer.Status()

		//
		var event *zerolog.Event

		// set message level
		switch {
		case statusCode >= 200 && statusCode < 300:
			event = z.Info()
		case statusCode >= 300 && statusCode < 400:
			event = z.Info()
		case statusCode >= 400 && statusCode < 500:
			event = z.Warn()
		case statusCode >= 500:
			event = z.Error()
		default:
			event = z.Trace()
		}

		// add fields
		for _, f := range opt.FieldsOrder {
			// Name field
			if f == NameFieldName && !opt.isExcluded(f) && len(opt.Name) > 0 {
				event.Str(NameFieldName, opt.Name)
			}
			// Hostname field
			if f == HostnameFieldName && !opt.isExcluded(f) && len(hostname) > 0 {
				event.Str(HostnameFieldName, hostname)
			}
			// ClientIP field
			if f == ClientIPFieldName && !opt.isExcluded(f) {
				event.Str(ClientIPFieldName, ctx.ClientIP())
			}
			// UserAgent field
			if f == UserAgentFieldName && !opt.isExcluded(f) && len(ctx.Request.UserAgent()) > 0 {
				event.Str(UserAgentFieldName, ctx.Request.UserAgent())
			}
			// Method field
			if f == MethodFieldName && !opt.isExcluded(f) {
				event.Str(MethodFieldName, ctx.Request.Method)
			}
			// Path field
			if f == PathFieldName && !opt.isExcluded(f) && len(path) > 0 {
				event.Str(PathFieldName, path)
			}
			// Payload field
			if f == PayloadFieldName && !opt.isExcluded(f) && len(payload) > 0 {
				event.Str(PayloadFieldName, string(payload))
			}
			// Timestamp field
			if f == TimestampFieldName && !opt.isExcluded(f) {
				event.Time(TimestampFieldName, begin)
			}
			// Duration field
			if f == DurationFieldName && !opt.isExcluded(f) {
				var durationFieldName string
				switch zerolog.DurationFieldUnit {
				case time.Nanosecond:
					durationFieldName = DurationFieldName + "_ns"
				case time.Microsecond:
					durationFieldName = DurationFieldName + "_us"
				case time.Millisecond:
					durationFieldName = DurationFieldName + "_ms"
				case time.Second:
					durationFieldName = DurationFieldName
				case time.Minute:
					durationFieldName = DurationFieldName + "_min"
				case time.Hour:
					durationFieldName = DurationFieldName + "_hr"
				default:
					z.Error().
						Interface("zerolog.DurationFieldUnit", zerolog.DurationFieldUnit).
						Msg("unknown value for DurationFieldUnit")
					durationFieldName = DurationFieldName
				}
				event.Dur(durationFieldName, duration)
			}
			// Referer field
			if f == RefererFieldName && !opt.isExcluded(f) && len(ctx.Request.Referer()) > 0 {
				event.Str(RefererFieldName, ctx.Request.Referer())
			}
			// statusCode field
			if f == statusCodeFieldName && !opt.isExcluded(f) {
				event.Int(statusCodeFieldName, statusCode)
			}
			// DataLength field
			if f == DataLengthFieldName && !opt.isExcluded(f) && ctx.Writer.Size() > 0 {
				event.Int(DataLengthFieldName, ctx.Writer.Size())
			}
			// Body field
			if f == BodyFieldName && !opt.isExcluded(f) && len(w.body.String()) > 0 {
				event.Str(BodyFieldName, w.body.String())
			}
		}

		// Message
		message := ctx.Errors.String()
		if message == "" {
			message = "Request"
		}

		// post the message
		event.Msg(message)
	}
}

// gormDefaultFieldsOrder defines the default order of fields.
func ginDefaultFieldsOrder() []string {
	return []string{
		NameFieldName,
		HostnameFieldName,
		ClientIPFieldName,
		UserAgentFieldName,
		MethodFieldName,
		PathFieldName,
		PayloadFieldName,
		TimestampFieldName,
		DurationFieldName,
		RefererFieldName,
		statusCodeFieldName,
		DataLengthFieldName,
		BodyFieldName,
	}
}

// isExcluded check if a field is excluded from the output.
func (o *Options) isExcluded(field string) bool {
	if o.FieldsExclude == nil {
		return false
	}
	for _, f := range o.FieldsExclude {
		if f == field {
			return true
		}
	}

	return false
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r responseBodyWriter) WriteString(s string) (n int, err error) {
	r.body.WriteString(s)
	return r.ResponseWriter.WriteString(s)
}
