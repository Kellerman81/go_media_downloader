package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
)

//Original Source: https://github.com/stashapp/stash/blob/develop/pkg/ffmpeg/ffprobe.go

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
		//Tags struct {
		//CompatibleBrands string   `json:"compatible_brands"`
		//CreationTime     JSONTime `json:"creation_time"`
		//Encoder      string `json:"encoder"`
		//MajorBrand   string `json:"major_brand"`
		//MinorVersion string `json:"minor_version"`
		//Title string `json:"title"`
		//Comment string `json:"comment"`
		//} `json:"tags"`
	} `json:"format"`
	Streams []FFProbeStream `json:"streams"`
	Error   struct {
		Code   int    `json:"code"`
		String string `json:"string"`
	} `json:"error"`
}

func (s *FFProbeJSON) Close() {
	if s != nil {
		s.Streams = nil
		s = nil
	}
}

type FFProbeStream struct {
	//AvgFrameRate string `json:"avg_frame_rate"`
	//BitRate string `json:"bit_rate"`
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
		//Rotate   int    `json:"rotate"`
		//Rotate   string `json:"rotate"`
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

var ffprobepath string

func getFFProbeFilename() string {
	if ffprobepath == "" {
		ffprobepath = config.Cfg.General.FfprobePath

		if runtime.GOOS == "windows" {
			ffprobepath = path.Join(ffprobepath, "ffprobe.exe")
		} else {
			ffprobepath = path.Join(ffprobepath, "ffprobe")
		}
		return ffprobepath
	} else {
		return ffprobepath
	}
}

// runProbe takes the fully configured ffprobe command and executes it, returning the ffprobe data if everything went fine.
func runProbe(cmd *exec.Cmd) (*FFProbeJSON, error) {
	var outputBuf bytes.Buffer
	var stdErr bytes.Buffer
	defer outputBuf.Reset()
	defer stdErr.Reset()

	cmd.Stdout = &outputBuf
	cmd.Stderr = &stdErr

	err := cmd.Run()
	if err != nil {
		cmd = nil
		return nil, errors.New(logger.StringBuild("error running FFProbe [", stdErr.String(), "] ", err.Error(), " [", outputBuf.String(), "]"))
	}

	if stdErr.Len() > 0 {
		cmd = nil
		return nil, errors.New("ffprobe error: " + stdErr.String())
	}

	data := new(FFProbeJSON)
	err = json.Unmarshal(outputBuf.Bytes(), data)
	if err != nil {
		cmd = nil
		data = nil
		return nil, errors.New("error parsing ffprobe output: " + err.Error())
	}
	cmd = nil
	return data, nil
}

var ffprobeargs []string

func getffprobeargs() interface{} {
	if len(ffprobeargs) == 0 {
		ffprobeargs = []string{
			"-loglevel", "fatal",
			"-print_format", "json",
			"-show_entries",
			"format=duration : stream=codec_name,codec_tag_string,codec_type,height,width : stream_tags=Language : error",
		}
		return ffprobeargs
	} else {
		return ffprobeargs
	}
}

func probeURL(ctx context.Context, fileURL string) (*FFProbeJSON, error) {
	// Add the file argument
	return runProbe(exec.CommandContext(ctx, getFFProbeFilename(), append(getffprobeargs().([]string), fileURL)...))
}

// Execute exec command and bind result to struct.
func newVideoFile(m *apiexternal.ParseInfo, ffprobePath string, videoPath string, stripExt bool, qualityTemplate string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := probeURL(ctx, videoPath)
	ctx.Done()
	if err != nil {
		return err
	}
	defer result.Close()

	if len(result.Streams) == 0 {
		return errors.New(logger.StringBuild("failed to get ffprobe json for <", videoPath, "> ", err.Error()))
	}

	if result.Error.Code != 0 {
		return errors.New(logger.StringBuild("ffprobe error code ", strconv.FormatInt(int64(result.Error.Code), 10), ": ", result.Error.String))
	}
	duration, err := strconv.ParseFloat(result.Format.Duration, 64)
	if err == nil {
		m.Runtime = int(math.Round(duration*100) / 100)
	}

	m.Languages = []string{}
	var getreso string
	redetermineprio := false
	for idxstream := range result.Streams {
		if result.Streams[idxstream].CodecType == "audio" {
			if result.Streams[idxstream].Tags.Language != "" {
				m.Languages = append(m.Languages, result.Streams[idxstream].Tags.Language)
			}
			if m.Audio == "" || (!strings.EqualFold(result.Streams[idxstream].CodecName, m.Audio) && result.Streams[idxstream].CodecName != "") {
				m.Audio = result.Streams[idxstream].CodecName
				m.AudioID = gettypeids(m, logger.DisableParserStringMatch, m.Audio, &database.DBConnect.GetaudiosIn)
				redetermineprio = true
			}
		}
		if result.Streams[idxstream].CodecType == "video" {
			if result.Streams[idxstream].Height > result.Streams[idxstream].Width {
				result.Streams[idxstream].Height, result.Streams[idxstream].Width = result.Streams[idxstream].Width, result.Streams[idxstream].Height
			}

			if strings.EqualFold(result.Streams[idxstream].CodecName, "mpeg4") && strings.EqualFold(result.Streams[idxstream].CodecTagString, "xvid") {
				result.Streams[idxstream].CodecName = result.Streams[idxstream].CodecTagString
			}
			if m.Codec == "" || (!strings.EqualFold(result.Streams[idxstream].CodecName, m.Codec) && result.Streams[idxstream].CodecName != "") {
				m.Codec = result.Streams[idxstream].CodecName
				m.CodecID = gettypeids(m, logger.DisableParserStringMatch, m.Codec, &database.DBConnect.GetcodecsIn)
				redetermineprio = true
			}
			getreso = ""
			if result.Streams[idxstream].Height == 360 {
				getreso = "360p"
			}
			if result.Streams[idxstream].Height > 360 {
				getreso = "368p"
			}
			if result.Streams[idxstream].Height > 368 || result.Streams[idxstream].Width == 720 {
				getreso = "480p"
			}
			if result.Streams[idxstream].Height > 480 {
				getreso = "576p"
			}
			if result.Streams[idxstream].Height > 576 || result.Streams[idxstream].Width == 1280 {
				getreso = "720p"
			}
			if result.Streams[idxstream].Height > 720 || result.Streams[idxstream].Width == 1920 {
				getreso = "1080p"
			}
			if result.Streams[idxstream].Height > 1080 || result.Streams[idxstream].Width == 3840 {
				getreso = "2160p"
			}
			m.Height = result.Streams[idxstream].Height
			m.Width = result.Streams[idxstream].Width
			if m.Resolution == "" || !strings.EqualFold(getreso, m.Resolution) {
				m.Resolution = getreso
				m.ResolutionID = gettypeids(m, logger.DisableParserStringMatch, m.Resolution, &database.DBConnect.GetresolutionsIn)
				redetermineprio = true
			}
		}
	}
	if redetermineprio {
		allQualityPrioritiesMu.RLock()
		defer allQualityPrioritiesMu.RUnlock()
		prio, ok := allQualityPriorities[qualityTemplate][strconv.Itoa(int(m.ResolutionID))+"_"+strconv.Itoa(int(m.QualityID))+"_"+strconv.Itoa(int(m.CodecID))+"_"+strconv.Itoa(int(m.AudioID))]
		if ok {
			m.Priority = prio
		}
	}
	return nil
}
