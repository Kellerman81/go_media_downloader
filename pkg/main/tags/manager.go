package tags

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// Manager provides a unified interface for reading and writing audio tags
// across multiple audio formats. It automatically dispatches operations
// to the appropriate format-specific handler based on file extension.
type Manager struct {
	mu       sync.RWMutex
	handlers map[string]TagHandler
}

// NewManager creates a new tag manager with default handlers registered
// for MP3, FLAC, and OGG formats.
func NewManager() *Manager {
	m := &Manager{
		handlers: make(map[string]TagHandler),
	}

	// Register default handlers
	m.RegisterHandler(NewMP3Handler())
	m.RegisterHandler(NewFLACHandlerWithConfig())
	m.RegisterHandler(NewOGGHandler())

	return m
}

// NewFLACHandlerWithConfig creates a FLAC handler that uses the metaflac path from config.
func NewFLACHandlerWithConfig() *FLACHandler {
	h := &FLACHandler{}
	// Get path from config if available
	if cfg := config.GetSettingsGeneral(); cfg != nil && cfg.MetaflacPath != "" {
		h.MetaflacPath = cfg.MetaflacPath
	}

	return h
}

// RegisterHandler registers a tag handler for its supported formats.
// This can be used to add support for additional formats or override
// existing handlers.
func (m *Manager) RegisterHandler(handler TagHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ext := range handler.SupportedFormats() {
		m.handlers[strings.ToLower(ext)] = handler
	}
}

// GetHandler returns the handler for a given file extension.
// Returns nil if no handler is registered for the format.
func (m *Manager) GetHandler(ext string) TagHandler {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.handlers[strings.ToLower(ext)]
}

// SupportedFormats returns a list of all supported file extensions.
func (m *Manager) SupportedFormats() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	formats := make([]string, 0, len(m.handlers))
	for ext := range m.handlers {
		formats = append(formats, ext)
	}

	return formats
}

// IsSupported checks if a file format is supported based on its extension.
func (m *Manager) IsSupported(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.handlers[ext]

	return ok
}

// ReadTags reads metadata tags from an audio file.
// The appropriate handler is selected based on the file extension.
func (m *Manager) ReadTags(path string) (*AudioTags, error) {
	ext := strings.ToLower(filepath.Ext(path))

	m.mu.RLock()

	handler, ok := m.handlers[ext]
	m.mu.RUnlock()

	if !ok {
		return nil, &ErrUnsupportedFormat{Format: ext}
	}

	return handler.ReadTags(path)
}

// WriteTags writes metadata tags to an audio file.
// The appropriate handler is selected based on the file extension.
func (m *Manager) WriteTags(ctx context.Context, path string, tags *AudioTags) error {
	ext := strings.ToLower(filepath.Ext(path))

	m.mu.RLock()

	handler, ok := m.handlers[ext]
	m.mu.RUnlock()

	if !ok {
		return &ErrUnsupportedFormat{Format: ext}
	}

	return handler.WriteTags(ctx, path, tags)
}

// CopyTags copies tags from a source file to a destination file.
// Both files must be in supported formats (can be different formats).
// Cover art is preserved: if the source handler implements CoverTagReader,
// ReadTagsWithCover is used; otherwise ReadTags is used as fallback.
func (m *Manager) CopyTags(ctx context.Context, srcPath, dstPath string) error {
	ext := strings.ToLower(filepath.Ext(srcPath))

	m.mu.RLock()

	handler, ok := m.handlers[ext]
	m.mu.RUnlock()

	if !ok {
		return &ErrUnsupportedFormat{Format: ext}
	}

	var (
		t   *AudioTags
		err error
	)

	if cr, ok := handler.(CoverTagReader); ok {
		t, err = cr.ReadTagsWithCover(srcPath)
	} else {
		t, err = handler.ReadTags(srcPath)
	}

	if err != nil {
		return err
	}

	return m.WriteTags(ctx, dstPath, t)
}

// MergeTags merges source tags into destination tags.
// Only non-empty fields from src are copied to dst.
func MergeTags(dst, src *AudioTags) {
	if src.Title != "" {
		dst.Title = src.Title
	}

	if src.Artist != "" {
		dst.Artist = src.Artist
	}

	if src.Album != "" {
		dst.Album = src.Album
	}

	if src.AlbumArtist != "" {
		dst.AlbumArtist = src.AlbumArtist
	}

	if src.Genre != "" {
		dst.Genre = src.Genre
	}

	if src.Year > 0 {
		dst.Year = src.Year
	}

	if src.TrackNumber > 0 {
		dst.TrackNumber = src.TrackNumber
	}

	if src.TotalTracks > 0 {
		dst.TotalTracks = src.TotalTracks
	}

	if src.DiscNumber > 0 {
		dst.DiscNumber = src.DiscNumber
	}

	if src.TotalDiscs > 0 {
		dst.TotalDiscs = src.TotalDiscs
	}

	if src.Comment != "" {
		dst.Comment = src.Comment
	}

	if src.Composer != "" {
		dst.Composer = src.Composer
	}

	if src.Conductor != "" {
		dst.Conductor = src.Conductor
	}

	if src.Label != "" {
		dst.Label = src.Label
	}

	if src.CatalogNum != "" {
		dst.CatalogNum = src.CatalogNum
	}

	if src.ISRC != "" {
		dst.ISRC = src.ISRC
	}

	if src.Lyrics != "" {
		dst.Lyrics = src.Lyrics
	}

	if src.Copyright != "" {
		dst.Copyright = src.Copyright
	}

	if src.MBRecordingID != "" {
		dst.MBRecordingID = src.MBRecordingID
	}

	if src.MBReleaseID != "" {
		dst.MBReleaseID = src.MBReleaseID
	}

	if src.MBReleaseGroupID != "" {
		dst.MBReleaseGroupID = src.MBReleaseGroupID
	}

	if src.MBArtistID != "" {
		dst.MBArtistID = src.MBArtistID
	}

	if src.MBAlbumArtistID != "" {
		dst.MBAlbumArtistID = src.MBAlbumArtistID
	}

	if src.AcoustID != "" {
		dst.AcoustID = src.AcoustID
	}

	if src.ReplayGainTrack != 0 {
		dst.ReplayGainTrack = src.ReplayGainTrack
	}

	if src.ReplayGainTrackPeak != 0 {
		dst.ReplayGainTrackPeak = src.ReplayGainTrackPeak
	}

	if src.ReplayGainAlbum != 0 {
		dst.ReplayGainAlbum = src.ReplayGainAlbum
	}

	if src.ReplayGainAlbumPeak != 0 {
		dst.ReplayGainAlbumPeak = src.ReplayGainAlbumPeak
	}

	if len(src.CoverData) > 0 {
		dst.CoverData = src.CoverData
		dst.CoverMIME = src.CoverMIME
	}
}

// DefaultManager is a package-level manager instance for convenience.
var DefaultManager = NewManager()

// ReadTags is a convenience function that uses the default manager.
func ReadTags(path string) (*AudioTags, error) {
	return DefaultManager.ReadTags(path)
}

// WriteTags is a convenience function that uses the default manager.
func WriteTags(ctx context.Context, path string, tags *AudioTags) error {
	return DefaultManager.WriteTags(ctx, path, tags)
}

// IsSupported is a convenience function that uses the default manager.
func IsSupported(path string) bool {
	return DefaultManager.IsSupported(path)
}

// formatPaddedNum formats a number with zero-padding based on the total.
// Width is 2 by default, 3 if total >= 100, 4 if total >= 1000.
func formatPaddedNum(num, total int) string {
	var width int
	switch {
	case total >= 1000:
		width = 4
	case total >= 100:
		width = 3
	default:
		width = 2
	}

	return fmt.Sprintf("%0*d", width, num)
}
