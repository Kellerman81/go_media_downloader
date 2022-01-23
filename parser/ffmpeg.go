package parser

import (
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
)

//Original Source: https://github.com/stashapp/stash/blob/develop/pkg/ffmpeg/ffprobe.go

// var currentLocation = time.Now().Location()

// type JSONTime struct {
// 	time.Time
// }

// func (jt *JSONTime) UnmarshalJSON(b []byte) error {
// 	s := strings.Trim(string(b), "\"")
// 	if s == "null" {
// 		jt.Time = time.Time{}
// 		return nil
// 	}

// 	var err error
// 	jt.Time, err = parseDateStringAsTime(s)
// 	if err != nil {
// 		logger.Log.Errorf("error unmarshalling JSONTime: %s", err.Error())
// 	}
// 	return nil
// }

// func (jt *JSONTime) MarshalJSON() ([]byte, error) {
// 	if jt.Time.IsZero() {
// 		return []byte("null"), nil
// 	}
// 	return []byte(fmt.Sprintf("\"%s\"", jt.Time.Format(time.RFC3339))), nil
// }

// func (jt *JSONTime) GetTime() time.Time {
// 	if currentLocation != nil {
// 		if jt.IsZero() {
// 			return time.Now().In(currentLocation)
// 		} else {
// 			return jt.Time.In(currentLocation)
// 		}
// 	} else {
// 		if jt.IsZero() {
// 			return time.Now()
// 		} else {
// 			return jt.Time
// 		}
// 	}
// }

// const railsTimeLayout = "2006-01-02 15:04:05 MST"

// func parseDateStringAsTime(dateString string) (time.Time, error) {
// 	// https://stackoverflow.com/a/20234207 WTF?

// 	t, e := time.Parse(time.RFC3339, dateString)
// 	if e == nil {
// 		return t, nil
// 	}

// 	t, e = time.Parse("2006-01-02", dateString)
// 	if e == nil {
// 		return t, nil
// 	}

// 	t, e = time.Parse("2006-01-02 15:04:05", dateString)
// 	if e == nil {
// 		return t, nil
// 	}

// 	t, e = time.Parse(railsTimeLayout, dateString)
// 	if e == nil {
// 		return t, nil
// 	}

// 	return time.Time{}, fmt.Errorf("ParseDateStringAsTime failed: dateString <%s>", dateString)
// }

type FFProbeJSON struct {
	Format struct {
		//BitRate        string `json:"bit_rate"`
		Duration string `json:"duration"`
		//Filename       string `json:"filename"`
		//FormatLongName string `json:"format_long_name"`
		//FormatName     string `json:"format_name"`
		//NbPrograms     int    `json:"nb_programs"`
		//NbStreams      int    `json:"nb_streams"`
		//ProbeScore     int    `json:"probe_score"`
		//Size           string `json:"size"`
		//StartTime      string `json:"start_time"`
		Tags struct {
			//CompatibleBrands string   `json:"compatible_brands"`
			//CreationTime     JSONTime `json:"creation_time"`
			//Encoder      string `json:"encoder"`
			//MajorBrand   string `json:"major_brand"`
			//MinorVersion string `json:"minor_version"`
			Title string `json:"title"`
			//Comment string `json:"comment"`
		} `json:"tags"`
	} `json:"format"`
	Streams []FFProbeStream `json:"streams"`
	Error   struct {
		Code   int    `json:"code"`
		String string `json:"string"`
	} `json:"error"`
}

type FFProbeStream struct {
	//AvgFrameRate string `json:"avg_frame_rate"`
	BitRate string `json:"bit_rate"`
	//BitsPerRawSample   string `json:"bits_per_raw_sample,omitempty"`
	//ChromaLocation     string `json:"chroma_location,omitempty"`
	//CodecLongName      string `json:"codec_long_name"`
	CodecName string `json:"codec_name"`
	//CodecTag           string `json:"codec_tag"`
	CodecTagString string `json:"codec_tag_string"`
	//CodecTimeBase      string `json:"codec_time_base"`
	CodecType string `json:"codec_type"`
	//CodedHeight        int    `json:"coded_height,omitempty"`
	//CodedWidth         int    `json:"coded_width,omitempty"`
	//DisplayAspectRatio string `json:"display_aspect_ratio,omitempty"`
	// Disposition        struct {
	// 	AttachedPic     int `json:"attached_pic"`
	// 	CleanEffects    int `json:"clean_effects"`
	// 	Comment         int `json:"comment"`
	// 	Default         int `json:"default"`
	// 	Dub             int `json:"dub"`
	// 	Forced          int `json:"forced"`
	// 	HearingImpaired int `json:"hearing_impaired"`
	// 	Karaoke         int `json:"karaoke"`
	// 	Lyrics          int `json:"lyrics"`
	// 	Original        int `json:"original"`
	// 	TimedThumbnails int `json:"timed_thumbnails"`
	// 	VisualImpaired  int `json:"visual_impaired"`
	// } `json:"disposition"`
	//Duration   string `json:"duration"`
	//DurationTs int    `json:"duration_ts"`
	//HasBFrames        int    `json:"has_b_frames,omitempty"`
	Height int `json:"height,omitempty"`
	//Index  int `json:"index"`
	//IsAvc             string `json:"is_avc,omitempty"`
	//Level             int    `json:"level,omitempty"`
	//NalLengthSize     string `json:"nal_length_size,omitempty"`
	//NbFrames          string `json:"nb_frames"`
	//PixFmt            string `json:"pix_fmt,omitempty"`
	//Profile    string `json:"profile"`
	//RFrameRate string `json:"r_frame_rate"`
	//Refs              int    `json:"refs,omitempty"`
	//SampleAspectRatio string `json:"sample_aspect_ratio,omitempty"`
	//StartPts          int    `json:"start_pts"`
	//StartTime         string `json:"start_time"`
	Tags struct {
		//CreationTime JSONTime `json:"creation_time"`
		//HandlerName  string   `json:"handler_name"`
		Language string `json:"language"`
		Rotate   string `json:"rotate"`
	} `json:"tags"`
	//TimeBase      string `json:"time_base"`
	Width int `json:"width,omitempty"`
	//BitsPerSample int    `json:"bits_per_sample,omitempty"`
	//ChannelLayout string `json:"channel_layout,omitempty"`
	//Channels      int    `json:"channels,omitempty"`
	//MaxBitRate    string `json:"max_bit_rate,omitempty"`
	//SampleFmt     string `json:"sample_fmt,omitempty"`
	//SampleRate    string `json:"sample_rate,omitempty"`
}

type VideoFile struct {
	JSON FFProbeJSON

	Path  string
	Title string
	//Comment   string
	//Container string
	Duration float64
	//StartTime float64
	//Bitrate   int64
	//Size      int64
	//CreationTime time.Time

	VideoCodec          string
	VideoCodecTagString string
	//VideoBitrate        int64
	Width  int
	Height int
	//FrameRate           float64
	//Rotation            int64

	AudioCodec     string
	AudioLanguages []string
}

func getFFProbeFilename() string {
	ffprobepath := ""
	if config.ConfigCheck("general") {
		cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

		ffprobepath = cfg_general.FfprobePath
	}

	if runtime.GOOS == "windows" {
		return path.Join(ffprobepath, "ffprobe.exe")
	}
	return path.Join(ffprobepath, "ffprobe")
}

// Execute exec command and bind result to struct.
func NewVideoFile(ffprobePath string, videoPath string, stripExt bool) (VideoFile, error) {
	args := []string{"-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", "-show_error", videoPath}

	out, err := exec.Command(ffprobePath, args...).Output()
	if err != nil {
		out = nil
		return VideoFile{}, fmt.Errorf("FFProbe encountered an error with <%s>.\nError JSON:\n%s\nError: %s", videoPath, string(out), err.Error())
	}
	probeJSON := FFProbeJSON{}
	if err := json.Unmarshal(out, &probeJSON); err != nil {
		out = nil
		return VideoFile{}, fmt.Errorf("error unmarshalling video data for <%s>: %s", videoPath, err.Error())
	}
	out = nil
	if len(probeJSON.Streams) == 0 {
		return VideoFile{}, fmt.Errorf("failed to get ffprobe json for <%s>", videoPath)
	}

	result := VideoFile{JSON: probeJSON}

	if result.JSON.Error.Code != 0 {
		return VideoFile{}, fmt.Errorf("ffprobe error code %d: %s", result.JSON.Error.Code, result.JSON.Error.String)
	}

	result.Path = videoPath
	result.Title = probeJSON.Format.Tags.Title

	if result.Title == "" {
		// default title to filename
		result.Title = result.setTitleFromPath(stripExt)
	}

	//result.Comment = probeJSON.Format.Tags.Comment

	//result.Bitrate, _ = strconv.ParseInt(probeJSON.Format.BitRate, 10, 64)
	//result.Container = probeJSON.Format.FormatName
	duration, _ := strconv.ParseFloat(probeJSON.Format.Duration, 64)
	result.Duration = math.Round(duration*100) / 100
	// fileStat, err := os.Stat(videoPath)
	// if err != nil {
	// 	logger.Log.Errorf("Error statting file <%s>: %s", videoPath, err.Error())
	// 	return VideoFile{}, err
	// }
	//result.Size = fileStat.Size()
	//fileStat = nil
	//result.StartTime, _ = strconv.ParseFloat(probeJSON.Format.StartTime, 64)
	//result.CreationTime = probeJSON.Format.Tags.CreationTime.Time

	audioStream, audioStreamindex := result.getAudioStream()
	if audioStreamindex != -1 {
		result.AudioCodec = audioStream.CodecName
	}
	for idxstream := range result.JSON.Streams {
		if result.JSON.Streams[idxstream].CodecType == "audio" {
			if result.JSON.Streams[idxstream].Tags.Language != "" {
				result.AudioLanguages = append(result.AudioLanguages, result.JSON.Streams[idxstream].Tags.Language)
			}
		}
	}

	videoStream, videoStreamindex := result.getVideoStream()
	if videoStreamindex != -1 {
		result.VideoCodec = videoStream.CodecName
		result.VideoCodecTagString = videoStream.CodecTagString
		//result.VideoBitrate, _ = strconv.ParseInt(videoStream.BitRate, 10, 64)
		//var framerate float64
		//if strings.Contains(videoStream.AvgFrameRate, "/") {
		//frameRateSplit := strings.Split(videoStream.AvgFrameRate, "/")
		//numerator, _ := strconv.ParseFloat(frameRateSplit[0], 64)
		//denominator, _ := strconv.ParseFloat(frameRateSplit[1], 64)
		//framerate = numerator / denominator
		//} else {
		//framerate, _ = strconv.ParseFloat(videoStream.AvgFrameRate, 64)
		//}
		//result.FrameRate = math.Round(framerate*100) / 100
		if rotate, err := strconv.ParseInt(videoStream.Tags.Rotate, 10, 64); err == nil && rotate != 180 {
			result.Width = videoStream.Height
			result.Height = videoStream.Width
		} else {
			result.Width = videoStream.Width
			result.Height = videoStream.Height
		}
	}
	result.JSON.Streams = nil
	return result, nil
}

func (v *VideoFile) getAudioStream() (FFProbeStream, int) {
	index := v.getStreamIndex("audio", v.JSON)
	if index != -1 {
		return v.JSON.Streams[index], index
	}
	return FFProbeStream{}, index
}

func (v *VideoFile) getVideoStream() (FFProbeStream, int) {
	index := v.getStreamIndex("video", v.JSON)
	if index != -1 {
		return v.JSON.Streams[index], index
	}
	return FFProbeStream{}, index
}

func (v *VideoFile) getStreamIndex(fileType string, probeJSON FFProbeJSON) int {
	for i, stream := range probeJSON.Streams {
		if stream.CodecType == fileType {
			return i
		}
	}
	return -1
}

func (v *VideoFile) setTitleFromPath(stripExtension bool) string {
	v.Title = filepath.Base(v.Path)
	if stripExtension {
		ext := filepath.Ext(v.Title)
		v.Title = strings.TrimSuffix(v.Title, ext)
	}
	return v.Title
}
