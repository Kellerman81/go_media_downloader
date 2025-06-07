package logger

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestInitLogger_CustomTimeFormat(t *testing.T) {
	config := Config{
		TimeFormat: "2006-01-02 15:04:05",
		LogLevel:   "info",
	}
	InitLogger(config)

	if timeFormat != config.TimeFormat {
		t.Errorf("Expected timeFormat to be %s, got %s", config.TimeFormat, timeFormat)
	}
}

func TestInitLogger_CustomTimeZone(t *testing.T) {
	config := Config{
		TimeZone: "America/New_York",
		LogLevel: "info",
	}
	InitLogger(config)

	location, _ := time.LoadLocation("America/New_York")
	if timeZone.String() != location.String() {
		t.Errorf("Expected timezone to be %s, got %s", location, &timeZone)
	}
}

func TestLogDynamicany_NilValues(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf)

	LogDynamicany("info", "test message", "field1", nil, "field2", nil)

	if buf.Len() == 0 {
		t.Error("Expected log output, got nothing")
	}
}

func TestLogDynamicany_MixedTypes(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf)

	LogDynamicany("info", "test message",
		"string", "value",
		"int", 42,
		"bool", true,
		"float", 3.14,
		"error", errors.New("test error"))

	if buf.Len() == 0 {
		t.Error("Expected log output, got nothing")
	}
}

func TestLogDynamicany_InvalidLevel(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf)

	LogDynamicany("invalid_level", "test message", "field", "value")

	if buf.Len() == 0 {
		t.Error("Expected fallback to info level and log output, got nothing")
	}
}

func TestInitLogger_LogToFileOnly(t *testing.T) {
	config := Config{
		LogToFileOnly: true,
		LogLevel:      "info",
	}
	InitLogger(config)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	LogDynamicany0("info", "test message")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)

	if buf.Len() > 0 {
		t.Error("Expected no output to stdout when LogToFileOnly is true")
	}
}

func TestLogDynamicany_EmptyFieldNames(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf)

	LogDynamicany("info", "test message", "", "value", "field2", "value2")

	if buf.Len() == 0 {
		t.Error("Expected log output despite empty field name")
	}
}

func TestLogDynamicany_PointerTypes(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf)

	str := "test"
	num := 42
	b := true
	f := 3.14

	LogDynamicany("info", "test message",
		"strPtr", &str,
		"intPtr", &num,
		"boolPtr", &b,
		"floatPtr", &f)

	if buf.Len() == 0 {
		t.Error("Expected log output for pointer types")
	}
}

func TestLogDynamicany_BytesBuffer(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf)

	testBuf := bytes.NewBufferString("test data")
	LogDynamicany("info", "test message", "buffer", testBuf)

	if buf.Len() == 0 {
		t.Error("Expected log output for bytes.Buffer")
	}
}

func TestLogDynamicany_DurationTypes(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf)

	duration := 5 * time.Second
	LogDynamicany("info", "test message", "duration", duration)

	if buf.Len() == 0 {
		t.Error("Expected log output for duration type")
	}
}
