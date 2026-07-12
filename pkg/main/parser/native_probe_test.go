package parser

import (
	"context"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

// Real sample files supplied for verification. Tests skip when absent.
const (
	sampleAVI  = `C:\Users\mkell\Downloads\Free_Test_Data_5MB_AVI.avi`
	sampleMP4  = `C:\Users\mkell\Downloads\Free_Test_Data_7MB_MP4.mp4`
	sampleMKV  = `C:\Users\mkell\Downloads\Free_Test_Data_3.67_WKV.mkv`
	sampleMPEG = `C:\Users\mkell\Downloads\FTD_5MB.mpeg`
)

func dumpStreams(t *testing.T, r *ffProbeJSON) {
	t.Helper()
	t.Logf("duration=%q", r.Format.Duration)

	for i := range r.Streams {
		s := &r.Streams[i]
		t.Logf("  %-5s codec=%-12s %dx%d ch=%d rate=%s lang=%q",
			s.CodecType, s.CodecName, s.Width, s.Height, s.Channels, s.SampleRate, s.language())
	}
}

func wantVideo(t *testing.T, r *ffProbeJSON, codec string, w, h int, dur float64) {
	t.Helper()

	var v *ffProbeStream

	for i := range r.Streams {
		if r.Streams[i].CodecType == "video" {
			v = &r.Streams[i]
			break
		}
	}

	if v == nil {
		t.Fatal("no video stream")
	}

	if v.CodecName != codec {
		t.Errorf("video codec = %q, want %q", v.CodecName, codec)
	}

	if v.Width != w || v.Height != h {
		t.Errorf("resolution = %dx%d, want %dx%d", v.Width, v.Height, w, h)
	}

	got, err := strconv.ParseFloat(r.Format.Duration, 64)
	if err != nil {
		t.Errorf("duration %q not numeric: %v", r.Format.Duration, err)
	} else if math.Abs(got-dur) > 1.0 {
		t.Errorf("duration = %.3f, want ~%.3f", got, dur)
	}
}

func TestNativeProbeAVI(t *testing.T) {
	if _, err := os.Stat(sampleAVI); err != nil {
		t.Skip("sample AVI not available")
	}

	r, err := nativeProbe(sampleAVI)
	if err != nil {
		t.Fatalf("nativeProbe: %v", err)
	}

	dumpStreams(t, r)
	wantVideo(t, r, "h264", 960, 540, 55.121733)
}

func TestNativeProbeMP4(t *testing.T) {
	if _, err := os.Stat(sampleMP4); err != nil {
		t.Skip("sample MP4 not available")
	}

	r, err := nativeProbe(sampleMP4)
	if err != nil {
		t.Fatalf("nativeProbe: %v", err)
	}

	dumpStreams(t, r)
	wantVideo(t, r, "h264", 960, 540, 55.121733)
}

func TestNativeProbeMKV(t *testing.T) {
	if _, err := os.Stat(sampleMKV); err != nil {
		t.Skip("sample MKV not available")
	}

	r, err := nativeProbe(sampleMKV)
	if err != nil {
		t.Fatalf("nativeProbe: %v", err)
	}

	dumpStreams(t, r)
	wantVideo(t, r, "h264", 1280, 720, 23.789)

	// Audio stream present with codec + channels.
	var a *ffProbeStream

	for i := range r.Streams {
		if r.Streams[i].CodecType == "audio" {
			a = &r.Streams[i]
			break
		}
	}

	if a == nil {
		t.Fatal("no audio stream")
	}

	if a.CodecName != "aac" || a.Channels != 2 {
		t.Errorf("audio = %q/%dch, want aac/2ch", a.CodecName, a.Channels)
	}
}

// ensureMediaEnv loads config + DB (codec/resolution/audio lists, priorities) so
// the end-to-end test can resolve codec/resolution/audio IDs. It is idempotent
// and skips the test when the environment isn't available.
func ensureMediaEnv(t *testing.T) {
	t.Helper()

	if config.GetSettingsGeneral() != nil && len(database.DBConnect.GetcodecsIn) > 0 {
		return // already initialized by another test in this run
	}

	if _, err := os.Stat("config"); err != nil {
		if err := os.Chdir(".."); err != nil {
			t.Skip("cannot locate config dir")
		}
	}

	if err := config.LoadCfgDB(false); err != nil {
		t.Skip("config not available: ", err)
	}

	database.InitCache()

	general := config.GetSettingsGeneral()
	if general == nil {
		t.Skip("settings not available")
	}

	worker.InitWorkerPools(
		general.WorkerSearch, general.WorkerFiles, general.WorkerMetadata,
		general.WorkerRSS, general.WorkerIndexer,
	)
	logger.InitLogger(logger.Config{LogLevel: general.LogLevel, LogToFileOnly: true})

	if err := database.InitDB(general.DBLogLevel); err != nil {
		t.Skip("database not available: ", err)
	}

	_ = database.InitImdbdb()
	database.SetVars()
	GenerateAllQualityPriorities()
	LoadDBPatterns()
}

// TestNativeProbeEndToEnd proves the native probe feeds codec/width/height/audio
// through the same updateVideo/updateAudio path: after ParseVideoFile the
// ParseInfo has codec, resolution and audio populated (with resolved DB IDs).
func TestNativeProbeEndToEnd(t *testing.T) {
	if _, err := os.Stat(sampleMKV); err != nil {
		t.Skip("sample MKV not available")
	}

	ensureMediaEnv(t)

	m := &database.ParseInfo{File: sampleMKV}

	// A non-nil quality config (any name) so the priority lookup doesn't panic.
	if err := ParseVideoFile(context.Background(), m, &config.QualityConfig{Name: "native-test"}); err != nil {
		t.Fatalf("ParseVideoFile: %v", err)
	}

	t.Logf("codec=%q(id %d) %dx%d res=%q(id %d) audio=%q(id %d) runtime=%d langs=%v",
		m.Codec, m.CodecID, m.Width, m.Height, m.Resolution, m.ResolutionID,
		m.Audio, m.AudioID, m.Runtime, m.Languages)

	if m.Codec != "h264" {
		t.Errorf("Codec = %q, want h264", m.Codec)
	}

	if m.Width != 1280 || m.Height != 720 {
		t.Errorf("dimensions = %dx%d, want 1280x720", m.Width, m.Height)
	}

	if m.Resolution != "720p" {
		t.Errorf("Resolution = %q, want 720p", m.Resolution)
	}

	if m.Audio != "aac" {
		t.Errorf("Audio = %q, want aac", m.Audio)
	}

	if m.CodecID == 0 || m.ResolutionID == 0 {
		t.Errorf("expected resolved DB ids, got codecID=%d resolutionID=%d", m.CodecID, m.ResolutionID)
	}
}

// TestNativeProbeMPEG confirms MPEG-PS is not claimed by the native prober, so
// it falls back to ffprobe.
func TestNativeProbeMPEG(t *testing.T) {
	if _, err := os.Stat(sampleMPEG); err != nil {
		t.Skip("sample MPEG not available")
	}

	if _, err := nativeProbe(sampleMPEG); err == nil {
		t.Error("expected native probe to decline MPEG-PS (so ffprobe handles it)")
	}
}
