package nzbget

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/goccy/go-json"
)

type nzbGetHistoryItem struct {
	NZBID  int    `json:"NZBID"`
	Name   string `json:"Name"`
	Status string `json:"Status"` // "SUCCESS/ALL", "FAILURE/PAR", "DELETED/MANUAL", ...
}

// historyState maps an NZBGet history status. WARNING means finished with a
// script or health warning; DELETED counts as failed, as in Sonarr.
func historyState(status string) string {
	kind, _, _ := strings.Cut(status, "/")
	switch kind {
	case "SUCCESS", "WARNING":
		return apiexternal_v2.DownloadCompleted
	case "FAILURE", "DELETED":
		return apiexternal_v2.DownloadFailed
	}

	return apiexternal_v2.DownloadDownloading
}

// queueState maps an NZBGet queue status; everything after the transfer
// (PP_QUEUED, REPAIRING, UNPACKING, ...) still counts as downloading.
func queueState(status string) string {
	switch status {
	case "QUEUED", "PAUSED":
		return apiexternal_v2.DownloadQueued
	}

	return apiexternal_v2.DownloadDownloading
}

// DownloadStates implements apiexternal_v2.DownloadStateLister with the queue
// and the history, keyed by NZBID.
func (p *Provider) DownloadStates(ctx context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	result, err := p.makeJSONRPCCall(ctx, "listgroups", []any{})
	if err != nil {
		return nil, err
	}

	var queue []nzbGetQueueItem
	if err := json.Unmarshal(result, &queue); err != nil {
		return nil, err
	}

	result, err = p.makeJSONRPCCall(ctx, "history", []any{false})
	if err != nil {
		return nil, err
	}

	var history []nzbGetHistoryItem
	if err := json.Unmarshal(result, &history); err != nil {
		return nil, err
	}

	out := make(map[string]apiexternal_v2.DownloadState, len(queue)+len(history))

	for _, h := range history {
		s := apiexternal_v2.DownloadState{Name: h.Name, State: historyState(h.Status)}
		if s.State == apiexternal_v2.DownloadFailed {
			s.Message = "NZBGet: " + h.Status
		} else {
			s.Progress = 100
		}

		out[strconv.Itoa(h.NZBID)] = s
	}

	for _, q := range queue {
		var progress float64
		if q.FileSizeMB > 0 {
			progress = float64(q.FileSizeMB-q.RemainingSizeMB) / float64(q.FileSizeMB) * 100
		}

		out[strconv.Itoa(q.NZBID)] = apiexternal_v2.DownloadState{
			Name: q.NZBName, State: queueState(q.Status), Progress: progress,
		}
	}

	return out, nil
}

// RemoveDownload implements apiexternal_v2.DownloadRemover for queued and
// finished (history) jobs.
func (p *Provider) RemoveDownload(ctx context.Context, id string, deleteData bool) error {
	nzbID, err := strconv.Atoi(id)
	if err != nil {
		return errors.New("nzbget: invalid NZBID " + id)
	}

	queueAction, historyAction := "GroupDelete", "HistoryDelete"
	if deleteData {
		queueAction, historyAction = "GroupFinalDelete", "HistoryFinalDelete"
	}

	queueErr := p.editQueue(ctx, []any{queueAction, 0, "", []int{nzbID}})
	if queueErr == nil {
		return nil
	}

	if err := p.editQueue(ctx, []any{historyAction, 0, "", []int{nzbID}}); err != nil {
		return errors.Join(queueErr, err)
	}

	return nil
}

// AddNZBURL adds the NZB at nzbURL as name and returns its NZBID.
func (p *Provider) AddNZBURL(ctx context.Context, nzbURL, category, name string, priority int, paused bool) (string, error) {
	id, err := p.AddNZBExtended(ctx, nzbURL, category, name, priority, paused)
	if err != nil || id <= 0 {
		return "", err
	}

	return strconv.Itoa(id), nil
}
