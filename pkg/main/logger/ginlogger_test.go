package logger

import (
	"context"
	"errors"
	"net/url"
	"testing"
)

func TestRedactQuery(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"a=1&b=2", "a=1&b=2"},
		{"apikey=secret", "apikey=REDACTED"},
		{"ApiKey=secret&x=1", "ApiKey=REDACTED&x=1"},
		{"x=1&api_key=s&token=t&password=p&key=k", "x=1&api_key=REDACTED&token=REDACTED&password=REDACTED&key=REDACTED"},
		{"general_WebAPIKey=abc&downloader_Password=pw&csrf_token=c", "general_WebAPIKey=REDACTED&downloader_Password=REDACTED&csrf_token=REDACTED"},
		{"monkey=1&keyboard=2", "monkey=1&keyboard=2"},
		{"api%6Bey=s", "api%6Bey=REDACTED"},
		{"flag&apikey=s", "flag&apikey=REDACTED"},
		{"r=s&i=1&t=search", "r=REDACTED&i=1&t=search"},
		{"client=s&meta=x", "client=REDACTED&meta=x"},
		{"X-Plex-Token=s&type=15", "X-Plex-Token=REDACTED&type=15"},
	}

	for _, tt := range tests {
		if got := RedactQuery(tt.in); got != tt.want {
			t.Errorf("RedactQuery(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRedactURL(t *testing.T) {
	if got := RedactURL("http://h/api/admin?apikey=s&a=1"); got != "http://h/api/admin?apikey=REDACTED&a=1" {
		t.Errorf("got %q", got)
	}

	if got := RedactURL("http://h/api/admin"); got != "http://h/api/admin" {
		t.Errorf("got %q", got)
	}

	for in, want := range map[string]string{
		"https://discord.com/api/webhooks/123/abc-tok":        "https://discord.com/api/webhooks/123/REDACTED",
		"https://hooks.slack.com/services/T1/B2/xyz":          "https://hooks.slack.com/services/REDACTED",
		"https://api.telegram.org/bot12:AA-b/sendMessage?x=1": "https://api.telegram.org/botREDACTED/sendMessage?x=1",
		"https://discordapp.com/api/webhooks/1/t?wait=true":   "https://discordapp.com/api/webhooks/1/REDACTED?wait=true",
	} {
		if got := RedactURL(in); got != want {
			t.Errorf("RedactURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactURLError(t *testing.T) {
	err := &url.Error{Op: "Get", URL: "http://h/api?apikey=s&t=1", Err: context.Canceled}

	got := RedactURLError(err)
	if got.Error() != `Get "http://h/api?apikey=REDACTED&t=1": context canceled` {
		t.Errorf("got %q", got.Error())
	}

	if !errors.Is(got, context.Canceled) {
		t.Error("underlying error not wrapped")
	}

	plain := errors.New("x")
	if RedactURLError(plain) != plain {
		t.Error("non-url error changed")
	}
}

func TestIsSensitiveParam(t *testing.T) {
	for in, want := range map[string]bool{
		"apikey": true, "general_WebAPIKey": true, "downloader_Password": true, "r": true, "client": true,
		"key": true, "tvdb_key": true, "AccessToken": true, "client_secret": true,
		"t": false, "q": false, "keyword": false, "cat": false, "limit": false,
	} {
		if got := isSensitiveParam(in); got != want {
			t.Errorf("isSensitiveParam(%q) = %t, want %t", in, got, want)
		}
	}
}
