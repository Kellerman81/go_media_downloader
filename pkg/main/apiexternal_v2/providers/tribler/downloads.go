package tribler

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/goccy/go-json"
)

// downloadState maps a Tribler download status.
func downloadState(status string, progress float64) string {
	switch status {
	case "STOPPED_ON_ERROR":
		return apiexternal_v2.DownloadFailed
	case "SEEDING":
		return apiexternal_v2.DownloadCompleted
	case "DOWNLOADING":
		return apiexternal_v2.ProgressState(progress, true)
	}

	return apiexternal_v2.ProgressState(progress, false)
}

// DownloadStates implements apiexternal_v2.DownloadStateLister.
func (p *Provider) DownloadStates(ctx context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	var resp struct {
		Downloads []struct {
			Infohash string  `json:"infohash"`
			Name     string  `json:"name"`
			Status   string  `json:"status"`
			Progress float64 `json:"progress"` // 0-1
			Error    string  `json:"error"`
		} `json:"downloads"`
	}

	if err := p.MakeRequest(ctx, http.MethodGet, "/downloads", nil, &resp, nil); err != nil {
		return nil, err
	}

	out := make(map[string]apiexternal_v2.DownloadState, len(resp.Downloads))
	for _, d := range resp.Downloads {
		out[apiexternal_v2.NormalizeDownloadID(d.Infohash)] = apiexternal_v2.DownloadState{
			Name: d.Name, State: downloadState(d.Status, d.Progress*100), Progress: d.Progress * 100, Message: d.Error,
		}
	}

	return out, nil
}

// RemoveDownload implements apiexternal_v2.DownloadRemover.
func (p *Provider) RemoveDownload(ctx context.Context, id string, deleteData bool) error {
	body, err := json.Marshal(map[string]bool{"remove_data": deleteData})
	if err != nil {
		return err
	}

	return p.MakeRequest(ctx, http.MethodDelete, "/downloads/"+strings.ToLower(id), bytes.NewReader(body), nil, nil)
}
