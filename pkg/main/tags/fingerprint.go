package tags

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
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

// configuredFpcalcPath returns the configured fpcalc path, or "" when config isn't loaded.
func configuredFpcalcPath() string {
	if general := config.GetSettingsGeneral(); general != nil {
		return general.FpcalcPath
	}

	return ""
}

// GenerateFingerprint creates a Chromaprint fingerprint for an audio file using fpcalc.
func GenerateFingerprint(ctx context.Context, audioPath string) (*FingerprintResult, error) {
	fpcalcPath := configuredFpcalcPath()
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
		return nil, errors.New("AcoustID provider not initialized")
	}

	matches, err := provider.LookupByFingerprint(ctx, fingerprint, duration)
	if err != nil {
		return nil, fmt.Errorf("AcoustID lookup error: %w", err)
	}

	if len(matches) == 0 {
		return nil, errors.New("no matches found")
	}

	best := bestAcoustIDMatch(matches)
	if best == nil {
		return nil, fmt.Errorf("no match with score >= %.2f", acoustIDMinScore)
	}

	title := best.RecordingTitle
	if title == "" {
		title = best.Title
	}

	artist := best.ArtistName
	if artist == "" && len(best.Artists) > 0 {
		artist = strings.Join(best.Artists, ", ")
	}

	return &IdentificationResult{
		AcoustID:       best.AcoustID,
		Score:          best.Score,
		MusicBrainzID:  best.MusicBrainzID,
		ReleaseID:      best.ReleaseID,
		RecordingTitle: title,
		Artist:         artist,
		Album:          best.Album,
		TrackNumber:    best.TrackNumber,
		DiscNumber:     best.DiscNumber,
	}, nil
}

// acoustIDMinScore is the minimum AcoustID match score accepted as an
// identification; below it the fingerprint match is too weak to tag from.
const acoustIDMinScore = 0.5

// bestAcoustIDMatch returns the highest-scoring match, or nil when none
// reaches acoustIDMinScore. The response is not guaranteed to be sorted.
func bestAcoustIDMatch(matches []apiexternal_v2.RecordingMatch) *apiexternal_v2.RecordingMatch {
	var best *apiexternal_v2.RecordingMatch

	for i := range matches {
		if matches[i].Score >= acoustIDMinScore && (best == nil || matches[i].Score > best.Score) {
			best = &matches[i]
		}
	}

	return best
}

// FingerprintReleaseIDs returns all unique MusicBrainz release IDs from AcoustID for an
// audio file. Unlike FingerprintAndIdentify, this collects release IDs from every
// RecordingMatch in the response, not just the best one, so callers can perform
// majority voting across multiple tracks.
func FingerprintReleaseIDs(ctx context.Context, audioPath string) ([]string, error) {
	provider := providers.GetAcoustID()
	if provider == nil {
		return nil, errors.New("AcoustID provider not initialized")
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
		return nil, errors.New("AcoustID provider not initialized")
	}

	if configuredFpcalcPath() == "" && !commandExists("fpcalc") {
		return nil, errors.New("fpcalc not found in PATH and FpcalcPath not configured")
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
