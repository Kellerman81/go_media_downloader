// Package whisper talks to a self-hosted Whisper ASR webservice
// (https://github.com/ahmetoner/whisper-asr-webservice, image
// onerahmet/openai-whisper-asr-webservice) to detect the spoken language of
// an audio clip.
package whisper

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

// Detection is the language the service heard in one clip.
type Detection struct {
	// Name is the English language name in lower case, e.g. "german".
	Name string `json:"detected_language"`
	// Code is the ISO 639-1 code, e.g. "de".
	Code       string  `json:"language_code"`
	Confidence float64 `json:"confidence"`
}

// Provider is a client for one Whisper ASR webservice instance.
type Provider struct {
	*base.BaseClient
}

// NewProvider creates a client for the service at baseURL, e.g.
// "http://192.168.1.10:9000".
func NewProvider(baseURL, userAgent string) *Provider {
	return &Provider{BaseClient: base.NewBaseClient(base.ClientConfig{
		Name:    "whisper",
		BaseURL: baseURL,
		// CPU detection takes about a second per clip; the first request after
		// a container start also loads the model.
		Timeout:          2 * time.Minute,
		RateLimitCalls:   10,
		RateLimitSeconds: 1,
		UserAgent:        userAgent,
	})}
}

// DetectLanguage sends a WAV clip and returns the language heard in it.
func (p *Provider) DetectLanguage(ctx context.Context, wav []byte) (*Detection, error) {
	var buf bytes.Buffer

	mw := multipart.NewWriter(&buf)

	fw, err := mw.CreateFormFile("audio_file", "clip.wav")
	if err != nil {
		return nil, fmt.Errorf("building whisper request: %w", err)
	}

	if _, err := fw.Write(wav); err != nil {
		return nil, fmt.Errorf("building whisper request: %w", err)
	}

	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("building whisper request: %w", err)
	}

	var det Detection
	if err := p.MakeRequestWithHeaders(ctx, "POST", "/detect-language?encode=true", &buf, &det, nil,
		map[string]string{"Content-Type": mw.FormDataContentType()}); err != nil {
		return nil, err
	}

	if det.Code == "" {
		return nil, fmt.Errorf("whisper returned no language")
	}

	return &det, nil
}
