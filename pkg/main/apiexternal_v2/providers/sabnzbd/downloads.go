package sabnzbd

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/goccy/go-json"
)

type sabHistoryResult struct {
	History struct {
		Slots []struct {
			NzoID       string `json:"nzo_id"`
			Name        string `json:"name"`
			Status      string `json:"status"`
			FailMessage string `json:"fail_message"`
		} `json:"slots"`
	} `json:"history"`
}

func (p *Provider) getJSON(ctx context.Context, params url.Values, target any) error {
	params.Set("output", "json")
	params.Set("apikey", p.apiKey)

	resp, err := p.makeRequest(ctx, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.handleHTTPError(resp)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

// historyState maps a SABnzbd history status. Anything not finished is
// post-processing (verifying, repairing, extracting, ...).
func historyState(status string) string {
	switch strings.ToLower(status) {
	case "completed":
		return apiexternal_v2.DownloadCompleted
	case "failed":
		return apiexternal_v2.DownloadFailed
	}

	return apiexternal_v2.DownloadDownloading
}

// DownloadStates implements apiexternal_v2.DownloadStateLister with the
// queue and the last 500 history entries, keyed by nzo_id.
func (p *Provider) DownloadStates(ctx context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	var queue sabQueueResult
	if err := p.getJSON(ctx, url.Values{"mode": {"queue"}, "limit": {"500"}}, &queue); err != nil {
		return nil, err
	}

	var history sabHistoryResult
	if err := p.getJSON(ctx, url.Values{"mode": {"history"}, "limit": {"500"}}, &history); err != nil {
		return nil, err
	}

	out := make(map[string]apiexternal_v2.DownloadState, len(queue.Queue.Slots)+len(history.History.Slots))

	for _, slot := range queue.Queue.Slots {
		t := p.convertSlotToTorrentInfo(slot)

		state := apiexternal_v2.DownloadQueued
		if strings.EqualFold(slot.Status, "downloading") {
			state = apiexternal_v2.DownloadDownloading
		}

		out[apiexternal_v2.NormalizeDownloadID(slot.NzoID)] = apiexternal_v2.DownloadState{
			Name: slot.Filename, State: state, Progress: t.Progress,
		}
	}

	for _, slot := range history.History.Slots {
		s := apiexternal_v2.DownloadState{Name: slot.Name, State: historyState(slot.Status), Message: slot.FailMessage}
		if s.State == apiexternal_v2.DownloadCompleted {
			s.Progress = 100
		}

		out[apiexternal_v2.NormalizeDownloadID(slot.NzoID)] = s
	}

	return out, nil
}

// RemoveDownload implements apiexternal_v2.DownloadRemover: the job is
// deleted from the queue and the history, so failed jobs go too.
func (p *Provider) RemoveDownload(ctx context.Context, id string, deleteData bool) error {
	var errs []error

	for _, mode := range []string{"queue", "history"} {
		var res sabBasicResult
		if err := p.getJSON(ctx, url.Values{
			"mode": {mode}, "name": {"delete"}, "value": {id},
			"del_files": {strconv.Itoa(boolInt(deleteData))},
		}, &res); err != nil {
			errs = append(errs, err)
			continue
		}

		if res.Status {
			return nil
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return errors.New("sabnzbd: job not found")
}

func boolInt(b bool) int {
	if b {
		return 1
	}

	return 0
}

// sabPaused is the SABnzbd priority that adds a job paused.
const sabPaused = -2

// AddNZBURL adds the NZB at nzbURL as name and returns its nzo_id. SABnzbd
// has no separate paused flag: a paused job is added with priority -2,
// which replaces the configured priority.
func (p *Provider) AddNZBURL(ctx context.Context, nzbURL, category, name string, priority int, paused bool) (string, error) {
	if paused {
		priority = sabPaused
	}

	return p.AddNZBNamed(ctx, nzbURL, category, name, priority)
}
