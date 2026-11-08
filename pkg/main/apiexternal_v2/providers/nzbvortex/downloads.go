package nzbvortex

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

type queueItem struct {
	ID         int     `json:"id"`
	UITitle    string  `json:"uiTitle"`
	State      int     `json:"state"`
	StatusText string  `json:"statusText"`
	Progress   float64 `json:"progress"`
	AddUUID    string  `json:"addUUID"`
}

// NZBVortex queue states (NzbVortexStateType in Sonarr).
const (
	stateDownloading      = 1
	stateDone             = 20
	stateUncompressFailed = 21
	stateBadlyEncoded     = 24
)

func (q *queueItem) state() apiexternal_v2.DownloadState {
	out := apiexternal_v2.DownloadState{Name: q.UITitle, Progress: q.Progress}

	switch {
	case q.State >= stateUncompressFailed && q.State <= stateBadlyEncoded:
		out.State, out.Message = apiexternal_v2.DownloadFailed, q.StatusText
	case q.State == stateDone:
		out.State, out.Progress = apiexternal_v2.DownloadCompleted, 100
	case q.State >= stateDownloading:
		out.State = apiexternal_v2.DownloadDownloading
	default:
		out.State = apiexternal_v2.DownloadQueued
	}

	return out
}

// queue returns the queue with the last 100 finished items.
func (p *Provider) queue(ctx context.Context) ([]queueItem, error) {
	var resp struct {
		result
		Items []queueItem `json:"nzbs"`
	}

	for _, renew := range []bool{false, true} {
		sid, err := p.session(ctx, renew)
		if err != nil {
			return nil, err
		}

		q := url.Values{"sessionid": {sid}, "limitDone": {"100"}}
		if err := p.MakeRequest(ctx, http.MethodGet, "/nzb?"+q.Encode(), nil, &resp, nil); err != nil {
			return nil, err
		}

		if !is(resp.Result, "notloggedin") {
			return resp.Items, nil
		}
	}

	return nil, errLoginFailed
}

// DownloadStates implements apiexternal_v2.DownloadStateLister, keyed by the
// add_uuid AddNZBURL returns.
func (p *Provider) DownloadStates(ctx context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	items, err := p.queue(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[string]apiexternal_v2.DownloadState, len(items))
	for i := range items {
		out[apiexternal_v2.NormalizeDownloadID(items[i].AddUUID)] = items[i].state()
	}

	return out, nil
}

var errNotQueued = errors.New("nzbvortex: no queue entry with this id")

// RemoveDownload implements apiexternal_v2.DownloadRemover for an add_uuid.
func (p *Provider) RemoveDownload(ctx context.Context, id string, deleteData bool) error {
	items, err := p.queue(ctx)
	if err != nil {
		return err
	}

	for _, it := range items {
		if apiexternal_v2.NormalizeDownloadID(it.AddUUID) != apiexternal_v2.NormalizeDownloadID(id) {
			continue
		}

		action := "/cancel"
		if deleteData {
			action = "/cancelDelete"
		}

		sid, err := p.session(ctx, false)
		if err != nil {
			return err
		}

		return p.MakeRequest(ctx, http.MethodGet, "/nzb/"+strconv.Itoa(it.ID)+action+"?sessionid="+url.QueryEscape(sid), nil, nil, nil)
	}

	return errNotQueued
}
