package parser

import (
	"errors"
	"math"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/goccy/go-json"
)

//Original Source: https://github.com/stashapp/stash/blob/develop/pkg/ffmpeg/ffprobe.go

type FFProbeJSON struct {
	Format  FFProbeJSONFormat `json:"format"`
	Streams []FFProbeStream   `json:"streams"`
	Error   FFProbeJSONError  `json:"error"`
}
type FFProbeJSONError struct {
	Code   int    `json:"code"`
	String string `json:"string"`
}

// ffProbeJSONFormat contains metadata about the media format
type FFProbeJSONFormat struct {
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
type FFProbeStream struct {
	// CodecName is a string containing the codec name
	CodecName string `json:"codec_name"`

	// CodecTagString is a string containing the codec tag string
	CodecTagString string `json:"codec_tag_string"`

	// CodecType is a string containing the codec type
	CodecType string `json:"codec_type"`

	// Height is an int containing the height
	Height int `json:"height,omitempty"`

	Tags  FFProbeStreamTags `json:"tags"`
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
type FFProbeStreamTags struct {
	// Language is a string containing the language
	Language string `json:"language"`
}

func (t *FFProbeJSON) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || t == nil {
		return
	}
	clear(t.Streams)
	t.Streams = nil
	*t = FFProbeJSON{}
}

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
	// if !filepath.IsAbs(ffprobepath) {
	// 	ffprobepath, _ = filepath.Abs(ffprobepath)
	// }
	return ffprobepath
}

// ExecCmd executes the given command with the provided arguments and returns
// the stdout, stderr, and error. typ indicates the command type, either
// "ffprobe" or "mediainfo". file is the path to the file to analyze.
func ExecCmd(com string, file string, typ string) ([]byte, string, error) {
	var params []string
	switch typ {
	case "ffprobe":
		if file == "" {
			return nil, "", logger.ErrNotFound
		}
		params = []string{"-loglevel", "fatal",
			"-print_format", "json",
			"-show_entries",
			"format=duration : stream=codec_name,codec_tag_string,codec_type,height,width : stream_tags=Language : error",
			file}
	case "mediainfo":
		if file == "" {
			return nil, "", logger.ErrNotFound
		}
		params = []string{"--Output=JSON", file}
	}
	outputBuf := logger.PlBuffer.Get()
	stdErr := logger.PlBuffer.Get()
	defer logger.PlBuffer.Put(outputBuf)
	defer logger.PlBuffer.Put(stdErr)
	cmd := exec.Command(com, params...)
	cmd.Stdout = outputBuf
	cmd.Stderr = stdErr
	err := cmd.Run()
	cmd = nil
	return outputBuf.Bytes(), stdErr.String(), err
}

// probeURL probes the provided file with ffprobe and parses the output to populate metadata fields on the FileParser struct.
// It executes ffprobe, parses the JSON output, extracts information like duration, codecs, resolution etc.
// It will re-determine the priority if certain key metadata fields change compared to what was already set.
func probeURL(m *database.ParseInfo, file string, qualcfg *config.QualityConfig) error {
	if file == "" {
		return logger.ErrNotFound
	}

	//var outputBuf, stdErr bytes.Buffer
	data, errstr, err := ExecCmd(getFFProbeFilename(), file, "ffprobe")
	if err != nil {
		return errors.New("error running FFProbe [" + errstr + "] " + err.Error() + " [" + logger.BytesToString2(data) + "]")
	}

	if errstr != "" {
		return errors.New("ffprobe error: " + errstr)
	}
	var result FFProbeJSON

	err = json.Unmarshal(data, &result)
	if err != nil {
		return err
	}
	//clear(data)
	defer result.Close()

	if len(result.Streams) == 0 {
		return logger.ErrTracksEmpty
	}

	if result.Error.Code != 0 {
		return errors.New("ffprobe error code " + strconv.Itoa(result.Error.Code) + " " + result.Error.String)
	}
	duration, err := strconv.ParseFloat(result.Format.Duration, 64)
	if err == nil {
		m.Runtime = int(math.Round(duration))
	}

	var redetermineprio bool

	var n int
	for idx := range result.Streams {
		if result.Streams[idx].Tags.Language != "" && (result.Streams[idx].CodecType == "audio" || strings.EqualFold(result.Streams[idx].CodecType, "audio")) {
			n++
		}
	}

	if n > 0 {
		m.Languages = make([]string, 0, n)
	}
	for idx := range result.Streams {
		if result.Streams[idx].CodecType == "audio" || strings.EqualFold(result.Streams[idx].CodecType, "audio") {
			if result.Streams[idx].Tags.Language != "" {
				m.Languages = append(m.Languages, result.Streams[idx].Tags.Language)
			}
			if m.Audio == "" || (result.Streams[idx].CodecName != "" && !strings.EqualFold(result.Streams[idx].CodecName, m.Audio)) {
				m.Audio = result.Streams[idx].CodecName
				m.AudioID = database.Gettypeids(m.Audio, database.DBConnect.GetaudiosIn)
				redetermineprio = true
			}
			continue
		}
		if result.Streams[idx].CodecType != "video" {
			if !strings.EqualFold(result.Streams[idx].CodecType, "video") {
				continue
			}
		}
		m.Height = result.Streams[idx].Height
		m.Width = result.Streams[idx].Width

		if (result.Streams[idx].CodecName == "mpeg4" || strings.EqualFold(result.Streams[idx].CodecName, "mpeg4")) && (result.Streams[idx].CodecTagString == "xvid" || strings.EqualFold(result.Streams[idx].CodecTagString, "xvid")) {
			result.Streams[idx].CodecName = result.Streams[idx].CodecTagString
		}
		if m.Codec == "" || (result.Streams[idx].CodecName != "" && !strings.EqualFold(result.Streams[idx].CodecName, m.Codec)) {
			m.Codec = result.Streams[idx].CodecName
			m.CodecID = database.Gettypeids(m.Codec, database.DBConnect.GetcodecsIn)
			redetermineprio = true
		}
		if m.Height > m.Width {
			m.Height, m.Width = m.Width, m.Height
		}
		getreso := m.Parseresolution()

		if getreso != "" && (m.Resolution == "" || !strings.EqualFold(getreso, m.Resolution)) {
			m.Resolution = getreso
			m.ResolutionID = database.Gettypeids(m.Resolution, database.DBConnect.GetresolutionsIn)
			redetermineprio = true
		}
	}
	if redetermineprio {
		intid := Findpriorityidxwanted(m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, qualcfg)
		if intid != -1 {
			m.Priority = allQualityPrioritiesWantedT[intid].Priority
		}
	}
	return nil
}
