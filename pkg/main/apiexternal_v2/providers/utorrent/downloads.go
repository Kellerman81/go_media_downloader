package utorrent

import (
	"context"
	"net/http"
	"net/url"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

// Status flags of a uTorrent list row.
const (
	statusStarted = 1
	statusError   = 16
)

// DownloadStates implements apiexternal_v2.DownloadStateLister. A list row
// is [hash, status, name, size, progress in per mille, ..., 21: status text].
func (p *Provider) DownloadStates(ctx context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	token, err := p.token(ctx)
	if err != nil {
		return nil, err
	}

	var list struct {
		Torrents [][]any `json:"torrents"`
	}

	err = p.MakeRequestWithHeaders(ctx, http.MethodGet, "/?"+url.Values{"list": {"1"}, "token": {token}}.Encode(),
		nil, &list, nil, anyAccepted)
	if err != nil {
		return nil, err
	}

	out := make(map[string]apiexternal_v2.DownloadState, len(list.Torrents))

	for _, row := range list.Torrents {
		if len(row) < 5 {
			continue
		}

		hash, _ := row[0].(string)
		status, _ := row[1].(float64)
		name, _ := row[2].(string)
		permille, _ := row[4].(float64)

		var message string
		if len(row) > 21 {
			message, _ = row[21].(string)
		}

		s := apiexternal_v2.DownloadState{
			Name:     name,
			Progress: permille / 10,
			State:    apiexternal_v2.ProgressState(permille/10, int(status)&statusStarted != 0),
		}
		if int(status)&statusError != 0 {
			s.State, s.Message = apiexternal_v2.DownloadFailed, message
		}

		out[apiexternal_v2.NormalizeDownloadID(hash)] = s
	}

	return out, nil
}

// RemoveDownload implements apiexternal_v2.DownloadRemover.
func (p *Provider) RemoveDownload(ctx context.Context, id string, deleteData bool) error {
	token, err := p.token(ctx)
	if err != nil {
		return err
	}

	action := "remove"
	if deleteData {
		action = "removedata"
	}

	return p.action(ctx, token, url.Values{"action": {action}, "hash": {id}})
}
