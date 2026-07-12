package parser

import (
	"context"
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

// ffProbeStream is one stream entry from ffprobe JSON output.
type ffProbeStream struct {
	Tags           map[string]string `json:"tags"`
	CodecName      string            `json:"codec_name"`
	CodecTagString string            `json:"codec_tag_string"`
	CodecType      string            `json:"codec_type"`
	Height         int               `json:"height,omitempty"`
	Width          int               `json:"width,omitempty"`
	SampleRate     string            `json:"sample_rate"`
	Channels       int               `json:"channels"`
	BitRate        string            `json:"bit_rate"`
	Duration       string            `json:"duration"`
}

// language returns the stream's language tag. ffprobe normally emits the
// lowercase "language" key; check the capitalized variant as well for
// containers/tools that preserve original casing.
func (s *ffProbeStream) language() string {
	if lang := s.Tags["language"]; lang != "" {
		return lang
	}

	return s.Tags["Language"]
}

type ffProbeJSON struct {
	Streams []ffProbeStream `json:"streams"`
	Format  struct {
		// Duration is a string containing the duration of the media
		Duration string            `json:"duration"`
		Filename string            `json:"filename"`
		BitRate  string            `json:"bit_rate"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
	Error struct {
		String string `json:"string"`
		Code   int    `json:"code"`
	} `json:"error"`
}

// CheckAnalyzerPaths verifies at startup that the configured media analyzer
// executables (ffprobe, and mediainfo when enabled) can be resolved, logging
// one clear warning per missing tool instead of letting a misconfigured path
// surface as cryptic per-file errors during scans.
func CheckAnalyzerPaths() {
	if _, err := exec.LookPath(getFFProbeFilename()); err != nil {
		logger.Logtype("warn", 2).
			Str(logger.StrPath, getFFProbeFilename()).
			Err(err).
			Msg("ffprobe executable not found - check FfprobePath; media analysis will fail")
	}

	if config.GetSettingsGeneral().UseMediainfo ||
		config.GetSettingsGeneral().UseMediaFallback {
		if _, err := exec.LookPath(getmediainfoFilename()); err != nil {
			logger.Logtype("warn", 2).
				Str(logger.StrPath, getmediainfoFilename()).
				Err(err).
				Msg("mediainfo executable not found - check MediainfoPath; media analysis will fail")
		}
	}
}

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

	ffprobepath = filepath.Join(config.GetSettingsGeneral().FfprobePath, executable)

	return ffprobepath
}

// buildFFProbeCmd creates an *exec.Cmd for running ffprobe with specific JSON output options on the specified file.
// It sets the log level to fatal, output format to JSON, and selects specific entries to show including
// format duration, stream details, and stream language tags.
func buildFFProbeCmd(ctx context.Context, file string) *exec.Cmd {
	return exec.CommandContext(
		ctx,
		getFFProbeFilename(),
		"-loglevel",
		"fatal",
		"-print_format",
		"json",
		"-show_entries",
		"format=filename,duration,bit_rate,tags : stream=codec_name,codec_tag_string,codec_type,height,width,sample_rate,bit_rate,channels,tags : error",
		file,
	)
}

// buildMediaInfoCmd creates an *exec.Cmd for running mediainfo with JSON output on the specified file.
// It uses getmediainfoFilename() to determine the path to the mediainfo executable
// and adds the "--Output=JSON" flag to generate JSON-formatted output.
func buildMediaInfoCmd(ctx context.Context, file string) *exec.Cmd {
	return exec.CommandContext(ctx,
		getmediainfoFilename(),
		"--Output=JSON",
		file,
	)
}

// getcmd returns an *exec.Cmd for the specified file and command type.
// If the file is empty, it returns nil.
// For "ffprobe" type, it returns a command with the ffprobe executable, loglevel set to "fatal",
// print format set to "json", and specific entries to show.
// For "mediainfo" type, it returns a command with the mediainfo executable and "--Output=JSON" option.
// For any other type, it returns a command with the getImdbFilename() executable.
func getcmd(ctx context.Context, file, typ string) *exec.Cmd {
	switch typ {
	case "ffprobe":
		if file == "" {
			return nil
		}

		return buildFFProbeCmd(ctx, file)

	case "mediainfo":
		if file == "" {
			return nil
		}

		return buildMediaInfoCmd(ctx, file)

	default:
		return exec.CommandContext(ctx,
			getImdbFilename(),
		)
	}
}

// ExecCmdJSON executes the given command with the provided arguments and returns
// the parsed JSON output as either a mediaInfoJSON or ffProbeJSON struct, or an error.
// The command type is specified by the 'typ' parameter, which can be either "ffprobe" or "mediainfo".
// The 'file' parameter is the path to the file to analyze.
// The 'm' and 'quality' parameters are used to pass additional context to the parsemediainfo and parseffprobe functions.
func ExecCmdJSON[T mediaInfoJSON | ffProbeJSON](
	ctx context.Context,
	file, typ string,
	m *database.ParseInfo,
	quality *config.QualityConfig,
) error {
	cmd := getcmd(ctx, file, typ)
	if cmd == nil {
		return logger.ErrNotFound
	}

	outputBuf := logger.PlAddBuffer.Get()

	stdErr := logger.PlAddBuffer.Get()
	defer func() {
		logger.PlAddBuffer.Put(outputBuf)
		logger.PlAddBuffer.Put(stdErr)
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
func ExecCmdString[t []byte | mediaInfoJSON | ffProbeJSON](
	ctx context.Context,
	file, typ string,
) (string, error) {
	cmd := getcmd(ctx, file, typ)
	if cmd == nil {
		return "", logger.ErrNotFound
	}

	outputBuf := logger.PlAddBuffer.Get()

	stdErr := logger.PlAddBuffer.Get()
	defer func() {
		logger.PlAddBuffer.Put(outputBuf)
		logger.PlAddBuffer.Put(stdErr)
	}()

	cmd.Stdout = outputBuf
	cmd.Stderr = stdErr

	err := cmd.Run()

	return outputBuf.String(), err
}
