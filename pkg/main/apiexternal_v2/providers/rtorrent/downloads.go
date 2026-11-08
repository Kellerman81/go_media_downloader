package rtorrent

import (
	"context"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

// str returns the value as a string, typed or not.
func (v *xmlValue) str() string {
	switch {
	case v.String != "":
		return v.String
	case v.I8 != "":
		return v.I8
	case v.I4 != "":
		return v.I4
	case v.Int != 0:
		return strconv.Itoa(v.Int)
	}

	return strings.TrimSpace(v.Text)
}

func (v *xmlValue) num() int64 {
	n, _ := strconv.ParseInt(v.str(), 10, 64)
	return n
}

// DownloadStates implements apiexternal_v2.DownloadStateLister. rTorrent has
// no failed state; its message (often a tracker warning) is passed on.
func (p *Provider) DownloadStates(ctx context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	resp, err := p.callXMLRPC(ctx, "d.multicall2", []any{
		"", "main", "d.hash=", "d.name=", "d.complete=", "d.bytes_done=", "d.size_bytes=", "d.message=", "d.is_active=",
	})
	if err != nil {
		return nil, err
	}

	out := make(map[string]apiexternal_v2.DownloadState)
	if resp.Params == nil || len(resp.Params.Params) == 0 || resp.Params.Params[0].Value.Array == nil {
		return out, nil
	}

	for _, row := range resp.Params.Params[0].Value.Array.Data {
		if row.Array == nil || len(row.Array.Data) < 7 {
			continue
		}

		f := row.Array.Data

		var progress float64
		if size := f[4].num(); size > 0 {
			progress = float64(f[3].num()) / float64(size) * 100
		}

		state := apiexternal_v2.ProgressState(progress, f[6].num() == 1)
		if f[2].num() == 1 {
			state, progress = apiexternal_v2.DownloadCompleted, 100
		}

		out[apiexternal_v2.NormalizeDownloadID(f[0].str())] = apiexternal_v2.DownloadState{
			Name: f[1].str(), State: state, Progress: progress, Message: f[5].str(),
		}
	}

	return out, nil
}

// RemoveDownload implements apiexternal_v2.DownloadRemover. rTorrent cannot
// delete data over XML-RPC, so the data stays.
func (p *Provider) RemoveDownload(ctx context.Context, id string, _ bool) error {
	return p.RemoveTorrent(ctx, strings.ToUpper(id), false)
}
