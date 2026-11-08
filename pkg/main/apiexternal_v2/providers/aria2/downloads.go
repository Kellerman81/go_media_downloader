package aria2

import (
	"context"
	"strconv"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

type status struct {
	GID             string   `json:"gid"`
	Status          string   `json:"status"` // active, waiting, paused, error, complete, removed
	TotalLength     string   `json:"totalLength"`
	CompletedLength string   `json:"completedLength"`
	ErrorMessage    string   `json:"errorMessage"`
	InfoHash        string   `json:"infoHash"`
	FollowedBy      []string `json:"followedBy"`
	Following       string   `json:"following"`
	Bittorrent      struct {
		Info struct {
			Name string `json:"name"`
		} `json:"info"`
	} `json:"bittorrent"`
}

var statusKeys = []string{
	"gid", "status", "totalLength", "completedLength", "errorMessage", "infoHash", "followedBy", "following", "bittorrent",
}

func (s *status) state() apiexternal_v2.DownloadState {
	total, _ := strconv.ParseFloat(s.TotalLength, 64)
	done, _ := strconv.ParseFloat(s.CompletedLength, 64)

	var progress float64
	if total > 0 {
		progress = done / total * 100
	}

	out := apiexternal_v2.DownloadState{Name: s.Bittorrent.Info.Name, Progress: progress}

	switch s.Status {
	case "error":
		out.State, out.Message = apiexternal_v2.DownloadFailed, s.ErrorMessage
	case "complete":
		out.State, out.Progress = apiexternal_v2.DownloadCompleted, 100
	case "active":
		out.State = apiexternal_v2.ProgressState(progress, true)
	default:
		out.State = apiexternal_v2.DownloadQueued
	}

	return out
}

// DownloadStates implements apiexternal_v2.DownloadStateLister, keyed by gid
// and info hash. A torrent URL's gid reports the torrent it was followed by.
func (p *Provider) DownloadStates(ctx context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	var all []status

	// Stopped first: a torrent URL's fetch finishes into the stopped list and
	// starts its torrent, which the later calls then see.
	for _, call := range []struct {
		method string
		params []any
	}{
		{"aria2.tellStopped", []any{0, 1000, statusKeys}},
		{"aria2.tellActive", []any{statusKeys}},
		{"aria2.tellWaiting", []any{0, 1000, statusKeys}},
	} {
		var list []status
		if err := p.call(ctx, call.method, &list, call.params...); err != nil {
			return nil, err
		}

		all = append(all, list...)
	}

	byGID := make(map[string]*status, len(all))
	for i := range all {
		byGID[all[i].GID] = &all[i]
	}

	out := make(map[string]apiexternal_v2.DownloadState, len(all))

	for i := range all {
		s := &all[i]
		if s.Status == "removed" {
			continue
		}

		st := s.state()
		if len(s.FollowedBy) > 0 {
			if f, ok := byGID[s.FollowedBy[0]]; ok {
				st = f.state()
			} else {
				// The fetched .torrent is done, its torrent not listed yet.
				st = apiexternal_v2.DownloadState{State: apiexternal_v2.DownloadQueued}
			}
		}

		out[apiexternal_v2.NormalizeDownloadID(s.GID)] = st

		if s.InfoHash != "" {
			out[apiexternal_v2.NormalizeDownloadID(s.InfoHash)] = st
		}

		// aria2 may already have dropped the fetch result of the torrent
		// URL whose gid was stored; its torrent still names it.
		if s.Following != "" {
			out[apiexternal_v2.NormalizeDownloadID(s.Following)] = st
		}
	}

	return out, nil
}

// RemoveDownload implements apiexternal_v2.DownloadRemover. aria2 cannot
// delete data, so deleteData is ignored.
func (p *Provider) RemoveDownload(ctx context.Context, id string, _ bool) error {
	var s status
	if err := p.call(ctx, "aria2.tellStatus", &s, id, statusKeys); err != nil {
		return err
	}

	gids := append([]string{id}, s.FollowedBy...)
	for _, gid := range gids {
		// Active downloads need forceRemove; finished ones only lose their result.
		_ = p.call(ctx, "aria2.forceRemove", nil, gid)
		_ = p.call(ctx, "aria2.removeDownloadResult", nil, gid)
	}

	return nil
}
