package deluge

import (
	"context"
	"errors"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

// downloadState maps a Deluge torrent state.
func downloadState(state string, progress float64) string {
	switch state {
	case "Error":
		return apiexternal_v2.DownloadFailed
	case "Seeding":
		return apiexternal_v2.DownloadCompleted
	case "Downloading":
		return apiexternal_v2.DownloadDownloading
	}

	return apiexternal_v2.ProgressState(progress, false)
}

// DownloadStates implements apiexternal_v2.DownloadStateLister.
func (p *Provider) DownloadStates(ctx context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	result, err := p.call(ctx, "core.get_torrents_status",
		[]any{map[string]any{}, []string{"name", "state", "progress", "message"}})
	if err != nil {
		return nil, err
	}

	torrents, ok := result.(map[string]any)
	if !ok {
		return nil, errors.New("deluge: unexpected torrent status format")
	}

	out := make(map[string]apiexternal_v2.DownloadState, len(torrents))
	for hash, data := range torrents {
		m, ok := data.(map[string]any)
		if !ok {
			continue
		}

		name, _ := m["name"].(string)
		state, _ := m["state"].(string)
		progress, _ := m["progress"].(float64)
		message, _ := m["message"].(string)

		s := apiexternal_v2.DownloadState{Name: name, State: downloadState(state, progress), Progress: progress}
		if s.State == apiexternal_v2.DownloadFailed || !strings.EqualFold(message, "OK") {
			s.Message = message
		}

		out[apiexternal_v2.NormalizeDownloadID(hash)] = s
	}

	return out, nil
}

// RemoveDownload implements apiexternal_v2.DownloadRemover.
func (p *Provider) RemoveDownload(ctx context.Context, id string, deleteData bool) error {
	return p.RemoveTorrent(ctx, strings.ToLower(id), deleteData)
}
