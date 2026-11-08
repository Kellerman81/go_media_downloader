package downloadstation

import (
	"context"
	"errors"
	"net/url"
	"strconv"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/goccy/go-json"
)

// v2Status are the DownloadStation2 task statuses, which come as numbers.
var v2Status = []string{
	"unknown", "waiting", "downloading", "paused", "finishing", "finished",
	"hash_checking", "seeding", "filehosting_waiting", "extracting", "error",
}

// downloadState maps a task status name.
func downloadState(status string) string {
	switch status {
	case "error":
		return apiexternal_v2.DownloadFailed
	case "finished", "seeding":
		return apiexternal_v2.DownloadCompleted
	case "downloading", "finishing", "extracting":
		return apiexternal_v2.DownloadDownloading
	}

	return apiexternal_v2.DownloadQueued
}

type task struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Size        int64             `json:"size"`
	Status      any               `json:"status"` // string in v1, number in v2
	StatusExtra map[string]string `json:"status_extra"`
	Additional  struct {
		Transfer map[string]any `json:"transfer"`
	} `json:"additional"`
}

func (t *task) state() apiexternal_v2.DownloadState {
	var status string

	switch v := t.Status.(type) {
	case string:
		status = v
	case float64:
		if i := int(v); i >= 0 && i < len(v2Status) {
			status = v2Status[i]
		}
	}

	out := apiexternal_v2.DownloadState{Name: t.Title, State: downloadState(status)}

	if done, ok := t.Additional.Transfer["size_downloaded"].(float64); ok && t.Size > 0 {
		out.Progress = done / float64(t.Size) * 100
	}

	if out.State == apiexternal_v2.DownloadCompleted {
		out.Progress = 100
	}

	if out.State == apiexternal_v2.DownloadFailed {
		out.Message = t.StatusExtra["error_detail"]
		if out.Message == "" {
			out.Message = "Download Station reports an error"
		}
	}

	return out
}

// taskAPI returns the task API, its path and version, preferring DSM 7's.
func taskAPI(apis map[string]apiInfo) (string, string, string, error) {
	if t, ok := apis[apiTask2]; ok {
		return apiTask2, t.Path, "2", nil
	}

	if t, ok := apis[apiTask1]; ok {
		return apiTask1, t.Path, "1", nil
	}

	return "", "", "", errNoTaskAPI
}

func (p *Provider) taskCall(ctx context.Context, q url.Values, out any) error {
	return p.taskCallRetry(ctx, q, out, true)
}

// taskCallRetry logs in again once when the session has expired.
func (p *Provider) taskCallRetry(ctx context.Context, q url.Values, out any, retry bool) error {
	apis, sid, err := p.session(ctx)
	if err != nil {
		return err
	}

	api, path, version, err := taskAPI(apis)
	if err != nil {
		return err
	}

	q.Set("api", api)
	q.Set("version", version)
	q.Set("_sid", sid)

	data, err := get[map[string]any](ctx, p, path, q)
	if retry && errors.Is(err, apiError(errNoAuth)) {
		p.dropSession()
		return p.taskCallRetry(ctx, q, out, false)
	}

	if err != nil || out == nil {
		return err
	}

	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, out)
}

// DownloadStates implements apiexternal_v2.DownloadStateLister, keyed by task
// id. The add call returns no id; it is found by comparing task lists.
func (p *Provider) DownloadStates(ctx context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	var data struct {
		Tasks []task `json:"tasks"` // v1
		Task  []task `json:"task"`  // v2
	}

	if err := p.taskCall(ctx, url.Values{"method": {"list"}, "additional": {"transfer"}}, &data); err != nil {
		return nil, err
	}

	out := make(map[string]apiexternal_v2.DownloadState, len(data.Tasks)+len(data.Task))
	for _, t := range append(data.Tasks, data.Task...) {
		out[apiexternal_v2.NormalizeDownloadID(t.ID)] = t.state()
	}

	return out, nil
}

// RemoveDownload implements apiexternal_v2.DownloadRemover. Download Station
// always deletes the incomplete data of a removed task.
func (p *Provider) RemoveDownload(ctx context.Context, id string, _ bool) error {
	return p.taskCall(ctx, url.Values{"method": {"delete"}, "id": {id}, "force_complete": {strconv.FormatBool(false)}}, nil)
}
