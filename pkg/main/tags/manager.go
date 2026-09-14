package tags

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
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
	m.RegisterHandler(NewMP4Handler())

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
	ext := logger.FileExt(path)

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.handlers[ext]

	return ok
}

// ReadTags reads metadata tags from an audio file.
// The appropriate handler is selected based on the file extension.
func (m *Manager) ReadTags(path string) (*AudioTags, error) {
	ext := logger.FileExt(path)

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
	ext := logger.FileExt(path)

	m.mu.RLock()

	handler, ok := m.handlers[ext]
	m.mu.RUnlock()

	if !ok {
		return &ErrUnsupportedFormat{Format: ext}
	}

	return handler.WriteTags(ctx, path, tags)
}

// ReadCoverData returns the embedded cover art already present in the file
// at path, if any, using ReadTagsWithCover when the format's handler
// supports it. Returns (nil, "") if the format has no cover-reading
// support (e.g. OGG/Opus currently) or the file has no embedded cover.
//
// Callers that write new tags onto an already-tagged file without an
// explicit replacement cover (e.g. structure.TagAlbumFiles when no new
// cover was fetched for the album) must call this first and carry the
// result into AudioTags.CoverData/CoverMIME - WriteTags's underlying
// libraries can drop the existing embedded cover on save otherwise
// (confirmed for MP3: id3v2's ParseFrames option, used here to avoid
// loading cover bytes on writes that don't need them, discards frames not
// in its list entirely rather than round-tripping them unmodified).
func (m *Manager) ReadCoverData(path string) ([]byte, string) {
	ext := logger.FileExt(path)

	m.mu.RLock()
	handler, ok := m.handlers[ext]
	m.mu.RUnlock()

	if !ok {
		return nil, ""
	}

	cr, ok := handler.(CoverTagReader)
	if !ok {
		return nil, ""
	}

	t, err := cr.ReadTagsWithCover(path)
	if err != nil || t == nil {
		return nil, ""
	}

	return t.CoverData, t.CoverMIME
}

// ReadCoverData is a convenience function that uses the default manager.
func ReadCoverData(path string) ([]byte, string) {
	return DefaultManager.ReadCoverData(path)
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
