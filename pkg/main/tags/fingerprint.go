package tags

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/goccy/go-json"
)

// fpcalcOutput is the JSON shape returned by fpcalc -json.
// Defined at package level so the variable in GenerateFingerprint escapes to heap
// once (at most) rather than once per call.
type fpcalcOutput struct {
	Duration    float64 `json:"duration"`
	Fingerprint string  `json:"fingerprint"`
}

// FingerprintResult represents the result of audio fingerprinting.
type FingerprintResult struct {
	Fingerprint string
	Duration    int // Duration in seconds
}

// IdentificationResult represents the result of identifying a track.
type IdentificationResult struct {
	AcoustID       string
	Score          float64
	MusicBrainzID  string // MusicBrainz recording ID
	ReleaseID      string // MusicBrainz release (album) ID
	RecordingTitle string
	Artist         string
	Album          string
	TrackNumber    int
	DiscNumber     int
}

// GenerateFingerprint creates a Chromaprint fingerprint for an audio file using fpcalc.
func GenerateFingerprint(ctx context.Context, audioPath string) (*FingerprintResult, error) {
	fpcalcPath := config.GetSettingsGeneral().FpcalcPath
	if fpcalcPath == "" {
		fpcalcPath = "fpcalc" // Default to PATH
	}

	cmd := exec.CommandContext(ctx, fpcalcPath, "-json", audioPath)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("fpcalc error: %w", err)
	}

	var result fpcalcOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse fpcalc output: %w", err)
	}

	return &FingerprintResult{
		Fingerprint: result.Fingerprint,
		Duration:    int(result.Duration),
	}, nil
}

// IdentifyTrack identifies a track using its fingerprint via AcoustID.
func IdentifyTrack(
	ctx context.Context,
	fingerprint string,
	duration int,
) (*IdentificationResult, error) {
	provider := providers.GetAcoustID()
	if provider == nil {
		return nil, fmt.Errorf("AcoustID provider not initialized")
	}

	matches, err := provider.LookupByFingerprint(ctx, fingerprint, duration)
	if err != nil {
		return nil, fmt.Errorf("AcoustID lookup error: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no matches found")
	}

	// Get the best match (highest score)
	bestMatch := matches[0]

	return &IdentificationResult{
		AcoustID:       bestMatch.AcoustID,
		Score:          bestMatch.Score,
		MusicBrainzID:  bestMatch.MusicBrainzID,
		ReleaseID:      bestMatch.ReleaseID,
		RecordingTitle: bestMatch.RecordingTitle,
		Artist:         bestMatch.ArtistName,
		Album:          bestMatch.Album,
		TrackNumber:    bestMatch.TrackNumber,
		DiscNumber:     bestMatch.DiscNumber,
	}, nil
}

// FingerprintReleaseIDs returns all unique MusicBrainz release IDs from AcoustID for an
// audio file. Unlike FingerprintAndIdentify, this collects release IDs from every
// RecordingMatch in the response, not just the best one, so callers can perform
// majority voting across multiple tracks.
func FingerprintReleaseIDs(ctx context.Context, audioPath string) ([]string, error) {
	provider := providers.GetAcoustID()
	if provider == nil {
		return nil, fmt.Errorf("AcoustID provider not initialized")
	}

	fp, err := GenerateFingerprint(ctx, audioPath)
	if err != nil {
		return nil, fmt.Errorf("fingerprint generation failed: %w", err)
	}

	matches, err := provider.LookupByFingerprint(ctx, fp.Fingerprint, fp.Duration)
	if err != nil {
		return nil, fmt.Errorf("AcoustID lookup error: %w", err)
	}

	seen := make(map[string]struct{}, len(matches))
	releaseIDs := make([]string, 0, len(matches))

	for i := range matches {
		if id := matches[i].ReleaseID; id != "" {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				releaseIDs = append(releaseIDs, id)
			}
		}
	}

	return releaseIDs, nil
}

// FingerprintAndIdentify generates a fingerprint and identifies the track in one operation.
func FingerprintAndIdentify(ctx context.Context, audioPath string) (*IdentificationResult, error) {
	if providers.GetAcoustID() == nil {
		return nil, fmt.Errorf("AcoustID provider not initialized")
	}

	if config.GetSettingsGeneral().FpcalcPath == "" && !commandExists("fpcalc") {
		return nil, fmt.Errorf("fpcalc not found in PATH and FpcalcPath not configured")
	}

	// Generate fingerprint
	logger.Logtype("debug", 2).Str("file", audioPath).Msg("Generating audio fingerprint")

	fingerprintResult, err := GenerateFingerprint(ctx, audioPath)
	if err != nil {
		return nil, fmt.Errorf("fingerprint generation failed: %w", err)
	}

	logger.Logtype("debug", 2).
		Str("file", audioPath).
		Int("duration", fingerprintResult.Duration).
		Msg("Fingerprint generated, looking up in AcoustID")

	// Identify track
	result, err := IdentifyTrack(ctx, fingerprintResult.Fingerprint, fingerprintResult.Duration)
	if err != nil {
		return nil, fmt.Errorf("track identification failed: %w", err)
	}

	logger.Logtype("debug", 1).
		Str("file", audioPath).
		Str("acoustid", result.AcoustID).
		Str("title", result.RecordingTitle).
		Str("artist", result.Artist).
		Float64("score", result.Score).
		Msg("Track identified via AcoustID")

	return result, nil
}

// FingerprintAndIdentifyWithTimeout is a convenience function that adds a timeout.
func FingerprintAndIdentifyWithTimeout(
	audioPath string,
	timeout time.Duration,
) (*IdentificationResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return FingerprintAndIdentify(ctx, audioPath)
}

// commandExists checks if a command exists in PATH.
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
