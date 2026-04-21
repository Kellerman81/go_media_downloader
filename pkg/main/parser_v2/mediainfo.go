package parser_v2

import (
	"errors"
	"math"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/goccy/go-json"
)

// MediaAnalyzer provides FFmpeg and mediainfo integration for extracting
// technical metadata from media files.
type MediaAnalyzer struct {
	ffprobePath   string
	mediainfoPath string
	useMediainfo  bool
	useFallback   bool
}

// MediaAnalyzerConfig contains configuration for the MediaAnalyzer.
type MediaAnalyzerConfig struct {
	// FFprobePath is the path to the ffprobe executable.
	FFprobePath string
	// MediainfoPath is the path to the mediainfo executable.
	MediainfoPath string
	// UseMediainfo prefers mediainfo over ffprobe.
	UseMediainfo bool
	// UseFallback enables fallback to the other tool if primary fails.
	UseFallback bool
}

// MediaInfo contains technical information extracted from a media file.
type MediaInfo struct {
	// Duration in seconds.
	Duration float64 `json:"duration,omitempty"`
	// DurationMS in milliseconds.
	DurationMS int64 `json:"duration_ms,omitempty"`
	// Video stream information.
	Video *VideoStreamInfo `json:"video,omitempty"`
	// Audio streams information.
	Audio []AudioStreamInfo `json:"audio,omitempty"`
	// Container format.
	Format string `json:"format,omitempty"`
	// Overall bitrate in kbps.
	Bitrate int `json:"bitrate,omitempty"`
	// File size in bytes.
	FileSize int64 `json:"file_size,omitempty"`
}

// VideoStreamInfo contains video stream technical details.
type VideoStreamInfo struct {
	// Codec name (e.g., "h264", "hevc").
	Codec string `json:"codec,omitempty"`
	// CodecTag (e.g., "XVID").
	CodecTag string `json:"codec_tag,omitempty"`
	// Width in pixels.
	Width int `json:"width,omitempty"`
	// Height in pixels.
	Height int `json:"height,omitempty"`
	// Bitrate in kbps.
	Bitrate int `json:"bitrate,omitempty"`
	// FrameRate frames per second.
	FrameRate float64 `json:"frame_rate,omitempty"`
	// BitDepth bits per channel.
	BitDepth int `json:"bit_depth,omitempty"`
	// HDR type if applicable (HDR10, Dolby Vision, etc.).
	HDRType string `json:"hdr_type,omitempty"`
}

// AudioStreamInfo contains audio stream technical details.
type AudioStreamInfo struct {
	// Codec name (e.g., "aac", "ac3", "dts").
	Codec string `json:"codec,omitempty"`
	// Language code.
	Language string `json:"language,omitempty"`
	// Channels count.
	Channels int `json:"channels,omitempty"`
	// SampleRate in Hz.
	SampleRate int `json:"sample_rate,omitempty"`
	// Bitrate in kbps.
	Bitrate int `json:"bitrate,omitempty"`
	// BitDepth bits per sample.
	BitDepth int `json:"bit_depth,omitempty"`
	// IsDefault indicates if this is the default track.
	IsDefault bool `json:"is_default,omitempty"`
}

// ffProbeJSON matches the JSON output from ffprobe.
type ffProbeJSON struct {
	Streams []ffProbeStream `json:"streams"`
	Format  ffProbeFormat   `json:"format"`
	Error   *ffProbeError   `json:"error,omitempty"`
}

type ffProbeStream struct {
	Tags struct {
		Language string `json:"language"`
	} `json:"tags"`
	CodecName      string `json:"codec_name"`
	CodecTagString string `json:"codec_tag_string"`
	CodecType      string `json:"codec_type"`
	Height         int    `json:"height,omitempty"`
	Width          int    `json:"width,omitempty"`
	SampleRate     string `json:"sample_rate,omitempty"`
	Channels       int    `json:"channels,omitempty"`
	BitsPerSample  int    `json:"bits_per_sample,omitempty"`
	BitRate        string `json:"bit_rate,omitempty"`
	RFrameRate     string `json:"r_frame_rate,omitempty"`
	AvgFrameRate   string `json:"avg_frame_rate,omitempty"`
	ColorPrimaries string `json:"color_primaries,omitempty"`
	ColorTransfer  string `json:"color_transfer,omitempty"`
	ColorSpace     string `json:"color_space,omitempty"`
	Disposition    struct {
		Default int `json:"default"`
	} `json:"disposition"`
}

type ffProbeFormat struct {
	Duration string `json:"duration"`
	BitRate  string `json:"bit_rate,omitempty"`
	Size     string `json:"size,omitempty"`
}

type ffProbeError struct {
	Code   int    `json:"code"`
	String string `json:"string"`
}

// mediaInfoJSON matches the JSON output from mediainfo.
type mediaInfoJSON struct {
	Media struct {
		Track []mediaInfoTrack `json:"track"`
	} `json:"media"`
}

type mediaInfoTrack struct {
	Type           string `json:"@type"`
	Format         string `json:"Format,omitempty"`
	Duration       string `json:"Duration,omitempty"`
	CodecID        string `json:"CodecID,omitempty"`
	Width          string `json:"Width,omitempty"`
	Height         string `json:"Height,omitempty"`
	Language       string `json:"Language,omitempty"`
	Channels       string `json:"Channels,omitempty"`
	SamplingRate   string `json:"SamplingRate,omitempty"`
	BitRate        string `json:"BitRate,omitempty"`
	BitDepth       string `json:"BitDepth,omitempty"`
	FrameRate      string `json:"FrameRate,omitempty"`
	HDRFormat      string `json:"HDR_Format,omitempty"`
	FileSize       string `json:"FileSize,omitempty"`
	OverallBitRate string `json:"OverallBitRate,omitempty"`
	Default        string `json:"Default,omitempty"`
}

// NewMediaAnalyzer creates a new MediaAnalyzer with default configuration.
func NewMediaAnalyzer() *MediaAnalyzer {
	return &MediaAnalyzer{
		ffprobePath:   defaultFFprobePath(),
		mediainfoPath: defaultMediainfoPath(),
		useMediainfo:  false,
		useFallback:   true,
	}
}

// NewMediaAnalyzerWithConfig creates a new MediaAnalyzer with custom configuration.
func NewMediaAnalyzerWithConfig(cfg MediaAnalyzerConfig) *MediaAnalyzer {
	ma := &MediaAnalyzer{
		useMediainfo: cfg.UseMediainfo,
		useFallback:  cfg.UseFallback,
	}

	if cfg.FFprobePath != "" {
		ma.ffprobePath = cfg.FFprobePath
	} else {
		ma.ffprobePath = defaultFFprobePath()
	}

	if cfg.MediainfoPath != "" {
		ma.mediainfoPath = cfg.MediainfoPath
	} else {
		ma.mediainfoPath = defaultMediainfoPath()
	}

	return ma
}

// defaultFFprobePath returns the default ffprobe executable path.
func defaultFFprobePath() string {
	if runtime.GOOS == "windows" {
		return "ffprobe.exe"
	}

	return "ffprobe"
}

// defaultMediainfoPath returns the default mediainfo executable path.
func defaultMediainfoPath() string {
	if runtime.GOOS == "windows" {
		return "mediainfo.exe"
	}

	return "mediainfo"
}

// Analyze extracts technical metadata from a media file.
func (ma *MediaAnalyzer) Analyze(filePath string) (*MediaInfo, error) {
	if filePath == "" {
		return nil, errors.New("file path is empty")
	}

	var (
		info *MediaInfo
		err  error
	)

	if ma.useMediainfo {
		info, err = ma.analyzeWithMediainfo(filePath)
		if err == nil {
			return info, nil
		}

		if ma.useFallback {
			return ma.analyzeWithFFprobe(filePath)
		}

		return nil, err
	}

	info, err = ma.analyzeWithFFprobe(filePath)
	if err == nil {
		return info, nil
	}

	if ma.useFallback {
		return ma.analyzeWithMediainfo(filePath)
	}

	return nil, err
}

// AnalyzeVideo analyzes a video file and updates the VideoParseResult with technical info.
func (ma *MediaAnalyzer) AnalyzeVideo(filePath string, result *VideoParseResult) error {
	info, err := ma.Analyze(filePath)
	if err != nil {
		return err
	}

	// Update video result with analyzed info
	result.Runtime = int(math.Round(info.Duration))

	if info.Video != nil {
		result.Width = info.Video.Width
		result.Height = info.Video.Height

		// Normalize dimensions (height should be smaller)
		if result.Height > result.Width {
			result.Height, result.Width = result.Width, result.Height
		}

		// Update codec if not already set or if more accurate
		if result.Codec == "" && info.Video.Codec != "" {
			result.Codec = info.Video.Codec
		}

		// Set resolution based on height if not already set
		if result.Resolution == "" {
			result.Resolution = ma.heightToResolution(result.Height)
		}
	}

	// Update audio codec from first audio stream
	if len(info.Audio) > 0 && result.Audio == "" {
		result.Audio = info.Audio[0].Codec
	}

	return nil
}

// AnalyzeAudiobook analyzes audiobook files and updates the result with runtime info.
func (ma *MediaAnalyzer) AnalyzeAudiobook(files []string, result *AudiobookParseResult) error {
	var totalRuntime int64

	for i, filePath := range files {
		info, err := ma.Analyze(filePath)
		if err != nil {
			continue
		}

		totalRuntime += info.DurationMS

		// Update file info if we have matching entries
		if i < len(result.Files) {
			result.Files[i].RuntimeMS = info.DurationMS
		}

		// Get audio properties from first file
		if i != 0 || len(info.Audio) == 0 {
			continue
		}

		audio := info.Audio[0]

		result.Bitrate = audio.Bitrate
		result.SampleRate = audio.SampleRate
	}

	result.RuntimeMS = totalRuntime

	return nil
}

// AnalyzeMusic analyzes music files and updates the result with technical info.
func (ma *MediaAnalyzer) AnalyzeMusic(files []string, result *MusicParseResult) error {
	var totalRuntime int64

	for i, filePath := range files {
		info, err := ma.Analyze(filePath)
		if err != nil {
			continue
		}

		totalRuntime += info.DurationMS

		// Update track info if we have matching entries
		if i < len(result.Tracks) {
			result.Tracks[i].RuntimeMS = info.DurationMS
		}

		// Get audio properties from first file
		if i != 0 || len(info.Audio) == 0 {
			continue
		}

		audio := info.Audio[0]

		result.Bitrate = audio.Bitrate
		result.SampleRate = audio.SampleRate
		result.BitDepth = audio.BitDepth

		// Determine if lossless
		ext := strings.ToLower(filepath.Ext(filePath))

		result.IsLossless = IsLosslessAudioExtension(ext)
	}

	result.TotalRuntimeMS = totalRuntime

	return nil
}

// GetAudioRuntime returns the duration of an audio file in milliseconds.
func (ma *MediaAnalyzer) GetAudioRuntime(filePath string) (int64, error) {
	info, err := ma.Analyze(filePath)
	if err != nil {
		return 0, err
	}

	return info.DurationMS, nil
}

// analyzeWithFFprobe uses ffprobe to analyze the file.
func (ma *MediaAnalyzer) analyzeWithFFprobe(filePath string) (*MediaInfo, error) {
	cmd := exec.Command(
		ma.ffprobePath,
		"-loglevel",
		"fatal",
		"-print_format",
		"json",
		"-show_entries",
		"format=duration,bit_rate,size : stream=codec_name,codec_tag_string,codec_type,height,width,sample_rate,channels,bits_per_sample,bit_rate,r_frame_rate,avg_frame_rate,color_primaries,color_transfer,color_space : stream_tags=language : stream_disposition=default : error",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var ffprobe ffProbeJSON
	if err := json.Unmarshal(output, &ffprobe); err != nil {
		return nil, err
	}

	if ffprobe.Error != nil && ffprobe.Error.Code != 0 {
		return nil, errors.New("ffprobe error: " + ffprobe.Error.String)
	}

	return ma.parseFFprobeResult(&ffprobe), nil
}

// analyzeWithMediainfo uses mediainfo to analyze the file.
func (ma *MediaAnalyzer) analyzeWithMediainfo(filePath string) (*MediaInfo, error) {
	cmd := exec.Command(ma.mediainfoPath, "--Output=JSON", filePath)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var mediainfo mediaInfoJSON
	if err := json.Unmarshal(output, &mediainfo); err != nil {
		return nil, err
	}

	if len(mediainfo.Media.Track) == 0 {
		return nil, errors.New("no tracks found in media file")
	}

	return ma.parseMediainfoResult(&mediainfo), nil
}

// parseFFprobeResult converts ffprobe output to MediaInfo.
func (ma *MediaAnalyzer) parseFFprobeResult(ffprobe *ffProbeJSON) *MediaInfo {
	info := &MediaInfo{}

	// Parse format info
	if ffprobe.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(ffprobe.Format.Duration, 64); err == nil {
			info.Duration = duration
			info.DurationMS = int64(duration * 1000)
		}
	}

	if ffprobe.Format.BitRate != "" {
		if bitrate, err := strconv.Atoi(ffprobe.Format.BitRate); err == nil {
			info.Bitrate = bitrate / 1000 // Convert to kbps
		}
	}

	if ffprobe.Format.Size != "" {
		if size, err := strconv.ParseInt(ffprobe.Format.Size, 10, 64); err == nil {
			info.FileSize = size
		}
	}

	// Parse streams
	for i := range ffprobe.Streams {
		stream := &ffprobe.Streams[i]

		switch strings.ToLower(stream.CodecType) {
		case "video":
			if info.Video != nil {
				break
			}

			info.Video = &VideoStreamInfo{
				Codec:    stream.CodecName,
				CodecTag: stream.CodecTagString,
				Width:    stream.Width,
				Height:   stream.Height,
			}

			// Parse frame rate
			if stream.AvgFrameRate != "" {
				info.Video.FrameRate = parseFrameRate(stream.AvgFrameRate)
			} else if stream.RFrameRate != "" {
				info.Video.FrameRate = parseFrameRate(stream.RFrameRate)
			}

			// Parse bitrate
			if stream.BitRate != "" {
				if bitrate, err := strconv.Atoi(stream.BitRate); err == nil {
					info.Video.Bitrate = bitrate / 1000
				}
			}

			// Detect HDR
			info.Video.HDRType = ma.detectHDR(stream)

		case "audio":
			audio := AudioStreamInfo{
				Codec:     stream.CodecName,
				Language:  stream.Tags.Language,
				Channels:  stream.Channels,
				BitDepth:  stream.BitsPerSample,
				IsDefault: stream.Disposition.Default == 1,
			}

			if stream.SampleRate != "" {
				if sr, err := strconv.Atoi(stream.SampleRate); err == nil {
					audio.SampleRate = sr
				}
			}

			if stream.BitRate != "" {
				if bitrate, err := strconv.Atoi(stream.BitRate); err == nil {
					audio.Bitrate = bitrate / 1000
				}
			}

			info.Audio = append(info.Audio, audio)
		}
	}

	return info
}

// parseMediainfoResult converts mediainfo output to MediaInfo.
func (ma *MediaAnalyzer) parseMediainfoResult(mediainfo *mediaInfoJSON) *MediaInfo {
	info := &MediaInfo{}

	for i := range mediainfo.Media.Track {
		track := &mediainfo.Media.Track[i]

		switch track.Type {
		case "General":
			// Parse duration
			if track.Duration != "" {
				if duration, err := strconv.ParseFloat(track.Duration, 64); err == nil {
					info.Duration = duration
					info.DurationMS = int64(duration * 1000)
				}
			}

			// Parse overall bitrate
			if track.OverallBitRate != "" {
				if bitrate, err := strconv.Atoi(track.OverallBitRate); err == nil {
					info.Bitrate = bitrate / 1000
				}
			}

			// Parse file size
			if track.FileSize == "" {
				break
			}

			size, err := strconv.ParseInt(track.FileSize, 10, 64)
			if err == nil {
				info.FileSize = size
			}

		case "Video":
			if info.Video != nil {
				break
			}

			info.Video = &VideoStreamInfo{
				Codec:    track.Format,
				CodecTag: track.CodecID,
			}

			if track.Width != "" {
				info.Video.Width, _ = strconv.Atoi(strings.TrimSpace(track.Width))
			}

			if track.Height != "" {
				info.Video.Height, _ = strconv.Atoi(strings.TrimSpace(track.Height))
			}

			if track.FrameRate != "" {
				info.Video.FrameRate, _ = strconv.ParseFloat(track.FrameRate, 64)
			}

			if track.BitRate != "" {
				if bitrate, err := strconv.Atoi(track.BitRate); err == nil {
					info.Video.Bitrate = bitrate / 1000
				}
			}

			if track.BitDepth != "" {
				info.Video.BitDepth, _ = strconv.Atoi(track.BitDepth)
			}

			if track.HDRFormat != "" {
				info.Video.HDRType = track.HDRFormat
			}

		case "Audio":
			audio := AudioStreamInfo{
				Codec:     track.Format,
				Language:  track.Language,
				IsDefault: strings.EqualFold(track.Default, "Yes"),
			}

			if track.Channels != "" {
				audio.Channels, _ = strconv.Atoi(track.Channels)
			}

			if track.SamplingRate != "" {
				audio.SampleRate, _ = strconv.Atoi(strings.TrimSpace(track.SamplingRate))
			}

			if track.BitRate != "" {
				if bitrate, err := strconv.Atoi(track.BitRate); err == nil {
					audio.Bitrate = bitrate / 1000
				}
			}

			if track.BitDepth != "" {
				audio.BitDepth, _ = strconv.Atoi(track.BitDepth)
			}

			info.Audio = append(info.Audio, audio)
		}
	}

	return info
}

// detectHDR determines the HDR type from ffprobe stream info.
func (ma *MediaAnalyzer) detectHDR(stream *ffProbeStream) string {
	// Check color transfer characteristics
	switch strings.ToLower(stream.ColorTransfer) {
	case "smpte2084":
		// Could be HDR10 or Dolby Vision
		return "HDR10"
	case "arib-std-b67":
		return "HLG"
	}

	// Check color primaries for BT.2020
	if strings.ToLower(stream.ColorPrimaries) == "bt2020" {
		return "HDR"
	}

	return ""
}

// heightToResolution converts video height to resolution string.
func (ma *MediaAnalyzer) heightToResolution(height int) string {
	switch {
	case height >= 2160:
		return "2160p"
	case height >= 1440:
		return "1440p"
	case height >= 1080:
		return "1080p"
	case height >= 720:
		return "720p"
	case height >= 576:
		return "576p"
	case height >= 480:
		return "480p"
	default:
		return ""
	}
}

// parseFrameRate parses a frame rate string like "24000/1001" or "24.0".
func parseFrameRate(s string) float64 {
	if strings.Contains(s, "/") {
		parts := strings.Split(s, "/")
		if len(parts) == 2 {
			num, err1 := strconv.ParseFloat(parts[0], 64)

			den, err2 := strconv.ParseFloat(parts[1], 64)
			if err1 == nil && err2 == nil && den != 0 {
				return num / den
			}
		}

		return 0
	}

	rate, _ := strconv.ParseFloat(s, 64)

	return rate
}

// DefaultMediaAnalyzer is a package-level MediaAnalyzer instance.
var DefaultMediaAnalyzer = NewMediaAnalyzer()

// Analyze uses the default analyzer to extract metadata.
func Analyze(filePath string) (*MediaInfo, error) {
	return DefaultMediaAnalyzer.Analyze(filePath)
}
