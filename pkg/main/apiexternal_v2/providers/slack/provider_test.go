package slack

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMessageText(t *testing.T) {
	if got := messageText("A & B", "<x>"); got != "*A &amp; B*\n&lt;x&gt;" {
		t.Errorf("got %q", got)
	}
}

func TestTestConnection(t *testing.T) {
	answer := "no_text"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, answer)
	}))
	defer srv.Close()

	p := NewProvider(srv.URL)
	if err := p.TestConnection(context.Background()); err != nil {
		t.Fatalf("valid webhook: %v", err)
	}

	answer = "invalid_payload"
	if err := p.TestConnection(context.Background()); err == nil {
		t.Fatal("other 400 accepted")
	}
}
