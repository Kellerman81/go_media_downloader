package apiexternal_v2

import (
	"context"
	"strings"
)

// Client-reported states of one download.
const (
	DownloadQueued      = "queued"      // added, not transferring (waiting, paused, checking)
	DownloadDownloading = "downloading" // transferring
	DownloadCompleted   = "completed"   // all data is there (seeding counts as completed)
	DownloadFailed      = "failed"      // the client gave up on it
)

// DownloadState is what a client reports about one download.
type DownloadState struct {
	Name     string
	State    string  // one of the Download* constants
	Progress float64 // 0-100
	Message  string  // the client's error or status text, if any
}

// DownloadStateLister is implemented by download clients that can report the
// state of all their downloads, finished and failed ones included.
type DownloadStateLister interface {
	// DownloadStates is keyed by NormalizeDownloadID of the id the add call
	// returned (info hash, job id, gid, ...).
	DownloadStates(ctx context.Context) (map[string]DownloadState, error)
}

// DownloadRemover is implemented by download clients that can remove a
// download, optionally with its data.
type DownloadRemover interface {
	RemoveDownload(ctx context.Context, id string, deleteData bool) error
}

// NormalizeDownloadID is the form download ids are compared in. Ids are
// stored as the client returned them: some (SABnzbd's nzo_id) are
// case-sensitive in the client's own calls.
func NormalizeDownloadID(id string) string {
	return strings.ToUpper(strings.TrimSpace(id))
}

// ProgressState returns DownloadCompleted at 100% and otherwise
// DownloadDownloading or DownloadQueued, for clients without a usable state.
func ProgressState(progress float64, active bool) string {
	switch {
	case progress >= 100:
		return DownloadCompleted
	case active:
		return DownloadDownloading
	default:
		return DownloadQueued
	}
}
