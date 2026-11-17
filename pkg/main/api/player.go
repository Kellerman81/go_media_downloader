package api

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/tags"
	"github.com/gin-gonic/gin"
)

// The Library player streams a file by its files-table ID (never a path):
//   - direct: the browser can play the file as it is (HTTP range requests);
//   - remux: H.264 video in another container is copied into fragmented MP4
//     with AAC audio, cheap enough for any machine;
//   - transcode: ffmpeg encodes to the chosen quality (player_video_profiles,
//     player_audio_bitrates), at most player_max_transcodes at a time.
// Remuxed and transcoded streams can't be seeked by the browser; the player
// restarts them at the wanted position (start=seconds).

// playKind is a files table the player streams from.
type playKind struct {
	Table string
	Audio bool
}

var playKinds = map[string]playKind{
	"movie":     {"movie_files", false},
	"episode":   {"serie_episode_files", false},
	"track":     {"album_files", true},
	"audiobook": {"audiobook_files", true},
}

var (
	directVideoExts = map[string]bool{".mp4": true, ".m4v": true, ".webm": true}
	directAudioExts = map[string]bool{
		".mp3": true, ".m4a": true, ".m4b": true, ".aac": true, ".flac": true,
		".ogg": true, ".oga": true, ".opus": true, ".wav": true,
	}
	browserVideoCodecs = map[string]bool{"h264": true, "vp8": true, "vp9": true, "av1": true}
	browserAudioCodecs = map[string]bool{"aac": true, "mp3": true, "opus": true, "vorbis": true, "flac": true}
)

// playFile returns the location of a file of a play kind.
func playFile(kind, id string) (string, playKind, bool) {
	pk, ok := playKinds[kind]
	if !ok {
		return "", pk, false
	}

	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return "", pk, false
	}

	loc := database.Getdatarow[string](false, "SELECT location FROM "+pk.Table+" WHERE id = ?", &n)
	if loc == "" {
		return "", pk, false
	}

	return loc, pk, true
}

// playProbe is what the player needs to know about a file.
type playProbe struct {
	Duration float64 `json:"duration"`
	VCodec   string  `json:"vcodec"`
	ACodec   string  `json:"acodec"`
	Height   int     `json:"height"`
	Kbps     int     `json:"kbps"` // overall bitrate of the file
	// AudioTracks and SubTracks of videos, in stream order.
	AudioTracks []playTrack `json:"-"`
	SubTracks   []playTrack `json:"-"`
	// Tags are the file's own tags (title, artist, album, ...), keys in lower case.
	Tags map[string]string `json:"-"`
}

type probeCacheEntry struct {
	modTime time.Time
	probe   playProbe
}

var probeCache sync.Map // path -> probeCacheEntry

// probeMedia reads duration and codecs of a file with ffprobe (cached by
// path and modification time).
func probeMedia(ctx context.Context, path string) (playProbe, error) {
	st, err := os.Stat(path)
	if err != nil {
		return playProbe{}, err
	}

	if e, ok := probeCache.Load(path); ok && e.(probeCacheEntry).modTime.Equal(st.ModTime()) {
		return e.(probeCacheEntry).probe, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, config.FfprobeExecutable(), "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path).Output()
	if err != nil {
		return playProbe{}, fmt.Errorf("ffprobe: %w", err)
	}

	var raw struct {
		Format struct {
			Duration string            `json:"duration"`
			BitRate  string            `json:"bit_rate"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			CodecType   string `json:"codec_type"`
			CodecName   string `json:"codec_name"`
			Height      int    `json:"height"`
			Channels    int    `json:"channels"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
				Default     int `json:"default"`
				Forced      int `json:"forced"`
			} `json:"disposition"`
			Tags struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(out, &raw); err != nil {
		return playProbe{}, err
	}

	var p playProbe

	// Vorbis/FLAC tags come in upper case (TITLE), others in lower case.
	p.Tags = make(map[string]string, len(raw.Format.Tags))
	for k, v := range raw.Format.Tags {
		if v = strings.TrimSpace(v); v != "" {
			p.Tags[strings.ToLower(k)] = v
		}
	}

	p.Duration, _ = strconv.ParseFloat(raw.Format.Duration, 64)
	if bps, err := strconv.Atoi(raw.Format.BitRate); err == nil {
		p.Kbps = bps / 1000
	}

	for _, s := range raw.Streams {
		switch {
		case s.CodecType == "video" && s.Disposition.AttachedPic == 0 && p.VCodec == "":
			p.VCodec, p.Height = s.CodecName, s.Height
		case s.CodecType == "audio" && p.ACodec == "":
			p.ACodec = s.CodecName
		}

		t := playTrack{Lang: langCode(s.Tags.Language), Title: strings.TrimSpace(s.Tags.Title), Codec: s.CodecName,
			Channels: s.Channels, Default: s.Disposition.Default == 1, Forced: s.Disposition.Forced == 1}

		switch s.CodecType {
		case "audio":
			t.Index = len(p.AudioTracks)
			p.AudioTracks = append(p.AudioTracks, t)
		case "subtitle":
			t.Index = len(p.SubTracks)
			p.SubTracks = append(p.SubTracks, t)
		}
	}

	probeCache.Store(path, probeCacheEntry{modTime: st.ModTime(), probe: p})

	return p, nil
}

// playMeta is what the player shows of a file: Title (the track, part,
// movie or episode) and Sub (artist · album, author · book, year · genres,
// series · S01E02); Artist and Album go to the media session.
type playMeta struct {
	Title  string `json:"title"`
	Sub    string `json:"sub"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
	// Cover of the album, audiobook, movie or series the file belongs to.
	Cover string `json:"cover,omitempty"`
}

// playMetaOf takes the metadata from the database and the file's own tags:
// for music the tags name the track and its performer (compilations list
// "Various Artists" for the album), the database the album.
func playMetaOf(kind, id, path string, p playProbe) playMeta {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	join := func(parts ...string) string {
		out := make([]string, 0, len(parts))
		for _, s := range parts {
			if s = strings.TrimSpace(s); s != "" && s != "0" {
				out = append(out, s)
			}
		}

		return strings.Join(out, " · ")
	}

	type row struct {
		A string `db:"a"`
		B string `db:"b"`
		C string `db:"c"`
		D string `db:"d"` // id of the db* row with the cover
	}

	one := func(query string) row {
		if rows := database.StructscanT[row](false, 1, query, id); len(rows) > 0 {
			return rows[0]
		}

		return row{}
	}

	var m playMeta

	switch kind {
	case "track":
		r := one(`SELECT COALESCE(t.title, '') AS a, COALESCE(d.title, '') AS b,
			COALESCE((SELECT ar.name FROM dbalbum_artists daa INNER JOIN dbartists ar ON ar.id = daa.dbartist_id
				WHERE daa.dbalbum_id = f.dbalbum_id ORDER BY daa.position LIMIT 1), '') AS c,
			COALESCE(CAST(f.dbalbum_id AS TEXT), '') AS d
			FROM album_files f LEFT JOIN dbtracks t ON t.id = f.dbtrack_id LEFT JOIN dbalbums d ON d.id = f.dbalbum_id WHERE f.id = ?`)
		m.Title = cmp.Or(p.Tags["title"], r.A, stem)
		m.Artist = cmp.Or(p.Tags["artist"], r.C, p.Tags["album_artist"])
		m.Album = cmp.Or(r.B, p.Tags["album"])
		m.Sub = join(m.Artist, m.Album)
		m.Cover = playCover("album", r.D)
	case "audiobook":
		r := one(`SELECT COALESCE(d.title, '') AS a,
			COALESCE((SELECT au.name FROM dbaudiobook_authors daa INNER JOIN dbauthors au ON au.id = daa.dbauthor_id
				WHERE daa.dbaudiobook_id = f.dbaudiobook_id ORDER BY daa.position LIMIT 1), '') AS b, '' AS c,
			COALESCE(CAST(f.dbaudiobook_id AS TEXT), '') AS d
			FROM audiobook_files f LEFT JOIN dbaudiobooks d ON d.id = f.dbaudiobook_id WHERE f.id = ?`)
		m.Title = cmp.Or(p.Tags["title"], stem)
		m.Artist = cmp.Or(r.B, p.Tags["artist"], p.Tags["album_artist"])
		m.Album = cmp.Or(r.A, p.Tags["album"])
		m.Sub = join(m.Artist, m.Album)
		m.Cover = playCover("audiobook", r.D)
	case "movie":
		r := one(`SELECT COALESCE(d.title, '') AS a, COALESCE(CAST(d.year AS TEXT), '') AS b, COALESCE(d.genres, '') AS c,
			COALESCE(CAST(f.dbmovie_id AS TEXT), '') AS d
			FROM movie_files f LEFT JOIN dbmovies d ON d.id = f.dbmovie_id WHERE f.id = ?`)
		m.Title = cmp.Or(r.A, stem)
		m.Sub = join(r.B, strings.Join(strings.Fields(strings.ReplaceAll(r.C, ",", ", ")), " "))
		m.Album = r.B
		m.Cover = playCover("movie", r.D)
	case "episode":
		r := one(`SELECT COALESCE(ds.seriename, '') AS a, COALESCE(dse.title, '') AS b, COALESCE(dse.identifier, '') AS c,
			COALESCE(CAST(f.dbserie_id AS TEXT), '') AS d
			FROM serie_episode_files f LEFT JOIN dbserie_episodes dse ON dse.id = f.dbserie_episode_id
			LEFT JOIN dbseries ds ON ds.id = f.dbserie_id WHERE f.id = ?`)
		m.Title = cmp.Or(r.B, r.C, stem)
		m.Sub = join(r.A, r.C)
		m.Artist, m.Album = r.A, r.C
		m.Cover = playCover("series", r.D)
	default:
		m.Title = stem
	}

	return m
}

// playCover is the library cover URL of a db* row ("" without one).
func playCover(typ, dbID string) string {
	if dbID == "" || dbID == "0" {
		return ""
	}

	return "/api/admin/library/cover/" + typ + "/" + dbID
}

// playMode decides how the original quality is played.
func playMode(path string, pk playKind, p playProbe) string {
	ext := strings.ToLower(filepath.Ext(path))

	if pk.Audio {
		if directAudioExts[ext] && (p.ACodec == "" || browserAudioCodecs[p.ACodec]) {
			return "direct"
		}

		return "transcode"
	}

	if directVideoExts[ext] && browserVideoCodecs[p.VCodec] && (p.ACodec == "" || browserAudioCodecs[p.ACodec]) {
		return "direct"
	}

	if p.VCodec == "h264" {
		return "remux"
	}

	return "transcode"
}

// videoProfile is one entry of player_video_profiles.
type videoProfile struct {
	Name                 string
	Height, VKbps, AKbps int
}

func videoProfiles() []videoProfile {
	var out []videoProfile

	for _, entry := range config.GetSettingsGeneral().PlayerVideoProfiles {
		parts := strings.Split(entry, "|")
		if len(parts) != 4 {
			continue
		}

		p := videoProfile{Name: strings.TrimSpace(parts[0])}
		p.Height, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
		p.VKbps, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
		p.AKbps, _ = strconv.Atoi(strings.TrimSpace(parts[3]))

		if p.Name != "" && p.Height > 0 && p.VKbps > 0 && p.AKbps > 0 {
			out = append(out, p)
		}
	}

	return out
}

// playQuality is a quality the player offers. Kbps is the bitrate the
// browser has to receive; Speed is how fast the server last converted this
// quality (1 = real time, 0 = not measured yet), for marking the qualities
// that play without stalls.
type playQuality struct {
	Value     string  `json:"value"`
	Label     string  `json:"label"`
	Kbps      int     `json:"kbps"`
	Converted bool    `json:"converted"`
	Speed     float64 `json:"speed,omitempty"`
}

type playChapter struct {
	Title string  `json:"title"`
	Start float64 `json:"start"`
}

// HandlePlayerInfo describes a file for the player (/admin/library/play/:kind/:id/info).
func HandlePlayerInfo(c *gin.Context) {
	path, pk, ok := playFileFor(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	p, err := probeMedia(c.Request.Context(), path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "the file can't be read: " + err.Error()})
		return
	}

	mode := playMode(path, pk, p)
	audioOnly := pk.Audio || p.VCodec == ""

	// The original as it is sent: the file itself, the file repackaged
	// (about its own bitrate), or converted like the first profile.
	orig := playQuality{Value: "original", Kbps: p.Kbps, Converted: mode != "direct"}

	switch {
	case mode == "transcode" && audioOnly:
		orig.Kbps = 192
	case mode == "transcode":
		if vps := videoProfiles(); len(vps) > 0 {
			orig.Kbps = vps[0].VKbps + vps[0].AKbps
		}
	}

	orig.Label = map[string]string{"direct": "Original", "remux": "Original (repackaged)", "transcode": "Original (converted)"}[mode] +
		kbpsLabel(orig.Kbps)
	qualities := []playQuality{orig}

	if audioOnly {
		for _, kbps := range config.GetSettingsGeneral().PlayerAudioBitrates {
			if kbps > 0 {
				qualities = append(qualities, playQuality{Value: strconv.Itoa(kbps), Label: strconv.Itoa(kbps) + " kbit/s", Kbps: kbps, Converted: true})
			}
		}
	} else {
		for _, vp := range videoProfiles() {
			qualities = append(qualities, playQuality{Value: vp.Name, Label: fmt.Sprintf("%s (%.1f Mbit/s)", vp.Name, float64(vp.VKbps+vp.AKbps)/1000),
				Kbps: vp.VKbps + vp.AKbps, Converted: true})
		}
	}

	for i := range qualities {
		if qualities[i].Converted {
			if v, ok := conversionSpeeds.Load(speedKey(audioOnly, p, qualities[i].Value, playerHWAccel())); ok {
				qualities[i].Speed = v.(float64)
			}
		}
	}

	var chapters []playChapter

	// Chapters belong to the whole audiobook; they only map onto a single file.
	if c.Param("kind") == "audiobook" {
		dbID := database.Getdatarow[uint](false, "SELECT dbaudiobook_id FROM audiobook_files WHERE id = ?", c.Param("id"))
		if database.Getdatarow[int](false, "SELECT count() FROM audiobook_files WHERE dbaudiobook_id = ?", &dbID) == 1 {
			for _, ch := range database.StructscanT[struct {
				Title string `db:"title"`
				Start int64  `db:"start_time_ms"`
			}](false, 50, "SELECT COALESCE(title, '') AS title, COALESCE(start_time_ms, 0) AS start_time_ms FROM dbaudiobook_chapters WHERE dbaudiobook_id = ? ORDER BY position, chapter_number", &dbID) {
				chapters = append(chapters, playChapter{Title: ch.Title, Start: float64(ch.Start) / 1000})
			}
		}
	}

	fileID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	resume := 0.0

	if c.Param("kind") != "track" {
		fid := uint(fileID)
		resume = database.PlayResume(accName(accessOf(c)), c.Param("kind"), &fid)
	}

	res := gin.H{
		"resume": resume, "serverKbps": serverShareKbps(),
		"duration": p.Duration, "mode": mode, "audio": audioOnly,
		"qualities": qualities, "chapters": chapters, "name": filepath.Base(path),
		"meta": playMetaOf(c.Param("kind"), c.Param("id"), path, p),
	}

	if !audioOnly {
		res["audioTracks"] = audioInfos(p)
		res["subtitles"] = subInfos(path, p)
	}

	c.JSON(http.StatusOK, res)
}

var (
	transcodeMu    sync.Mutex
	transcodeCount int
)

func acquireTranscode() bool {
	transcodeMu.Lock()
	defer transcodeMu.Unlock()

	if transcodeCount >= config.GetSettingsGeneral().PlayerMaxTranscodes {
		return false
	}

	transcodeCount++

	return true
}

func releaseTranscode() {
	transcodeMu.Lock()
	transcodeCount--
	transcodeMu.Unlock()
}

// transcodeWait is how long a stream waits for a free conversion: a stream
// the player just left frees its slot only once the server noticed and
// ffmpeg ended.
const transcodeWait = 3 * time.Second

// acquireTranscodeWait is acquireTranscode, retried for up to wait.
func acquireTranscodeWait(ctx context.Context, wait time.Duration) bool {
	deadline := time.Now().Add(wait)

	for {
		if acquireTranscode() {
			return true
		}

		if time.Now().After(deadline) {
			return false
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// playerStream is the stream a player (?pid=, one per page) is receiving.
type playerStream struct {
	cancel context.CancelFunc
	done   chan struct{}
}

var (
	playerStreamsMu sync.Mutex
	playerStreams   = map[string]*playerStream{}
)

// claimPlayerStream makes this request the player's only stream: one it
// started before (a jump that left the buffered part, several in a row) is
// stopped, and its conversion slot waited for. The returned function ends
// the claim; it must run after the slot is released.
func claimPlayerStream(pid string, cancel context.CancelFunc) func() {
	if pid == "" {
		return func() {}
	}

	mine := &playerStream{cancel: cancel, done: make(chan struct{})}

	playerStreamsMu.Lock()
	old := playerStreams[pid]
	playerStreams[pid] = mine
	playerStreamsMu.Unlock()

	if old != nil {
		old.cancel()

		select {
		case <-old.done:
		case <-time.After(transcodeWait):
		}
	}

	return func() {
		playerStreamsMu.Lock()
		if playerStreams[pid] == mine {
			delete(playerStreams, pid)
		}
		playerStreamsMu.Unlock()
		close(mine.done)
	}
}

// kbpsLabel is " (12.3 Mbit/s)" or " (320 kbit/s)"; empty when unknown.
func kbpsLabel(kbps int) string {
	switch {
	case kbps <= 0:
		return ""
	case kbps >= 1000:
		return fmt.Sprintf(" (%.1f Mbit/s)", float64(kbps)/1000)
	}

	return fmt.Sprintf(" (%d kbit/s)", kbps)
}

// conversionSpeeds holds the last measured conversion speed per quality and
// source size (speedKey), as a multiple of real time.
var conversionSpeeds sync.Map

// speedKey groups conversions that cost about the same: the quality and,
// for video, the source height.
func speedKey(audioOnly bool, p playProbe, quality, hw string) string {
	if audioOnly {
		return "audio|" + quality
	}

	return fmt.Sprintf("video|%s|%d|%s|%s", quality, p.Height, p.VCodec, hw)
}

// ffmpegArgs builds the ffmpeg arguments of a remux or transcode to
// fragmented MP4 (video) or MP3 (audio) on stdout. quality is "original" or
// a profile/bitrate; sel picks the audio track and a subtitle to burn in.
// hw is "none", "vaapi" or "qsv"; usedHW reports whether the GPU encodes
// (only video conversions use it).
func ffmpegArgs(path string, pk playKind, p playProbe, quality string, start float64, hw string, sel playSel) (args []string, usedHW bool, err error) {
	// -progress reports the conversion speed on stderr (ffmpegStderr).
	args = []string{"-hide_banner", "-loglevel", "error", "-nostats", "-progress", "pipe:2"}

	audioOnly := pk.Audio || p.VCodec == ""
	copyVideo := copiesVideo(pk, p, quality, sel)
	usedHW = hw != "none" && !audioOnly && !copyVideo

	dev := config.GetSettingsGeneral().PlayerHWDevice
	if usedHW && hw == "vaapi" {
		args = append(args, "-vaapi_device", dev)
	}

	if usedHW && hw == "qsv" {
		args = append(args, "-init_hw_device", "vaapi=va:"+dev, "-init_hw_device", "qsv=qs@va", "-filter_hw_device", "qs")
	}

	if start > 0 {
		// Copied video starts at a keyframe: the audio must start there too,
		// not trimmed to the start (streamStart).
		if copyVideo {
			args = append(args, "-noaccurate_seek")
		}

		args = append(args, "-ss", strconv.FormatFloat(start, 'f', 3, 64))
	}

	// Some containers (AVI with packed B-frames) carry no timestamp on every
	// other frame; ffmpeg 7 then writes broken ones (negative, or a frame
	// lasting seconds), which leaves browsers stuck seeking or buffering.
	args = append(args, "-fflags", "+genpts", "-i", path)

	if audioOnly {
		kbps := 192
		if quality != "original" {
			n, err := strconv.Atoi(quality)
			if err != nil || n <= 0 {
				return nil, false, fmt.Errorf("unknown audio quality %q", quality)
			}

			kbps = n
		}

		// Plain MP3, not fragmented MP4: browsers play an audio-only fMP4
		// stream badly (Chrome re-requests it and gives up).
		return append(args, "-vn", "-map", "0:a:0", "-c:a", "libmp3lame", "-b:a", strconv.Itoa(min(kbps, 320))+"k",
			"-ar", "44100", "-ac", "2", "-f", "mp3", "pipe:1"), false, nil
	} else {
		audioMap := "0:a:" + strconv.Itoa(sel.Audio) + "?"

		if copyVideo {
			args = append(args, "-map", "0:v:0", "-map", audioMap, "-sn")
			args = append(args, "-c:v", "copy", "-c:a", "aac", "-b:a", "192k", "-ac", "2", "-af", audioResync)
		} else {
			profiles := videoProfiles()
			if len(profiles) == 0 {
				return nil, false, fmt.Errorf("no player_video_profiles configured")
			}

			vp := profiles[0]

			if quality != "original" {
				found := false

				for _, cand := range profiles {
					if cand.Name == quality {
						vp, found = cand, true
					}
				}

				if !found {
					return nil, false, fmt.Errorf("unknown video quality %q", quality)
				}
			}

			rate := []string{"-b:v", strconv.Itoa(vp.VKbps) + "k", "-maxrate", strconv.Itoa(vp.VKbps) + "k", "-bufsize", strconv.Itoa(2*vp.VKbps) + "k"}
			chain := fmt.Sprintf("scale=-2:'min(%d,ih)'", vp.Height)

			// The GPU only encodes: decoding and scaling stay on the CPU,
			// which works for every source codec.
			var codec []string

			switch {
			case usedHW && hw == "vaapi":
				chain += ",format=nv12,hwupload"
				codec = []string{"-c:v", "h264_vaapi"}
			case usedHW && hw == "qsv":
				chain += ",format=nv12,hwupload=extra_hw_frames=64"
				codec = []string{"-c:v", "h264_qsv", "-preset", "veryfast"}
			default:
				codec = []string{"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p"}
			}

			if sel.Burn >= 0 {
				// The picture subtitle is laid over the video before scaling,
				// centred at the bottom when its canvas differs.
				args = append(args, "-filter_complex",
					"[0:v:0][0:s:"+strconv.Itoa(sel.Burn)+"]overlay=x=(W-w)/2:y=H-h:eof_action=pass,"+chain+"[v]",
					"-map", "[v]", "-map", audioMap, "-sn")
			} else {
				args = append(args, "-map", "0:v:0", "-map", audioMap, "-sn", "-vf", chain)
			}

			args = append(args, codec...)

			args = append(args, rate...)
			args = append(args, "-c:a", "aac", "-b:a", strconv.Itoa(vp.AKbps)+"k", "-ac", "2", "-af", audioResync)
		}
	}

	return append(args, "-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof", "pipe:1"), usedHW, nil
}

// audioResync keeps the converted audio continuous with its timestamps:
// gaps (a broken first packet after a seek, damaged sources) are filled with
// silence and overlaps dropped. Browsers otherwise report "Failed to
// reconcile encoded audio times" and stall on such a stream.
const audioResync = "aresample=async=1"

// copiesVideo: the stream repackages the video as it is (original h264);
// a burned-in subtitle means converting, like any other quality.
func copiesVideo(pk playKind, p playProbe, quality string, sel playSel) bool {
	return !pk.Audio && p.VCodec == "h264" && quality == "original" && sel.Burn < 0
}

// keyframeStart is where a repackaged stream asked to start at t begins:
// copied video can only start at a keyframe, so the stream starts at the
// keyframe at or before t, audio included. Started at t instead, the audio
// begins later than the video, the MP4 marks that gap as one long first
// audio frame, and browsers play the audio that much early (1-3 s).
func keyframeStart(ctx context.Context, path string, t float64) float64 {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// ffprobe seeks like ffmpeg (to the keyframe before t) and lists the
	// video packets from there.
	out, err := exec.CommandContext(ctx, config.FfprobeExecutable(), "-v", "error", "-read_intervals", strconv.FormatFloat(t, 'f', 3, 64)+"%+#50",
		"-select_streams", "v:0", "-show_entries", "packet=pts_time,flags", "-of", "csv=p=0", path).Output()
	if err != nil {
		return t
	}

	for line := range strings.SplitSeq(string(out), "\n") {
		pts, flags, ok := strings.Cut(strings.TrimSpace(line), ",")
		if !ok || !strings.HasPrefix(flags, "K") {
			continue
		}

		if k, err := strconv.ParseFloat(pts, 64); err == nil && k <= t && k > t-60 {
			return k
		}

		break
	}

	return t
}

// streamStart moves the start of a repackaged stream to its keyframe
// (keyframeStart). ffmpeg gets a start 0.2 s after it: it seeks to the
// keyframe before (start - 0.13 s with B-frames), which is this one, and
// with -noaccurate_seek (ffmpegArgs) keeps the audio from there too.
func streamStart(ctx context.Context, path string, pk playKind, p playProbe, quality string, sel playSel, start float64) (shown, seek float64) {
	if start <= 0 || !copiesVideo(pk, p, quality, sel) {
		return start, start
	}

	k := keyframeStart(ctx, path, start)

	return k, k + 0.2
}

var (
	hwFailMu    sync.Mutex
	hwFailedAt  time.Time
	hwRetryWait = 10 * time.Minute
)

// playerHWAccel is the configured GPU encoding, or "none" while it failed
// in the last hwRetryWait.
func playerHWAccel() string {
	hw := config.GetSettingsGeneral().PlayerHWAccel
	if hw != "vaapi" && hw != "qsv" {
		return "none"
	}

	hwFailMu.Lock()
	defer hwFailMu.Unlock()

	if !hwFailedAt.IsZero() && time.Since(hwFailedAt) < hwRetryWait {
		return "none"
	}

	return hw
}

func markHWFailed() {
	hwFailMu.Lock()
	hwFailedAt = time.Now()
	hwFailMu.Unlock()
}

// HandlePlayerStream streams a file (/admin/library/play/:kind/:id/stream?q=&start=).
func HandlePlayerStream(c *gin.Context) {
	path, pk, ok := playFileFor(c)
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	done, ok := streamGuard(c, c.Param("kind"), c.Param("id"))
	if !ok {
		return
	}
	defer done()

	quality := c.DefaultQuery("q", "original")

	p, err := probeMedia(c.Request.Context(), path)
	if err != nil {
		c.String(http.StatusInternalServerError, "the file can't be read")
		return
	}

	sel, err := selFromQuery(c, p)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	if quality == "original" && playMode(path, pk, p) == "direct" && sel.isDefault() {
		// ServeFile handles range requests, so the browser can seek.
		c.File(path)
		return
	}

	start, _ := strconv.ParseFloat(c.Query("start"), 64)
	shown, start := streamStart(c.Request.Context(), path, pk, p, quality, sel, start)

	ffmpeg, err := tags.ResolveFFmpeg()
	if err != nil {
		c.String(http.StatusServiceUnavailable, err.Error())
		return
	}

	// A newer stream of the same player cancels this one (claimPlayerStream).
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	defer claimPlayerStream(c.Query("pid"), cancel)()

	if !acquireTranscodeWait(ctx, transcodeWait) {
		c.String(http.StatusServiceUnavailable, "too many streams are converted right now (player_max_transcodes)")
		return
	}
	defer releaseTranscode()

	buf := newStreamBuffer(streamBufferBytes)

	// Cancelled: stop sending what is still buffered.
	stop := context.AfterFunc(ctx, buf.Close)
	defer stop()
	hw := playerHWAccel()

	var (
		cmd    *exec.Cmd
		stdout io.ReadCloser
		stderr *ffmpegStderr
		first  = make([]byte, 64<<10)
		n      int
	)

	// A GPU encoder that fails (no device, unsupported) ends ffmpeg before
	// any output: then the same request is served by the CPU.
	for {
		args, usedHW, err := ffmpegArgs(path, pk, p, quality, start, hw, sel)
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		if !usedHW {
			hw = "none"
		}

		// Bound to the request: closing the player stops ffmpeg.
		cmd = exec.CommandContext(ctx, ffmpeg, args...)

		stdout, err = cmd.StdoutPipe()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		stderr = &ffmpegStderr{buf: buf}
		cmd.Stderr = stderr

		if err := cmd.Start(); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		n, _ = io.ReadAtLeast(stdout, first, 1)
		if n > 0 || hw == "none" || ctx.Err() != nil {
			break
		}

		_ = cmd.Wait()

		markHWFailed()
		logger.Logtype("warn", 0).Str("hwaccel", hw).Str("ffmpeg", strings.TrimSpace(stderr.messages())).
			Msg("Player: GPU encoding failed, using the CPU (GPU retried in 10 minutes)")

		hw = "none"
	}

	contentType := "video/mp4"
	if pk.Audio || p.VCodec == "" {
		contentType = "audio/mpeg"
	}

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "no-store")
	// Where in the file the stream begins (the player's offset).
	c.Header("X-Stream-Start", strconv.FormatFloat(shown, 'f', 3, 64))
	c.Status(http.StatusOK)

	// ffmpeg writes into a buffer instead of the connection, so it keeps
	// converting ahead while the network is slow, and the backlog goes out
	// as soon as the connection is fast again.
	filled := make(chan struct{})

	go func() {
		defer close(filled)

		if _, err := buf.Write(first[:n]); err != nil {
			buf.CloseWrite(err)
			return
		}

		_, err := io.Copy(buf, stdout)
		buf.CloseWrite(err)
	}()

	_, copyErr := io.Copy(c.Writer, buf)

	// The client is gone (or got everything): stop ffmpeg if it still runs,
	// and let the reader goroutine end before Wait closes the pipe.
	buf.Close()

	if copyErr != nil {
		_ = cmd.Process.Kill()
	}

	<-filled

	waitErr := cmd.Wait()

	if speed := stderr.measured(); speed > 0 {
		conversionSpeeds.Store(speedKey(pk.Audio || p.VCodec == "", p, quality, hw), speed)
	}

	if copyErr == nil && waitErr != nil && ctx.Err() == nil {
		logger.Logtype("error", 0).Err(waitErr).Str("file", path).Str("ffmpeg", strings.TrimSpace(stderr.messages())).
			Msg("Player: ffmpeg failed")
	}
}

// streamBufferBytes is how far ffmpeg may run ahead of the client.
const streamBufferBytes = 64 << 20

var errStreamClosed = errors.New("stream closed")

// streamBuffer is a bounded in-memory pipe: Write blocks while it holds max
// bytes, Read blocks while it is empty and the writer isn't done.
//
// It holds the data in fixed blocks from streamBlockPool rather than a
// bytes.Buffer: a buffer written and read at the same time keeps
// reallocating and copying (a 64 MB stream peaked at ~180 MB of heap, and
// every seek starts a new stream). Blocks are taken as the backlog grows and
// returned once sent, so memory follows the backlog and is reused across
// streams.
type streamBuffer struct {
	mu     sync.Mutex
	cond   *sync.Cond
	blocks []*[]byte // queued data; the last block may still be filling
	off    int       // read position in blocks[0]
	size   int       // bytes queued
	max    int
	full   bool  // was full at least once
	done   bool  // writer finished
	err    error // writer's error
	closed bool  // reader gone
}

const streamBlockSize = 256 << 10

var streamBlockPool = sync.Pool{New: func() any {
	b := make([]byte, 0, streamBlockSize)
	return &b
}}

func newStreamBuffer(maxBytes int) *streamBuffer {
	b := &streamBuffer{max: maxBytes}
	b.cond = sync.NewCond(&b.mu)

	return b
}

func (b *streamBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	written := 0

	for len(p) > 0 {
		for b.size >= b.max && !b.closed {
			b.full = true
			b.cond.Wait()
		}

		if b.closed {
			return written, errStreamClosed
		}

		if len(b.blocks) == 0 || len(*b.blocks[len(b.blocks)-1]) == streamBlockSize {
			blk := streamBlockPool.Get().(*[]byte)
			*blk = (*blk)[:0]
			b.blocks = append(b.blocks, blk)
		}

		last := b.blocks[len(b.blocks)-1]
		n := min(len(p), streamBlockSize-len(*last), b.max-b.size)
		*last = append(*last, p[:n]...)
		p = p[n:]
		b.size += n
		written += n

		b.cond.Broadcast()
	}

	return written, nil
}

func (b *streamBuffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for b.size == 0 && !b.done && !b.closed {
		b.cond.Wait()
	}

	if b.size == 0 {
		if b.err != nil {
			return 0, b.err
		}

		return 0, io.EOF
	}

	first := b.blocks[0]
	n := copy(p, (*first)[b.off:])
	b.off += n
	b.size -= n

	// A block read to its end goes back to the pool; the block the writer
	// is still filling is reused in place.
	if b.off == len(*first) {
		b.off = 0
		if len(b.blocks) > 1 || len(*first) == streamBlockSize {
			b.blocks[0] = nil
			b.blocks = b.blocks[1:]
			streamBlockPool.Put(first)
		} else {
			*first = (*first)[:0]
		}
	}

	b.cond.Broadcast()

	return n, nil
}

// CloseWrite marks the end of the data; err is passed on to the reader.
func (b *streamBuffer) CloseWrite(err error) {
	b.mu.Lock()
	b.done, b.err = true, err
	b.cond.Broadcast()
	b.mu.Unlock()
}

// Close ends the stream from the reader's side; a blocked Write returns.
func (b *streamBuffer) Close() {
	b.mu.Lock()
	b.closed = true

	for _, blk := range b.blocks {
		streamBlockPool.Put(blk)
	}

	b.blocks, b.off, b.size = nil, 0, 0
	b.cond.Broadcast()
	b.mu.Unlock()
}

// ffmpegStderr splits ffmpeg's stderr into -progress lines (key=value) and
// messages. It keeps the reported speed while ffmpeg could run freely: once
// the stream buffer was full, ffmpeg waits for the client and its speed only
// shows the playback rate.
type ffmpegStderr struct {
	buf *streamBuffer

	mu      sync.Mutex
	line    []byte
	msgs    strings.Builder
	outTime float64 // seconds converted
	speed   float64
}

func (w *ffmpegStderr) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.line = append(w.line, p...)

	for {
		i := bytes.IndexByte(w.line, '\n')
		if i < 0 {
			break
		}

		w.handle(strings.TrimSpace(string(w.line[:i])))
		w.line = w.line[i+1:]
	}

	return len(p), nil
}

func (w *ffmpegStderr) handle(line string) {
	key, value, ok := strings.Cut(line, "=")
	if !ok || strings.ContainsAny(key, " \t") || strings.Contains(value, " ") {
		if line != "" && w.msgs.Len() < 4096 {
			w.msgs.WriteString(line + "\n")
		}

		return
	}

	switch key {
	case "out_time_us":
		if us, err := strconv.ParseInt(value, 10, 64); err == nil {
			w.outTime = float64(us) / 1e6
		}
	case "speed":
		// The first seconds are dominated by the start-up; a full buffer
		// means ffmpeg was held back by the client.
		if v, err := strconv.ParseFloat(strings.TrimSuffix(value, "x"), 64); err == nil && w.outTime >= 10 && !w.buf.wasFull() {
			w.speed = v
		}
	}
}

func (w *ffmpegStderr) messages() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.msgs.String()
}

// measured is the conversion speed, 0 when it couldn't be measured.
func (w *ffmpegStderr) measured() float64 {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.speed
}

// speedTestBlock is incompressible data for the player's connection test.
var speedTestBlock = func() []byte {
	b := make([]byte, 1<<20)
	_, _ = rand.Read(b)

	return b
}()

// HandlePlayerSpeedTest sends kb kilobytes of random data
// (/admin/library/play/speedtest?kb=), so the player can measure the
// connection and mark the qualities that play without stalls.
func HandlePlayerSpeedTest(c *gin.Context) {
	kb, _ := strconv.Atoi(c.Query("kb"))
	kb = min(max(kb, 64), 32<<10)

	// Within the total bandwidth: the result is what a stream would get now.
	done, _ := streamGuard(c, "speedtest", "")
	defer done()

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Length", strconv.Itoa(kb<<10))
	c.Status(http.StatusOK)

	for left := kb << 10; left > 0; {
		n := min(left, len(speedTestBlock))
		if _, err := c.Writer.Write(speedTestBlock[:n]); err != nil {
			return
		}

		left -= n
	}
}

func (b *streamBuffer) wasFull() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.full
}

// bookPart is one file of a multi-part audiobook on the book's timeline.
type bookPart struct {
	ID       uint    `json:"id"`
	Start    float64 `json:"start"`
	Duration float64 `json:"duration"`
}

// HandlePlayerBook lays the given audiobook files end to end
// (/admin/library/play/book?ids=1,2,3, in playing order) and returns the
// book's chapters on that timeline, so the player shows one timeline and
// chapter list across all parts.
func HandlePlayerBook(c *gin.Context) {
	var ids []uint

	for _, f := range strings.Split(c.Query("ids"), ",") {
		if n, err := strconv.ParseUint(strings.TrimSpace(f), 10, 64); err == nil && len(ids) < 1000 {
			ids = append(ids, uint(n))
		}
	}

	if acc := accessOf(c); !acc.Admin() {
		ak := libKindByType("audiobook")
		for i := range ids {
			if !acc.allowsFile(ak, &ids[i]) {
				ids = nil
				break
			}
		}
	}

	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no files"})
		return
	}

	parts := make([]bookPart, len(ids))

	// The first probe of a long book takes a moment; four at a time.
	var wg sync.WaitGroup

	sem := make(chan struct{}, 4)

	for i, id := range ids {
		parts[i].ID = id

		wg.Add(1)

		go func() {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			loc := database.Getdatarow[string](false, "SELECT location FROM audiobook_files WHERE id = ?", &id)
			if loc == "" {
				return
			}

			if p, err := probeMedia(c.Request.Context(), loc); err == nil {
				parts[i].Duration = p.Duration
			}
		}()
	}

	wg.Wait()

	total := 0.0
	for i := range parts {
		parts[i].Start = total
		total += parts[i].Duration
	}

	dbID := database.Getdatarow[uint](false, "SELECT dbaudiobook_id FROM audiobook_files WHERE id = ?", &ids[0])

	chapters := []playChapter{}
	for _, ch := range database.StructscanT[struct {
		Title string `db:"title"`
		Start int64  `db:"start_time_ms"`
	}](false, 100, "SELECT COALESCE(title, '') AS title, COALESCE(start_time_ms, 0) AS start_time_ms FROM dbaudiobook_chapters WHERE dbaudiobook_id = ? ORDER BY position, chapter_number", &dbID) {
		if start := float64(ch.Start) / 1000; start < total {
			chapters = append(chapters, playChapter{Title: ch.Title, Start: start})
		}
	}

	c.JSON(http.StatusOK, gin.H{"parts": parts, "chapters": chapters, "duration": total})
}

// ---- HLS for Apple's WebKit ----
// iPhones and iPads (every browser there is WebKit) and Safari only play
// video they can fetch with byte ranges, which a stream converted on the fly
// can't offer. For them ffmpeg writes an HLS event playlist with 4 s
// segments into hlsRoot, which WebKit plays natively and can seek in.
// Audio stays a plain MP3 stream, which WebKit plays.

const (
	hlsIdle    = 3 * time.Minute // a session nobody asks for is stopped and deleted
	hlsSegment = 4               // seconds
)

var hlsFileRe = regexp.MustCompile(`^(index\.m3u8|seg\d{5}\.ts)$`)

type hlsSession struct {
	owner  string // the account that started it; only it may use and stop it
	kind   string // the file it converts (player kind and id)
	id     string
	dir    string
	cancel context.CancelFunc
	done   chan struct{}
	last   atomic.Int64 // unix seconds of the last request
}

var (
	hlsSessions sync.Map // id -> *hlsSession
	hlsJanitor  sync.Once
)

// hlsRoot is next to the cover cache (a mapped volume in Docker, not the
// container's /tmp).
func hlsRoot() string {
	return filepath.Join(filepath.Dir(filepath.Clean(coverCacheDir())), "hls")
}

// hlsArgs turns the stream arguments into an HLS conversion into dir.
func hlsArgs(path string, pk playKind, p playProbe, quality string, start float64, hw, dir string, sel playSel) ([]string, bool, error) {
	args, usedHW, err := ffmpegArgs(path, pk, p, quality, start, hw, sel)
	if err != nil {
		return nil, false, err
	}

	// Drop the fragmented MP4 output (-f mp4 -movflags ... pipe:1).
	args = args[:len(args)-5]

	// Read the file at twice real time after a minute's head start: the
	// segments stay on disk, so converting far ahead only fills the disk.
	if i := slices.Index(args, "-i"); i >= 0 {
		args = slices.Insert(args, i, "-readrate", "2", "-readrate_initial_burst", "60")
	}

	// Converted video gets a keyframe at every segment start.
	if !slices.Contains(args, "copy") {
		args = append(args, "-force_key_frames", "expr:gte(t,n_forced*"+strconv.Itoa(hlsSegment)+")")
	}

	return append(args, "-f", "hls", "-hls_time", strconv.Itoa(hlsSegment), "-hls_playlist_type", "event",
		"-hls_segment_type", "mpegts", "-hls_segment_filename", filepath.Join(dir, "seg%05d.ts"),
		filepath.Join(dir, "index.m3u8")), usedHW, nil
}

func hlsStop(id string) {
	v, ok := hlsSessions.LoadAndDelete(id)
	if !ok {
		return
	}

	s := v.(*hlsSession)
	s.cancel()
	<-s.done
	os.RemoveAll(s.dir)
}

// hlsStartJanitor removes what an earlier run left and then stops idle
// sessions.
func hlsStartJanitor() {
	hlsJanitor.Do(func() {
		os.RemoveAll(hlsRoot())

		go func() {
			for range time.Tick(30 * time.Second) {
				hlsSessions.Range(func(k, v any) bool {
					if time.Since(time.Unix(v.(*hlsSession).last.Load(), 0)) > hlsIdle {
						hlsStop(k.(string))
					}

					return true
				})
			}
		}()
	})
}

// hlsPlaylistReady waits until the playlist lists a segment, or ffmpeg ended.
func hlsPlaylistReady(dir string, done <-chan struct{}) bool {
	deadline := time.After(45 * time.Second)

	for {
		if b, err := os.ReadFile(filepath.Join(dir, "index.m3u8")); err == nil && strings.Contains(string(b), "#EXTINF") {
			return true
		}

		select {
		case <-done:
			b, err := os.ReadFile(filepath.Join(dir, "index.m3u8"))
			return err == nil && strings.Contains(string(b), "#EXTINF")
		case <-deadline:
			return false
		case <-time.After(150 * time.Millisecond):
		}
	}
}

// HandlePlayerHLSStart starts an HLS conversion of a video
// (/admin/library/play/:kind/:id/hls?q=&start=&prev=) and returns the
// playlist URL once the first segment is ready; prev stops the session the
// player used before.
func HandlePlayerHLSStart(c *gin.Context) {
	path, pk, ok := playFileFor(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	if prev := c.Query("prev"); prev != "" && hlsOwnedBy(c, prev) {
		hlsStop(prev)
	}

	if streamBlocked(accessOf(c).Name, c.Param("kind"), c.Param("id")) {
		c.JSON(http.StatusGone, gin.H{"error": "stopped by an admin"})
		return
	}

	p, err := probeMedia(c.Request.Context(), path)
	if err != nil || pk.Audio || p.VCodec == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no video to convert"})
		return
	}

	ffmpeg, err := tags.ResolveFFmpeg()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	hlsStartJanitor()

	quality := c.DefaultQuery("q", "original")
	start, _ := strconv.ParseFloat(c.Query("start"), 64)

	sel, err := selFromQuery(c, p)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	shown, start := streamStart(c.Request.Context(), path, pk, p, quality, sel, start)

	if !acquireTranscode() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "too many streams are converted right now (player_max_transcodes)"})
		return
	}

	idBytes := make([]byte, 12)
	_, _ = rand.Read(idBytes)
	id := hex.EncodeToString(idBytes)
	dir := filepath.Join(hlsRoot(), id)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		releaseTranscode()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}

	hw := playerHWAccel()

	// Like the stream: a GPU encoder that fails before the first segment is
	// replaced by the CPU.
	for {
		args, usedHW, err := hlsArgs(path, pk, p, quality, start, hw, dir, sel)
		if err != nil {
			releaseTranscode()
			os.RemoveAll(dir)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

			return
		}

		if !usedHW {
			hw = "none"
		}

		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, ffmpeg, args...)
		stderr := &ffmpegStderr{buf: newStreamBuffer(1)}
		cmd.Stderr = stderr

		if err := cmd.Start(); err != nil {
			cancel()
			releaseTranscode()
			os.RemoveAll(dir)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

			return
		}

		s := &hlsSession{owner: accessOf(c).Name, kind: c.Param("kind"), id: c.Param("id"), dir: dir, cancel: cancel, done: make(chan struct{})}
		s.last.Store(time.Now().Unix())

		go func() {
			waitErr := cmd.Wait()
			if waitErr != nil && ctx.Err() == nil {
				logger.Logtype("error", 0).Err(waitErr).Str("file", path).Str("ffmpeg", strings.TrimSpace(stderr.messages())).
					Msg("Player: HLS conversion failed")
			}

			close(s.done)
		}()

		if hlsPlaylistReady(dir, s.done) {
			// The transcode slot is held until the session is stopped.
			go func() { <-s.done; releaseTranscode() }()

			hlsSessions.Store(id, s)
			c.JSON(http.StatusOK, gin.H{"session": id, "url": "/api/admin/library/play/hls/" + id + "/index.m3u8", "start": shown})

			return
		}

		cancel()
		<-s.done

		if hw == "none" {
			releaseTranscode()
			os.RemoveAll(dir)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "the conversion didn't start: " + strings.TrimSpace(stderr.messages())})

			return
		}

		markHWFailed()
		logger.Logtype("warn", 0).Str("hwaccel", hw).Str("ffmpeg", strings.TrimSpace(stderr.messages())).
			Msg("Player: GPU encoding failed, using the CPU (GPU retried in 10 minutes)")

		hw = "none"

		os.RemoveAll(dir)
		_ = os.MkdirAll(dir, 0o755)
	}
}

// HandlePlayerHLSFile serves the playlist and segments of a session
// (/admin/library/play/hls/:sid/:file); file "stop" ends the session.
// hlsOwnedBy reports whether the account of the request started the HLS
// session (admins may use every session); unknown sessions count as owned,
// stopping them does nothing.
func hlsOwnedBy(c *gin.Context, id string) bool {
	v, ok := hlsSessions.Load(id)
	if !ok {
		return true
	}

	acc := accessOf(c)

	return acc.Admin() || strings.EqualFold(v.(*hlsSession).owner, acc.Name)
}

func HandlePlayerHLSFile(c *gin.Context) {
	id, file := c.Param("sid"), c.Param("file")

	if !hlsOwnedBy(c, id) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if file == "stop" {
		hlsStop(id)
		c.Status(http.StatusNoContent)

		return
	}

	v, ok := hlsSessions.Load(id)
	if !ok || !hlsFileRe.MatchString(file) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	s := v.(*hlsSession)
	s.last.Store(time.Now().Unix())

	done, ok := streamGuard(c, s.kind, s.id)
	if !ok {
		return
	}
	defer done()

	if file == "index.m3u8" {
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
		c.Header("Cache-Control", "no-store")
	} else {
		c.Header("Content-Type", "video/mp2t")
	}

	c.File(filepath.Join(s.dir, file))
}
