package parser

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/goccy/go-json"
)

// Original Source: https://github.com/stashapp/stash/blob/develop/pkg/ffmpeg/ffprobe.go

type ffProbeJSON struct {
	Streams []struct {
		Tags struct {
			Language string `json:"language"`
		} `json:"tags"`
		CodecName      string `json:"codec_name"`
		CodecTagString string `json:"codec_tag_string"`
		CodecType      string `json:"codec_type"`
		Height         int    `json:"height,omitempty"`
		Width          int    `json:"width,omitempty"`
	} `json:"streams"`
	Format struct {
		// BitRate is a string containing the bit rate of the media
		// BitRate        string `json:"bit_rate"`

		// Duration is a string containing the duration of the media
		Duration string `json:"duration"`
	} `json:"format"`
	Error struct {
		String string `json:"string"`
		Code   int    `json:"code"`
	} `json:"error"`
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

// getFFProbeFilename returns the path to the ffprobe executable.
// It checks if the path has already been set, otherwise determines the path based on OS.
func getFFProbeFilename() string {
	if ffprobepath != "" {
		return ffprobepath
	}

	executable := "ffprobe"
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	ffprobepath = filepath.Join(config.SettingsGeneral.FfprobePath, executable)
	return ffprobepath
}

func buildFFProbeCmd(file string) *exec.Cmd {
	return exec.Command(
		getFFProbeFilename(),
		"-loglevel", "fatal",
		"-print_format", "json",
		"-show_entries", "format=duration : stream=codec_name,codec_tag_string,codec_type,height,width : stream_tags=Language : error",
		file,
	)
}

func buildMediaInfoCmd(file string) *exec.Cmd {
	return exec.Command(getmediainfoFilename(), "--Output=JSON", file)
}

// getcmd returns an *exec.Cmd for the specified file and command type.
// If the file is empty, it returns nil.
// For "ffprobe" type, it returns a command with the ffprobe executable, loglevel set to "fatal",
// print format set to "json", and specific entries to show.
// For "mediainfo" type, it returns a command with the mediainfo executable and "--Output=JSON" option.
// For any other type, it returns a command with the getImdbFilename() executable.
func getcmd(file, typ string) *exec.Cmd {
	switch typ {
	case "ffprobe":
		if file == "" {
			return nil
		}
		return buildFFProbeCmd(file)
	case "mediainfo":
		if file == "" {
			return nil
		}
		return buildMediaInfoCmd(file)
	default:
		return exec.Command(getImdbFilename())
	}
}

// ExecCmdJson executes the given command with the provided arguments and returns
// the parsed JSON output as either a mediaInfoJSON or ffProbeJSON struct, or an error.
// The command type is specified by the 'typ' parameter, which can be either "ffprobe" or "mediainfo".
// The 'file' parameter is the path to the file to analyze.
// The 'm' and 'quality' parameters are used to pass additional context to the parsemediainfo and parseffprobe functions.
func ExecCmdJSON[T mediaInfoJSON | ffProbeJSON](
	file, typ string,
	m *database.ParseInfo,
	quality *config.QualityConfig,
) error {
	cmd := getcmd(file, typ)
	if cmd == nil {
		return logger.ErrNotFound
	}

	outputBuf := logger.PlBuffer.Get()
	stdErr := logger.PlBuffer.Get()
	defer func() {
		logger.PlBuffer.Put(outputBuf)
		logger.PlBuffer.Put(stdErr)
	}()

	cmd.Stdout = outputBuf
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return errors.New(
			"error running cmd [" + stdErr.String() + "] " + err.Error() + " [" + outputBuf.String() + "]",
		)
	}

	if stdErr.Len() > 0 {
		return errors.New("cmd error: " + stdErr.String())
	}
	var result T
	if err := json.Unmarshal(outputBuf.Bytes(), &result); err != nil {
		return err
	}
	switch v := any(&result).(type) {
	case *mediaInfoJSON:
		return parsemediainfo(m, quality, v)
	case *ffProbeJSON:
		return parseffprobe(m, quality, v)
	}
	return nil
}

// ExecCmdString executes the given command with the provided arguments and returns
// the stdout as a string, and any error that occurred. The command type is specified
// by the 'typ' parameter, which can be either "ffprobe" or "mediainfo". The 'file'
// parameter is the path to the file to analyze.
func ExecCmdString[t []byte | mediaInfoJSON | ffProbeJSON](file, typ string) (string, error) {
	cmd := getcmd(file, typ)
	if cmd == nil {
		return "", logger.ErrNotFound
	}

	outputBuf := logger.PlBuffer.Get()
	stdErr := logger.PlBuffer.Get()
	defer func() {
		logger.PlBuffer.Put(outputBuf)
		logger.PlBuffer.Put(stdErr)
	}()

	cmd.Stdout = outputBuf
	cmd.Stderr = stdErr
	err := cmd.Run()
	return outputBuf.String(), err
}
