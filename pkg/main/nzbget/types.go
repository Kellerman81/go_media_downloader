package nzbget

import "time"

//Source: https://github.com/dashotv/flame

type Status struct {
	RemainingSizeMB     int  // 0
	ForcedSizeMB        int  // 0
	DownloadedSizeMB    int  // 0
	MonthSizeMB         int  // 0
	DaySizeMB           int  // 0
	ArticleCacheMB      int  // 0
	DownloadRate        int  // 0
	AverageDownloadRate int  // 0
	DownloadLimit       int  // 0
	UpTimeSec           int  // 2281
	DownloadTimeSec     int  // 0
	ServerPaused        bool // false
	DownloadPaused      bool // false
	Download2Paused     bool // false
	ServerStandBy       bool // true
	PostPaused          bool // false
	ScanPaused          bool // false
	QuotaReached        bool // false
	FreeDiskSpaceMB     int  // 134539
	ServerTime          int  // 1586063906
	ResumeTime          int  // 0
	FeedActive          bool // false
	QueueScriptCount    int  // 0
	NewsServers         []newsServer
	//RemainingSizeLo     int  // 0
	//RemainingSizeHi     int  // 0
	//ForcedSizeLo        int  // 0
	//ForcedSizeHi        int  // 0
	//DownloadedSizeLo    int  // 0
	//DownloadedSizeHi    int  // 0
	//MonthSizeLo         int  // 0
	//MonthSizeHi         int  // 0
	//DaySizeLo           int  // 0
	//DaySizeHi           int  // 0
	//ArticleCacheLo      int  // 0
	//ArticleCacheHi      int  // 0
	//ThreadCount         int  // 7
	//ParJobCount         int  // 0
	//PostJobCount        int  // 0
	//UrlCount            int  // 0
	//FreeDiskSpaceLo     int  // 3635539968
	//FreeDiskSpaceHi     int  // 32
}

type newsServer struct {
	ID     int
	Active bool
}

type statusResponse struct {
	*response
	Result *Status
}

type versionResponse struct {
	*response
	Version string `json:"result"`
}

type History struct {
	ID                 int `json:"nzbid"`
	Name               string
	RemainingFileCount int
	RetryData          bool
	HistoryTime        int
	Status             string
	Log                []string
	NZBName            string
	NZBNicename        string
	Kind               string
	URL                string
	NZBFilename        string
	DestDir            string
	FinalDir           string
	Category           string
	ParStatus          string
	ExParStatus        string
	UnpackStatus       string
	MoveStatus         string
	ScriptStatus       string
	DeleteStatus       string
	MarkStatus         string
	URLStatus          string
	FileSizeLo         int
	FileSizeHi         int
	FileSizeMB         int
	FileCount          int
	MinPostTime        int
	MaxPostTime        int
	TotalArticles      int
	SuccesArticles     int
	FailedArticles     int
	Health             int
	CriticalHealth     int
	DupeKey            string
	DupeScore          int
	DupeMode           string
	Deleted            bool
	DownloadedSizeLo   int
	DownloadedSizeHi   int
	DownloadedSizeMB   int
	DownloadTimeSec    int
	PostTotalTimeSec   int
	ParTimeSec         int
	RepairTimeSec      int
	UnpackTimeSec      int
	MessageCount       int
	ExtraParBlocks     int
	Parameters         []parameter
	ScriptStatuses     []scriptStatus
	ServerStats        []serverStat
}

type parameter struct {
	Name  string
	Value string
}

type serverStat struct {
	ServerID        int
	SuccessArticles int
	FailedArticles  int
}

type historyResponse struct {
	*response
	Result []History `json:"Result"`
}

type Group struct {
	ID                 int    `json:"nzbid"` // 4
	RemainingSizeMB    int    // 3497
	PausedSizeMB       int    // 3497
	RemainingFileCount int    // 73
	RemainingParCount  int    // 9
	MinPriority        int    // 0
	MaxPriority        int    // 0
	ActiveDownloads    int    // 0
	Status             string // PAUSED
	NZBName            string // Brave.Are.the.Fallen.2020.1080p.AMZN.WEB-DL.DDP2.0.H.264-ExREN,
	NZBNicename        string // Brave.Are.the.Fallen.2020.1080p.AMZN.WEB-DL.DDP2.0.H.264-ExREN,
	Kind               string // NZB
	URL                string // ,
	NZBFilename        string // Brave.Are.the.Fallen.2020.1080p.AMZN.WEB-DL.DDP2.0.H.264-ExREN,
	DestDir            string // /data/intermediate/Brave.Are.the.Fallen.2020.1080p.AMZN.WEB-DL.DDP2.0.H.264-ExREN.#4,
	FinalDir           string // ,
	Category           string // ,
	ParStatus          string // NONE
	ExParStatus        string // NONE
	UnpackStatus       string // NONE
	MoveStatus         string // NONE
	ScriptStatus       string // NONE
	DeleteStatus       string // NONE
	MarkStatus         string // NONE
	URLStatus          string // NONE
	FileSizeMB         int    // 3651
	FileCount          int    // 77
	MinPostTime        int    // 1586073677
	MaxPostTime        int    // 1586073793
	TotalArticles      int    // 4992
	SuccessArticles    int    // 212
	FailedArticles     int    // 0
	Health             int    // 1000
	CriticalHealth     int    // 898
	DupeKey            string //
	DupeScore          int    // 0
	DupeMode           string // SCORE
	Deleted            bool
	DownloadedSizeMB   int // 235
	DownloadTimeSec    int // 44
	PostTotalTimeSec   int // 0
	ParTimeSec         int // 0
	RepairTimeSec      int // 0
	UnpackTimeSec      int // 0
	MessageCount       int // 95
	ExtraParBlocks     int // 0
	Parameters         []parameter
	ScriptStatuses     []scriptStatus
	ServerStats        []serverStat
	PostInfoText       string // NONE
	PostStageProgress  int    // 9193728
	PostStageTimeSec   int    // 0
	Log                []log
	//FirstID            int    // 4
	//LastID             int    // 4
	//RemainingSizeLo    int    // 3666882216
	//RemainingSizeHi    int    // 0
	//PausedSizeLo       int    // 3666882216
	//PausedSizeHi       int    // 0
	//FileSizeLo         int    // 3829038352
	//FileSizeHi         int    // 0
	//DownloadedSizeLo   int // 247289836
	//DownloadedSizeHi   int // 0
}

type scriptStatus struct {
	Name   string
	Status string
}

type log struct {
}

type GroupResponse struct {
	*response
	Result []Group
}

type Client struct {
	URL string
	rpc RPCClient
}

type AppendOptions struct {
	NiceName   string
	Category   string
	Priority   int
	AddToTop   bool
	AddPaused  bool
	DupeKey    string
	DupeScore  int
	DupeMode   string
	Parameters []struct {
		Name  string
		Value string
	}
}

type response struct {
	APIVersion string `json:"version"`
	Error      string
	Status     *Status
	Timestamp  time.Time
}
