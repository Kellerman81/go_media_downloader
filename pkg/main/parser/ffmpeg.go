package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
)

//Original Source: https://github.com/stashapp/stash/blob/develop/pkg/ffmpeg/ffprobe.go

type ffProbeJSON struct {
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
	Streams []ffProbeStream `json:"streams"`
	Error   struct {
		Code   int    `json:"code"`
		String string `json:"string"`
	} `json:"error"`
}
type ffProbeStream struct {
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

var (
	ffprobepath string
	ffprobeargs = []string{
		"-loglevel", "fatal",
		"-print_format", "json",
		"-show_entries",
		"format=duration : stream=codec_name,codec_tag_string,codec_type,height,width : stream_tags=Language : error",
	}
)

func (s *ffProbeJSON) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	logger.Clear(&s.Streams)
	logger.ClearVar(s)
}

func getFFProbeFilename() string {
	if ffprobepath != "" {
		return ffprobepath
	}

	if runtime.GOOS == "windows" {
		ffprobepath = logger.PathJoin(config.SettingsGeneral.FfprobePath, "ffprobe.exe")
	} else {
		ffprobepath = logger.PathJoin(config.SettingsGeneral.FfprobePath, "ffprobe")
	}
	return ffprobepath
}

func GetImdbFilename() string {
	var str string
	if runtime.GOOS == "windows" {
		str = "init_imdb.exe"
	} else {
		str = "./init_imdb"
	}
	return str
}

func ExecCmd(com string, file *string, typ string, out *bytes.Buffer, err *bytes.Buffer) error {
	var args []string
	switch typ {
	case "imdb":
		break
	case "ffprobe":
		args = append(ffprobeargs, *file)
	case "mediainfo":
		args = []string{"--Output=JSON", *file}
	}
	cmd := exec.Command(com, args...)
	logger.Clear(&args)
	cmd.Stdout = out
	cmd.Stderr = err
	defer logger.ClearVar(cmd)
	return cmd.Run()
}

func probeURL(m *apiexternal.ParseInfo, file *string, qualityTemplate string) error {
	// Add the file argument
	//ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer cancel()
	//cmd := exec.CommandContext(ctx, getFFProbeFilename(), append(ffprobeargs, *file)...)
	var outputBuf, stdErr bytes.Buffer
	err := ExecCmd(getFFProbeFilename(), file, "ffprobe", &outputBuf, &stdErr)
	defer outputBuf.Reset()
	defer stdErr.Reset()
	// cmd := exec.Command(getFFProbeFilename(), append(ffprobeargs, *file)...)

	// cmd.Stdout = &outputBuf
	// cmd.Stderr = &stdErr
	// err := cmd.Run()
	if err != nil {
		return errors.New("error running FFProbe [" + stdErr.String() + "] " + err.Error() + " [" + outputBuf.String() + "]")
	}
	//cancel()

	if stdErr.Len() > 0 {
		return errors.New("ffprobe error: " + stdErr.String())
	}
	var result ffProbeJSON
	err = json.NewDecoder(&outputBuf).Decode(&result)
	if err != nil {
		return errors.New("error parsing ffprobe output: " + err.Error())
	}

	if len(result.Streams) == 0 {
		result.Close()
		return errors.New("no tracks")
	}

	if result.Error.Code != 0 {
		defer result.Close()
		return errors.New("ffprobe error code " + logger.IntToString(result.Error.Code) + " " + result.Error.String)
	}
	duration, err := strconv.ParseFloat(result.Format.Duration, 64)
	if err == nil {
		m.Runtime = int(math.Round(duration))
	}

	//logger.Grow(&m.Languages, len(result.Streams))
	var redetermineprio bool
	var getreso string
	for idxstream := range result.Streams {
		if strings.EqualFold(result.Streams[idxstream].CodecType, "audio") {
			if result.Streams[idxstream].Tags.Language != "" {
				m.Languages = append(m.Languages, result.Streams[idxstream].Tags.Language)
			}
			if m.Audio == "" || (!strings.EqualFold(result.Streams[idxstream].CodecName, m.Audio) && result.Streams[idxstream].CodecName != "") {
				m.Audio = result.Streams[idxstream].CodecName
				m.AudioID = gettypeids(logger.DisableParserStringMatch, m.Audio, &database.DBConnect.GetaudiosIn)
				redetermineprio = true
			}
			continue
		}
		if !strings.EqualFold(result.Streams[idxstream].CodecType, "video") {
			continue
		}
		if result.Streams[idxstream].Height > result.Streams[idxstream].Width {
			result.Streams[idxstream].Height, result.Streams[idxstream].Width = result.Streams[idxstream].Width, result.Streams[idxstream].Height
		}

		if strings.EqualFold(result.Streams[idxstream].CodecName, "mpeg4") && strings.EqualFold(result.Streams[idxstream].CodecTagString, "xvid") {
			result.Streams[idxstream].CodecName = result.Streams[idxstream].CodecTagString
		}
		if m.Codec == "" || (!strings.EqualFold(result.Streams[idxstream].CodecName, m.Codec) && result.Streams[idxstream].CodecName != "") {
			m.Codec = result.Streams[idxstream].CodecName
			m.CodecID = gettypeids(logger.DisableParserStringMatch, m.Codec, &database.DBConnect.GetcodecsIn)
			redetermineprio = true
		}
		getreso = parseresolution(result.Streams[idxstream].Height, result.Streams[idxstream].Width)
		m.Height = result.Streams[idxstream].Height
		m.Width = result.Streams[idxstream].Width
		if getreso != "" && (m.Resolution == "" || !strings.EqualFold(getreso, m.Resolution)) {
			m.Resolution = getreso
			m.ResolutionID = gettypeids(logger.DisableParserStringMatch, m.Resolution, &database.DBConnect.GetresolutionsIn)
			redetermineprio = true
		}
	}
	if redetermineprio {
		//allQualityPrioritiesMu.Lock()
		intid := -1
		for idxi := range allQualityPrioritiesWantedT {
			if strings.EqualFold(allQualityPrioritiesWantedT[idxi].QualityGroup, config.SettingsQuality["quality_"+qualityTemplate].Name) && allQualityPrioritiesWantedT[idxi].ResolutionID == m.ResolutionID && allQualityPrioritiesWantedT[idxi].QualityID == m.QualityID && allQualityPrioritiesWantedT[idxi].CodecID == m.CodecID && allQualityPrioritiesWantedT[idxi].AudioID == m.AudioID {
				intid = idxi
				break
			}
		}
		//intid := logger.IndexFunc(&allQualityPrioritiesWantedT, func(e Prioarr) bool {
		//	return strings.EqualFold(e.QualityGroup, cfgname) && e.ResolutionID == m.ResolutionID && e.QualityID == m.QualityID && e.CodecID == m.CodecID && e.AudioID == m.AudioID
		//})
		if intid != -1 {
			m.Priority = allQualityPrioritiesWantedT[intid].Priority
		}
	}
	return nil
}

func parseresolution(height int, width int) string {

	var getreso string
	if height == 360 {
		getreso = "360p"
	} else if height > 1080 {
		getreso = "2160p"
	} else if height > 720 {
		getreso = "1080p"
	} else if height > 576 {
		getreso = "720p"
	} else if height > 480 {
		getreso = "576p"
	} else if height > 368 {
		getreso = "480p"
	} else if height > 360 {
		getreso = "368p"
	}
	if width == 720 {
		getreso = "480p"
		if height >= 576 {
			getreso = "576p"
		}
	}
	if width == 1280 {
		getreso = "720p"
	}
	if width == 1920 {
		getreso = "1080p"
	}
	if width == 3840 {
		getreso = "2160p"
	}
	return getreso
}
