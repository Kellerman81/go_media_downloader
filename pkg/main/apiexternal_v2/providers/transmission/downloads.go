package transmission

import (
	"context"
	"errors"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

// errLocal is Transmission's error kind for a local (disk/data) error.
// Tracker warnings and errors (1, 2) leave the download running.
const errLocal = 3

// downloadState maps a Transmission status (0 stopped ... 6 seeding).
func downloadState(status, errKind int, progress float64) string {
	switch {
	case errKind == errLocal:
		return apiexternal_v2.DownloadFailed
	case progress >= 100 || status == 5 || status == 6:
		return apiexternal_v2.DownloadCompleted
	case status == 4:
		return apiexternal_v2.DownloadDownloading
	}

	return apiexternal_v2.DownloadQueued
}

// DownloadStates implements apiexternal_v2.DownloadStateLister.
func (p *Provider) DownloadStates(ctx context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	response, err := p.makeRPCCall(ctx, "torrent-get", map[string]any{
		"fields": []string{"hashString", "name", "status", "percentDone", "error", "errorString"},
	})
	if err != nil {
		return nil, err
	}

	if response.Result != "success" {
		return nil, errors.New("transmission: " + response.Result)
	}

	args, _ := response.Arguments.(map[string]any)
	list, _ := args["torrents"].([]any)

	out := make(map[string]apiexternal_v2.DownloadState, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		hash, _ := m["hashString"].(string)
		name, _ := m["name"].(string)
		status, _ := m["status"].(float64)
		percent, _ := m["percentDone"].(float64)
		errKind, _ := m["error"].(float64)
		errText, _ := m["errorString"].(string)

		out[apiexternal_v2.NormalizeDownloadID(hash)] = apiexternal_v2.DownloadState{
			Name:     name,
			State:    downloadState(int(status), int(errKind), percent*100),
			Progress: percent * 100,
			Message:  errText,
		}
	}

	return out, nil
}

// RemoveDownload implements apiexternal_v2.DownloadRemover.
func (p *Provider) RemoveDownload(ctx context.Context, id string, deleteData bool) error {
	return p.RemoveTorrent(ctx, strings.ToLower(id), deleteData)
}
