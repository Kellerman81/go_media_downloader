package logger

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
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

// maxLoggedPayloadBytes bounds how large a request body may be before its payload
// is skipped for logging. Avoids buffering large uploads into a log field.
const maxLoggedPayloadBytes = 16 << 10 // 16 KiB

func ErrorLogger() gin.HandlerFunc {
	return ErrorLoggerT(gin.ErrorTypeAny)
}

func ErrorLoggerT(typ gin.ErrorType) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if c.Writer.Written() {
			return
		}

		json := c.Errors.ByType(typ).JSON()
		if json != nil {
			c.JSON(-1, json)
		}
	}
}

// GinLogger is a gin middleware that uses zerolog for structured logging.
// Request bodies are not logged: login and config-save POSTs carry the web
// password (= WebAPIKey) and downloader/indexer credentials.
func GinLogger() gin.HandlerFunc {
	o := Options{
		FieldsOrder:   ginDefaultFieldsOrder(),
		FieldsExclude: []string{PayloadFieldName},
	}
	return LoggerWithOptions(&o)
}

// LoggerWithOptions is a gin middleware which use zerolog.
func LoggerWithOptions(opt *Options) gin.HandlerFunc {
	// List of fields
	if len(opt.FieldsOrder) == 0 {
		opt.FieldsOrder = ginDefaultFieldsOrder()
	}

	// Logger to use - defaults to the dedicated web logger (weblogs.log only).
	if opt.Logger == nil {
		opt.Logger = &webLog
	}

	//
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return func(ctx *gin.Context) {
		// get zerolog
		z := opt.Logger

		// return if zerolog is disabled (verbosity is gated by the global level)
		if zerolog.GlobalLevel() == zerolog.Disabled {
			ctx.Next()
			return
		}

		// before executing the next handlers
		begin := time.Now()
		path := ctx.Request.URL.Path

		raw := ctx.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + RedactQuery(raw)
		}

		// Get payload from request
		var (
			payload    []byte
			payloadErr error
		)

		// Only buffer the body for logging when its size is known and small enough.
		// Large or unknown-length (chunked) bodies are left untouched so they are
		// neither truncated for the handler nor fully buffered just to be logged.
		if !opt.isExcluded(PayloadFieldName) &&
			ctx.Request.ContentLength >= 0 &&
			ctx.Request.ContentLength <= maxLoggedPayloadBytes {
			payload, payloadErr = io.ReadAll(ctx.Request.Body)
			if payloadErr != nil {
				// Log the error but continue processing
				z.Warn().Err(payloadErr).Msg("Failed to read request payload")

				payload = []byte{}
			}

			ctx.Request.Body = io.NopCloser(bytes.NewReader(payload))
		}

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
				if strings.HasPrefix(ctx.ContentType(), "application/x-www-form-urlencoded") {
					event.Str(PayloadFieldName, RedactQuery(string(payload)))
				} else {
					event.Str(PayloadFieldName, string(payload))
				}
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
				event.Str(RefererFieldName, RedactURL(ctx.Request.Referer()))
			}

			// statusCode field
			if f == statusCodeFieldName && !opt.isExcluded(f) {
				event.Int(statusCodeFieldName, statusCode)
			}

			// DataLength field
			if f == DataLengthFieldName && !opt.isExcluded(f) && ctx.Writer.Size() > 0 {
				event.Int(DataLengthFieldName, ctx.Writer.Size())
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
	}
}

// isExcluded check if a field is excluded from the output.
func (o *Options) isExcluded(field string) bool {
	if o.FieldsExclude == nil {
		return false
	}

	return slices.Contains(o.FieldsExclude, field)
}

// isSensitiveParam reports whether a query/form parameter name likely holds a
// credential. Substring matches catch config form names such as
// "general_WebAPIKey" or "downloader_Password". "r" is the newznab RSS API key
// and "client" the AcoustID application key.
func isSensitiveParam(name string) bool {
	n := strings.ToLower(name)
	if n == "key" || n == "r" || n == "client" || strings.HasSuffix(n, "_key") {
		return true
	}

	for _, s := range []string{"apikey", "api_key", "token", "password", "passwd", "secret"} {
		if strings.Contains(n, s) {
			return true
		}
	}

	return false
}

// RedactQuery replaces the values of sensitive parameters in a raw
// "a=1&b=2" string, leaving everything else byte-for-byte unchanged.
func RedactQuery(raw string) string {
	if raw == "" {
		return raw
	}

	parts := strings.Split(raw, "&")
	changed := false

	for i, p := range parts {
		k, _, hasValue := strings.Cut(p, "=")
		if !hasValue {
			continue
		}

		name, err := url.QueryUnescape(k)
		if err != nil {
			name = k
		}

		if isSensitiveParam(name) {
			parts[i] = k + "=REDACTED"
			changed = true
		}
	}

	if !changed {
		return raw
	}

	return JoinStringsSep(parts, "&")
}

// secretPathRe matches the services that carry their credential in the URL
// path: Discord and Slack webhooks and the Telegram bot API.
var secretPathRe = regexp.MustCompile(
	`(discord(?:app)?\.com/api/webhooks/[^/?#]+/|api\.telegram\.org/bot)[^/?#]+|(hooks\.slack\.com/services/)[^?#]+`)

// RedactURL applies RedactQuery to the query part of a full URL string and
// hides the credential of webhook-style URLs.
func RedactURL(raw string) string {
	if strings.Contains(raw, "/api/webhooks/") || strings.Contains(raw, "hooks.slack.com") ||
		strings.Contains(raw, "api.telegram.org") {
		raw = secretPathRe.ReplaceAllString(raw, "${1}${2}REDACTED")
	}

	base, query, ok := strings.Cut(raw, "?")
	if !ok {
		return raw
	}

	return JoinStrings(base, "?", RedactQuery(query))
}

// RedactURLError returns err with the URL of a top-level *url.Error redacted,
// since net/http embeds the full request URL (including API keys) in it.
// The underlying cause stays wrapped.
func RedactURLError(err error) error {
	ue, ok := err.(*url.Error)
	if !ok {
		return err
	}

	redacted := RedactURL(ue.URL)
	if redacted == ue.URL {
		return err
	}

	return &url.Error{Op: ue.Op, URL: redacted, Err: ue.Err}
}
