package flood

import (
	"context"
	"net/http"
	"slices"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

// downloadState maps Flood's status flags. Flood flags "error" for any
// client message, tracker warnings included, so like Sonarr it is passed on
// as a message, not treated as a failed download.
func downloadState(status []string, percent float64) string {
	switch {
	case slices.Contains(status, "complete") || percent >= 100:
		return apiexternal_v2.DownloadCompleted
	case slices.Contains(status, "downloading"):
		return apiexternal_v2.DownloadDownloading
	}

	return apiexternal_v2.DownloadQueued
}

// DownloadStates implements apiexternal_v2.DownloadStateLister.
func (p *Provider) DownloadStates(ctx context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	if err := p.login(ctx); err != nil {
		return nil, err
	}

	var resp struct {
		Torrents map[string]struct {
			Name            string   `json:"name"`
			Status          []string `json:"status"`
			PercentComplete float64  `json:"percentComplete"`
			Message         string   `json:"message"`
		} `json:"torrents"`
	}

	if err := p.MakeRequest(ctx, http.MethodGet, "/torrents", nil, &resp, nil); err != nil {
		return nil, err
	}

	out := make(map[string]apiexternal_v2.DownloadState, len(resp.Torrents))
	for hash, t := range resp.Torrents {
		out[apiexternal_v2.NormalizeDownloadID(hash)] = apiexternal_v2.DownloadState{
			Name: t.Name, State: downloadState(t.Status, t.PercentComplete), Progress: t.PercentComplete, Message: t.Message,
		}
	}

	return out, nil
}

// RemoveDownload implements apiexternal_v2.DownloadRemover.
func (p *Provider) RemoveDownload(ctx context.Context, id string, deleteData bool) error {
	if err := p.login(ctx); err != nil {
		return err
	}

	return p.post(ctx, "/torrents/delete", map[string]any{"hashes": []string{id}, "deleteData": deleteData})
}
