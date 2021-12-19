package sabnzbd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type BytesFromGB int

func (b *BytesFromGB) UnmarshalJSON(data []byte) error {
	var gb float64
	var gbStr string
	err := json.Unmarshal(data, &gb)
	if err != nil {
		err = json.Unmarshal(data, &gbStr)
		if err != nil {
			return err
		}
		gb, err = strconv.ParseFloat(gbStr, 32)
		if err != nil {
			return err
		}
	}
	*b = BytesFromGB(gb * float64(GByte))
	return nil
}

type BytesFromMB int

func (b *BytesFromMB) UnmarshalJSON(data []byte) error {
	var mb float64
	var mbStr string
	err := json.Unmarshal(data, &mb)
	if err != nil {
		err = json.Unmarshal(data, &mbStr)
		if err != nil {
			return err
		}
		mb, err = strconv.ParseFloat(mbStr, 32)
		if err != nil {
			return err
		}
	}
	*b = BytesFromMB(mb * float64(MByte))
	return nil
}

type BytesFromKB int

func (b *BytesFromKB) UnmarshalJSON(data []byte) error {
	var kb float64
	var kbStr string
	err := json.Unmarshal(data, &kb)
	if err != nil {
		err = json.Unmarshal(data, &kbStr)
		if err != nil {
			return err
		}
		kb, err = strconv.ParseFloat(kbStr, 32)
		if err != nil {
			return err
		}
	}
	*b = BytesFromKB(kb * float64(KByte))
	return nil
}

type BytesFromB int

func (b *BytesFromB) UnmarshalJSON(data []byte) error {
	var bytes float64
	var bytesStr string
	err := json.Unmarshal(data, &bytes)
	if err != nil {
		err = json.Unmarshal(data, &bytesStr)
		if err != nil {
			return err
		}
		bytes, err = strconv.ParseFloat(bytesStr, 32)
		if err != nil {
			return err
		}
	}
	*b = BytesFromB(bytes)
	return nil
}

type SabDuration time.Duration

func (d *SabDuration) UnmarshalJSON(data []byte) error {
	var str string
	err := json.Unmarshal(data, &str)
	if err != nil {
		return err
	}
	var h, m, s time.Duration
	if _, err = fmt.Sscanf(str, `%d:%2d:%2d`, &h, &m, &s); err != nil {
		return err
	}
	*d = SabDuration(h*time.Hour + m*time.Minute + s*time.Second)
	return nil
}

type apiError struct {
	ErrorMsg string `json:"error,omitempty"`
}

func (e apiError) Error() string {
	return e.ErrorMsg
}

type versionResponse struct {
	Version string `json:"version"`
}

type authResponse struct {
	Auth string `json:"auth"`
}

type SimpleQueueResponse struct {
	TimeLeft              SabDuration      `json:"timeleft"`
	Size                  BytesFromMB      `json:"mb"`
	NoOfSlots             int              `json:"noofslots"`
	Paused                bool             `json:"paused"`
	BytesLeft             BytesFromMB      `json:"mbleft"`
	DownloadDiskFreeSpace BytesFromGB      `json:"diskspace1"`
	CompleteDiskFreeSpace BytesFromGB      `json:"diskspace2"`
	BytesPerSec           BytesFromKB      `json:"kbpersec"`
	Jobs                  []SimpleQueueJob `json:"jobs"`
	apiError
}

type SimpleQueueJob struct {
	ID        string      `json:"id"`
	MsgID     string      `json:"msgid"`
	Filename  string      `json:"filename"`
	BytesLeft BytesFromMB `json:"mbleft"`
	Bytes     BytesFromMB `json:"mb"`
}

type AdvancedQueueResponse struct {
	CacheLimit             string              `json:"cache_limit"`
	Categories             []string            `json:"categories"`
	Scripts                []string            `json:"scripts"`
	Paused                 bool                `json:"paused"`
	NewRelURL              string              `json:"new_rel_url"`
	RestartRequested       bool                `json:"restart_req"`
	Slots                  []AdvancedQueueSlot `json:"slots"`
	HelpURI                string              `json:"helpuri"`
	Uptime                 string              `json:"uptime"`
	RefreshRate            string              `json:"refresh_rate"`
	IsVerbose              bool                `json:"isverbose"`
	Start                  int                 `json:"start"`
	Version                string              `json:"version"`
	DownloadDiskTotalSpace BytesFromGB         `json:"diskspacetotal1"`
	CompleteDiskTotalSpace BytesFromGB         `json:"diskspacetotal2"`
	ColorScheme            string              `json:"color_scheme"`
	Darwin                 bool                `json:"darwin"`
	NT                     bool                `json:"nt"`
	Status                 string              `json:"status"`
	LastWarning            string              `json:"last_warning"`
	HaveWarnings           string              `json:"have_warnings"`
	CacheArt               string              `json:"cache_art"`
	FinishAction           *string             `json:"finishaction"`
	NoOfSlots              int                 `json:"noofslots"`
	CacheSize              string              `json:"cache_size"`
	Finish                 int                 `json:"finish"`
	NewRelease             string              `json:"new_release"`
	PauseInt               string              `json:"pause_int"`
	Bytes                  BytesFromMB         `json:"mb"`
	BytesLeft              BytesFromMB         `json:"mbleft"`
	TimeLeft               SabDuration         `json:"timeleft"`
	ETA                    string              `json:"eta"`
	DownloadDiskFreeSpace  BytesFromGB         `json:"diskspace1"`
	CompleteDiskFreeSpace  BytesFromGB         `json:"diskspace2"`
	NZBQuota               string              `json:"nzb_quota"`
	LoadAverage            string              `json:"loadavg"`
	Limit                  int                 `json:"limit"`
	BytesPerSec            BytesFromKB         `json:"kbpersec"`
	SpeedLimit             string              `json:"speedlimit"`
	WebDir                 string              `json:"webdir"`
	QueueDetails           string              `json:"queue_details"`
	apiError
}

type advancedQueueResponse *AdvancedQueueResponse

func (r *AdvancedQueueResponse) UnmarshalJSON(data []byte) error {
	var queue struct {
		Queue json.RawMessage `json:"queue"`
	}
	if err := json.Unmarshal(data, &queue); err != nil {
		return err
	}
	err := json.Unmarshal(queue.Queue, advancedQueueResponse(r))
	return err
}

type AdvancedQueueSlot struct {
	Status     string      `json:"status"`
	Index      int         `json:"index"`
	ETA        string      `json:"eta"`
	TimeLeft   SabDuration `json:"timeleft"`
	AverageAge string      `json:"avg_age"`
	Script     string      `json:"script"`
	MsgID      string      `json:"msgid"`
	Verbosity  string      `json:"verbosity"`
	Bytes      BytesFromMB `json:"mb"`
	Filename   string      `json:"filename"`
	Priority   string      `json:"priority"`
	Category   string      `json:"cat"`
	BytesLeft  BytesFromMB `json:"mbleft"`
	Percentage string      `json:"percentage"`
	NzoID      string      `json:"nzo_id"`
	UnpackOpts string      `json:"unpackopts"`
	Size       string      `json:"size"`
}

type HistoryResponse struct {
	TotalSize              string        `json:"total_size"`
	MonthSize              string        `json:"month_size"`
	WeekSize               string        `json:"week_size"`
	CacheLimit             string        `json:"cache_limit"`
	Paused                 bool          `json:"paused"`
	NewRelURL              string        `json:"string"`
	RestartRequested       bool          `json:"restart_req"`
	Slots                  []HistorySlot `json:"slots"`
	HelpURI                string        `json:"helpuri"`
	Uptime                 string        `json:"uptime"`
	Version                string        `json:"version"`
	DownloadDiskTotalSpace BytesFromGB   `json:"diskspacetotal1"`
	CompleteDiskTotalSpace BytesFromGB   `json:"diskspacetotal2"`
	ColorScheme            string        `json:"color_scheme"`
	Darwin                 bool          `json:"darwin"`
	NT                     bool          `json:"nt"`
	Status                 string        `json:"status"`
	LastWarning            string        `json:"last_warning"`
	HaveWarnings           string        `json:"have_warnings"`
	CacheArt               string        `json:"cache_art"`
	FinishAction           *string       `json:"finishaction"`
	NoOfSlots              int           `json:"noofslots"`
	CacheSize              string        `json:"cache_size"`
	NewRelease             string        `json:"new_release"`
	PauseInt               string        `json:"pause_int"`
	Bytes                  BytesFromMB   `json:"mb"`
	BytesLeft              BytesFromMB   `json:"mbleft"`
	TimeLeft               SabDuration   `json:"timeleft"`
	ETA                    string        `json:"eta"`
	DownloadDiskFreeSpace  BytesFromGB   `json:"diskspace1"`
	CompleteDiskFreeSpace  BytesFromGB   `json:"diskspace2"`
	NZBQuota               string        `json:"nzb_quota"`
	LoadAverage            string        `json:"loadavg"`
	BytesPerSec            BytesFromKB   `json:"kbpersec"`
	SpeedLimit             string        `json:"speedlimit"`
	WebDir                 string        `json:"webdir"`
	apiError
}

type historyResponse *HistoryResponse

func (r *HistoryResponse) UnmarshalJSON(data []byte) error {
	var history struct {
		History json.RawMessage `json:"history"`
	}
	if err := json.Unmarshal(data, &history); err != nil {
		return err
	}
	err := json.Unmarshal(history.History, historyResponse(r))
	return err
}

type HistorySlot struct {
	ActionLine         string            `json:"action_line"`
	ShowDetails        string            `json:"show_details"`
	ScriptLog          string            `json:"script_log"`
	FailMessage        string            `json:"fail_message"`
	Loaded             bool              `json:"loaded"`
	ID                 int               `json:"id"`
	Size               string            `json:"size"`
	Category           string            `json:"category"`
	PP                 string            `json:"pp"`
	Completeness       int               `json:"completeness"`
	Script             string            `json:"script"`
	NZBName            string            `json:"nzb_name"`
	DownloadTime       int               `json:"download_time"` // change to time.Duration
	Storage            string            `json:"storage"`
	Status             string            `json:"status"`
	ScriptLine         string            `json:"script_line"`
	Completed          int               `json:"completed"` // change to time.Time
	NzoID              string            `json:"nzo_id"`
	Downloaded         int               `json:"downloaded"` // change to time.Time
	Report             string            `json:"report"`
	Path               string            `json:"path"`
	PostProcessingTime int               `json:"postproc_time"` // change to time.Duration
	Name               string            `json:"name"`
	URL                string            `json:"url"`
	Bytes              int               `json:"bytes"`
	URLInfo            string            `json:"url_info"`
	StageLogs          []HistoryStageLog `json:"stage_log"`
}

type HistoryStageLog struct {
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}

type warningsResponse struct {
	Warnings []string `json:"warnings"`
	apiError
}

type categoriesResponse struct {
	Categories []string `json:"categories"`
	apiError
}

type scriptsResponse struct {
	Scripts []string `json:"scripts"`
	apiError
}

type addFileResponse struct {
	NzoIDs []string `json:"nzo_ids"`
	apiError
}

type ItemFilesResponse struct {
	Files []ItemFile `json:"files"`
	apiError
}

type ItemFile struct {
	ID        string      `json:"id"`
	NzfID     string      `json:"nzf_id"`
	Status    string      `json:"status"`
	Filename  string      `json:"filename"`
	Age       string      `json:"age"`
	Bytes     BytesFromB  `json:"bytes"`
	BytesLeft BytesFromMB `json:"mbleft"`
}

type itemFile *ItemFile

func (f *ItemFile) UnmarshalJSON(data []byte) (err error) {
	err = json.Unmarshal(data, itemFile(f))
	if err != nil {
		return err
	}
	if int(f.BytesLeft) > int(f.Bytes) {
		f.BytesLeft = BytesFromMB(f.Bytes)
	}
	return nil
}
