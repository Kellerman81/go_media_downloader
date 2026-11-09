package whisper

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	var gotPath, gotFile string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()

		f, _, err := r.FormFile("audio_file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		b, _ := io.ReadAll(f)
		gotFile = string(b)

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"detected_language":"german","language_code":"de","confidence":0.93}`)
	}))
	defer srv.Close()

	det, err := NewProvider(srv.URL, "test").DetectLanguage(context.Background(), []byte("RIFFwav"))
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/detect-language?encode=true" || gotFile != "RIFFwav" {
		t.Errorf("request: path %q, file %q", gotPath, gotFile)
	}

	if det.Code != "de" || det.Name != "german" || det.Confidence != 0.93 {
		t.Errorf("detection = %+v", det)
	}
}
