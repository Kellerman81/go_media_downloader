package qbittorrent

import (
	"context"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

// downloadState maps a qBittorrent torrent state.
func downloadState(state string, progress float64) (string, string) {
	switch state {
	case "error":
		return apiexternal_v2.DownloadFailed, "qBittorrent reports an error"
	case "missingFiles":
		return apiexternal_v2.DownloadFailed, "qBittorrent reports missing files"
	case "uploading", "stalledUP", "pausedUP", "stoppedUP", "queuedUP", "forcedUP", "checkingUP":
		return apiexternal_v2.DownloadCompleted, ""
	case "downloading", "forcedDL", "metaDL", "forcedMetaDL", "stalledDL":
		return apiexternal_v2.DownloadDownloading, ""
	}

	return apiexternal_v2.ProgressState(progress, false), ""
}

// DownloadStates implements apiexternal_v2.DownloadStateLister.
func (p *Provider) DownloadStates(ctx context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	list, err := p.ListTorrents(ctx, "")
	if err != nil {
		return nil, err
	}

	out := make(map[string]apiexternal_v2.DownloadState, len(list.Torrents))
	for _, t := range list.Torrents {
		state, msg := downloadState(t.State, t.Progress)
		out[apiexternal_v2.NormalizeDownloadID(t.Hash)] = apiexternal_v2.DownloadState{
			Name: t.Name, State: state, Progress: t.Progress, Message: msg,
		}
	}

	return out, nil
}

// RemoveDownload implements apiexternal_v2.DownloadRemover.
func (p *Provider) RemoveDownload(ctx context.Context, id string, deleteData bool) error {
	return p.RemoveTorrent(ctx, strings.ToLower(id), deleteData)
}
