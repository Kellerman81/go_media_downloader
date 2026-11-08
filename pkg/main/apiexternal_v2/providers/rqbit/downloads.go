package rqbit

import (
	"context"
	"net/http"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

type torrentStats struct {
	State         string  `json:"state"` // initializing, live, paused, error
	Error         *string `json:"error"`
	Finished      bool    `json:"finished"`
	ProgressBytes int64   `json:"progress_bytes"`
	TotalBytes    int64   `json:"total_bytes"`
}

func (s *torrentStats) state() apiexternal_v2.DownloadState {
	var out apiexternal_v2.DownloadState
	if s.TotalBytes > 0 {
		out.Progress = float64(s.ProgressBytes) / float64(s.TotalBytes) * 100
	}

	switch {
	case s.State == "error":
		out.State = apiexternal_v2.DownloadFailed
		if s.Error != nil {
			out.Message = *s.Error
		}
	case s.Finished:
		out.State, out.Progress = apiexternal_v2.DownloadCompleted, 100
	default:
		out.State = apiexternal_v2.ProgressState(out.Progress, s.State == "live")
	}

	return out
}

// DownloadStates implements apiexternal_v2.DownloadStateLister.
func (p *Provider) DownloadStates(ctx context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	var resp struct {
		Torrents []struct {
			InfoHash string       `json:"info_hash"`
			Name     string       `json:"name"`
			Stats    torrentStats `json:"stats"`
		} `json:"torrents"`
	}

	if err := p.MakeRequest(ctx, http.MethodGet, "/torrents?with_stats=true", nil, &resp, nil); err != nil {
		return nil, err
	}

	out := make(map[string]apiexternal_v2.DownloadState, len(resp.Torrents))
	for _, t := range resp.Torrents {
		s := t.Stats.state()
		s.Name = t.Name
		out[apiexternal_v2.NormalizeDownloadID(t.InfoHash)] = s
	}

	return out, nil
}

// RemoveDownload implements apiexternal_v2.DownloadRemover: "delete" also
// removes the data, "forget" keeps it.
func (p *Provider) RemoveDownload(ctx context.Context, id string, deleteData bool) error {
	action := "/forget"
	if deleteData {
		action = "/delete"
	}

	return p.MakeRequest(ctx, http.MethodPost, "/torrents/"+id+action, nil, nil, nil)
}
