package parser

import (
	"errors"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/goccy/go-json"
)

//Original Source: https://github.com/stashapp/stash/blob/develop/pkg/ffmpeg/ffprobe.go

type ffProbeJSON struct {
	Format  ffProbeJSONFormat `json:"format"`
	Streams []ffProbeStream   `json:"streams"`
	Error   ffProbeJSONError  `json:"error"`
}
type ffProbeJSONError struct {
	Code   int    `json:"code"`
	String string `json:"string"`
}

// ffProbeJSONFormat contains metadata about the media format
type ffProbeJSONFormat struct {
	//BitRate is a string containing the bit rate of the media
	//BitRate        string `json:"bit_rate"`

	// Duration is a string containing the duration of the media
	Duration string `json:"duration"`
}

//	type ffProbeJSONFormat struct {
//		//BitRate        string `json:"bit_rate"`
//		Duration string `json:"duration"`
//		//Filename       string `json:"filename"`
//		//FormatLongName string `json:"format_long_name"`
//		//FormatName     string `json:"format_name"`
//		//NbPrograms     int    `json:"nb_programs"`
//		//NbStreams      int    `json:"nb_streams"`
//		//ProbeScore     int    `json:"probe_score"`
//		//Size           string `json:"size"`
//		//StartTime      string `json:"start_time"`
//		//Tags struct {
//		//CompatibleBrands string   `json:"compatible_brands"`
//		//CreationTime     JSONTime `json:"creation_time"`
//		//Encoder      string `json:"encoder"`
//		//MajorBrand   string `json:"major_brand"`
//		//MinorVersion string `json:"minor_version"`
//		//Title string `json:"title"`
//		//Comment string `json:"comment"`
//		//} `json:"tags"`
//	}
//
// ffProbeStream contains metadata about a media stream
type ffProbeStream struct {
	// CodecName is a string containing the codec name
	CodecName string `json:"codec_name"`

	// CodecTagString is a string containing the codec tag string
	CodecTagString string `json:"codec_tag_string"`

	// CodecType is a string containing the codec type
	CodecType string `json:"codec_type"`

	// Height is an int containing the height
	Height int `json:"height,omitempty"`

	Tags  ffProbeStreamTags `json:"tags"`
	Width int               `json:"width,omitempty"`
}

//		type ffProbeStream struct {
//		//AvgFrameRate string `json:"avg_frame_rate"`
//		//BitRate string `json:"bit_rate"`
//		//BitsPerRawSample   string `json:"bits_per_raw_sample,omitempty"`
//		//ChromaLocation     string `json:"chroma_location,omitempty"`
//		//CodecLongName      string `json:"codec_long_name"`
//		CodecName string `json:"codec_name"`
//		//CodecTag           string `json:"codec_tag"`
//		CodecTagString string `json:"codec_tag_string"`
//		//CodecTimeBase      string `json:"codec_time_base"`
//		CodecType string `json:"codec_type"`
//		//CodedHeight        int    `json:"coded_height,omitempty"`
//		//CodedWidth         int    `json:"coded_width,omitempty"`
//		//DisplayAspectRatio string `json:"display_aspect_ratio,omitempty"`
//		// Disposition        struct {
//		// 	AttachedPic     int `json:"attached_pic"`
//		// 	CleanEffects    int `json:"clean_effects"`
//		// 	Comment         int `json:"comment"`
//		// 	Default         int `json:"default"`
//		// 	Dub             int `json:"dub"`
//		// 	Forced          int `json:"forced"`
//		// 	HearingImpaired int `json:"hearing_impaired"`
//		// 	Karaoke         int `json:"karaoke"`
//		// 	Lyrics          int `json:"lyrics"`
//		// 	Original        int `json:"original"`
//		// 	TimedThumbnails int `json:"timed_thumbnails"`
//		// 	VisualImpaired  int `json:"visual_impaired"`
//		// } `json:"disposition"`
//		//Duration   string `json:"duration"`
//		//DurationTs int    `json:"duration_ts"`
//		//HasBFrames        int    `json:"has_b_frames,omitempty"`
//		Height int `json:"height,omitempty"`
//		//Index  int `json:"index"`
//		//IsAvc             string `json:"is_avc,omitempty"`
//		//Level             int    `json:"level,omitempty"`
//		//NalLengthSize     string `json:"nal_length_size,omitempty"`
//		//NbFrames          string `json:"nb_frames"`
//		//PixFmt            string `json:"pix_fmt,omitempty"`
//		//Profile    string `json:"profile"`
//		//RFrameRate string `json:"r_frame_rate"`
//		//Refs              int    `json:"refs,omitempty"`
//		//SampleAspectRatio string `json:"sample_aspect_ratio,omitempty"`
//		//StartPts          int    `json:"start_pts"`
//		//StartTime         string `json:"start_time"`
//		Tags ffProbeStreamTags `json:"tags"`
//		//TimeBase      string `json:"time_base"`
//		Width int `json:"width,omitempty"`
//		//BitsPerSample int    `json:"bits_per_sample,omitempty"`
//		//ChannelLayout string `json:"channel_layout,omitempty"`
//		//Channels      int    `json:"channels,omitempty"`
//		//MaxBitRate    string `json:"max_bit_rate,omitempty"`
//		//SampleFmt     string `json:"sample_fmt,omitempty"`
//		//SampleRate    string `json:"sample_rate,omitempty"`
//	}
//
// ffProbeStreamTags contains metadata tags about a media stream
type ffProbeStreamTags struct {
	// Language is a string containing the language
	Language string `json:"language"`
}

var plffprobe = pool.NewPool(100, 0, func(b *ffProbeJSON) {}, func(b *ffProbeJSON) {
	clear(b.Streams)
	*b = ffProbeJSON{}
})

// getFFProbeFilename returns the path to the ffprobe executable.
// It checks if the path has already been set, otherwise determines the path based on OS.
func getFFProbeFilename() string {
	if ffprobepath != "" {
		return ffprobepath
	}

	if runtime.GOOS == "windows" {
		ffprobepath = filepath.Join(config.SettingsGeneral.FfprobePath, "ffprobe.exe")
	} else {
		ffprobepath = filepath.Join(config.SettingsGeneral.FfprobePath, "ffprobe")
	}
	return ffprobepath
}

// Cmdout contains the stdout, stderr, and error from running a command
type Cmdout struct {
	// Out contains the stdout bytes from running the command
	Out []byte
	// Outerror contains the stderr as a string from running the command
	Outerror string
	// Err contains any error that occurred while running the command
	Err error
}

func (c *Cmdout) Close() {
	if c == nil {
		return
	}
	clear(c.Out)
	*c = Cmdout{}
}

// ExecCmd executes the given command with the provided arguments and returns
// the stdout, stderr, and error. typ indicates the command type, either
// "ffprobe" or "mediainfo". file is the path to the file to analyze.
func ExecCmd(com string, file string, typ string) Cmdout {
	var args []string
	switch typ {
	case "ffprobe":
		if file == "" {
			return Cmdout{Err: logger.ErrNotFound}
		}
		args = []string{"-loglevel", "fatal",
			"-print_format", "json",
			"-show_entries",
			"format=duration : stream=codec_name,codec_tag_string,codec_type,height,width : stream_tags=Language : error",
			file}
	case "mediainfo":
		if file == "" {
			return Cmdout{Err: logger.ErrNotFound}
		}
		args = []string{"--Output=JSON", file}
	}
	outputBuf := logger.PlBuffer.Get()
	stdErr := logger.PlBuffer.Get()
	cmd := exec.Command(com, args...)
	clear(args)
	cmd.Stdout = outputBuf
	cmd.Stderr = stdErr
	out := Cmdout{Err: cmd.Run()}
	if outputBuf.Len() >= 1 {
		out.Out = outputBuf.Bytes()
	}
	if stdErr.Len() >= 1 {
		out.Outerror = stdErr.String()
	}

	logger.PlBuffer.Put(outputBuf)
	logger.PlBuffer.Put(stdErr)
	return out
}

// probeURL probes the provided file with ffprobe and parses the output to populate metadata fields on the FileParser struct.
// It executes ffprobe, parses the JSON output, extracts information like duration, codecs, resolution etc.
// It will re-determine the priority if certain key metadata fields change compared to what was already set.
func probeURL(m *apiexternal.FileParser, file string, qualcfg *config.QualityConfig) error {
	if file == "" {
		return logger.ErrNotFound
	}

	//var outputBuf, stdErr bytes.Buffer
	out := ExecCmd(getFFProbeFilename(), file, "ffprobe")
	defer out.Close()
	if out.Err != nil {
		return fmt.Errorf("error running FFProbe [%s] %s [%s]", out.Outerror, out.Err.Error(), out.Out)
	}

	if out.Outerror != "" {
		return errors.New("ffprobe error: " + out.Outerror)
	}
	result := plffprobe.Get()
	defer plffprobe.Put(result)
	err := json.Unmarshal(out.Out, result)
	if err != nil {
		return err
	}

	if len(result.Streams) == 0 {
		return logger.ErrTracksEmpty
	}

	if result.Error.Code != 0 {
		return fmt.Errorf("ffprobe error code %d %s", result.Error.Code, result.Error.String)
	}
	duration, err := strconv.ParseFloat(result.Format.Duration, 64)
	if err == nil {
		m.M.Runtime = int(math.Round(duration))
	}

	var redetermineprio bool

	var n int
	for idx := range result.Streams {
		if result.Streams[idx].Tags.Language != "" && strings.EqualFold(result.Streams[idx].CodecType, "audio") {
			n++
		}
	}

	m.M.Languages = make([]string, 0, n)
	for _, stream := range result.Streams {
		if strings.EqualFold(stream.CodecType, "audio") {
			if stream.Tags.Language != "" {
				m.M.Languages = append(m.M.Languages, stream.Tags.Language)
			}
			if m.M.Audio == "" || (stream.CodecName != "" && !strings.EqualFold(stream.CodecName, m.M.Audio)) {
				m.M.Audio = stream.CodecName
				m.M.AudioID = gettypeids(m.M.Audio, database.DBConnect.GetaudiosIn)
				redetermineprio = true
			}
			continue
		}
		if !strings.EqualFold(stream.CodecType, "video") {
			continue
		}
		if stream.Height > stream.Width {
			stream.Height, stream.Width = stream.Width, stream.Height
		}

		if strings.EqualFold(stream.CodecName, "mpeg4") && strings.EqualFold(stream.CodecTagString, "xvid") {
			stream.CodecName = stream.CodecTagString
		}
		if m.M.Codec == "" || (stream.CodecName != "" && !strings.EqualFold(stream.CodecName, m.M.Codec)) {
			m.M.Codec = stream.CodecName
			m.M.CodecID = gettypeids(m.M.Codec, database.DBConnect.GetcodecsIn)
			redetermineprio = true
		}
		m.M.Height = stream.Height
		m.M.Width = stream.Width
		getreso := parseresolution(&m.M)

		if getreso != "" && (m.M.Resolution == "" || !strings.EqualFold(getreso, m.M.Resolution)) {
			m.M.Resolution = getreso
			m.M.ResolutionID = gettypeids(m.M.Resolution, database.DBConnect.GetresolutionsIn)
			redetermineprio = true
		}
	}

	if redetermineprio {
		intid := findpriorityidxwanted(m.M.ResolutionID, m.M.QualityID, m.M.CodecID, m.M.AudioID, qualcfg)
		if intid != -1 {
			m.M.Priority = allQualityPrioritiesWantedT[intid].Priority
		}
	}
	return nil
}

// parseresolution determines the video resolution string based on the height and width values in the ParseInfo struct.
// It handles common video resolutions like 360p, 480p, 720p, 1080p.
func parseresolution(m *database.ParseInfo) string {
	switch {
	case m.Height == 360:
		return "360p"
	case m.Height > 1080:
		return "2160p"
	case m.Height > 720:
		return "1080p"
	case m.Height > 576:
		return "720p"
	case m.Height > 480:
		return "576p"
	case m.Height > 368:
		return "480p"
	case m.Height > 360:
		return "368p"
	case m.Width == 720:
		if m.Height >= 576 {
			return "576p"
		}
		return "480p"
	case m.Width == 1280:
		return "720p"
	case m.Width == 1920:
		return "1080p"
	case m.Width == 3840:
		return "2160p"
	default:
		return "Unknown Resolution"
	}
}
