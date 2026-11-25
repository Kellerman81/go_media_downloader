package api

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// Transcoding for Jellyfin apps that can't play a file as it is: the
// playback info compares the file with the app's device profile (direct
// play containers and codecs) and its bitrate limit; when it doesn't fit,
// the app gets a transcoding URL. That starts the Library player's HLS
// conversion (player_video_profiles, player_max_transcodes); its segments
// are served under the session ID, which apps load without their login.

// jfDeviceProfile is the part of an app's device profile used here.
type jfDeviceProfile struct {
	DirectPlayProfiles []struct {
		Container  string `json:"Container"`
		VideoCodec string `json:"VideoCodec"`
		AudioCodec string `json:"AudioCodec"`
		Type       string `json:"Type"`
	} `json:"DirectPlayProfiles"`
}

// jfPlaybackRequest is the body of a playback info request.
type jfPlaybackRequest struct {
	DeviceProfile       *jfDeviceProfile `json:"DeviceProfile"`
	MaxStreamingBitrate int64            `json:"MaxStreamingBitrate"`
	EnableDirectPlay    *bool            `json:"EnableDirectPlay"`
	EnableDirectStream  *bool            `json:"EnableDirectStream"`
	AudioStreamIndex    *int             `json:"AudioStreamIndex"`
	StartTimeTicks      int64            `json:"StartTimeTicks"`
}

// listHas reports whether a comma list holds a value; an empty list holds
// everything.
func listHas(list, v string) bool {
	if strings.TrimSpace(list) == "" {
		return true
	}

	for s := range strings.SplitSeq(strings.ToLower(list), ",") {
		if strings.TrimSpace(s) == strings.ToLower(v) {
			return true
		}
	}

	return false
}

// jfContainerNames are the other names apps give a container.
var jfContainerNames = map[string][]string{"mkv": {"mkv", "matroska", "webm"}, "m4v": {"m4v", "mp4", "mov"}, "mp4": {"mp4", "m4v", "mov"}}

// directPlayable reports whether the device profile plays a video file as
// it is (no profile: yes).
func directPlayable(dp *jfDeviceProfile, container string, p playProbe) bool {
	if dp == nil || len(dp.DirectPlayProfiles) == 0 {
		return true
	}

	names := jfContainerNames[container]
	if names == nil {
		names = []string{container}
	}

	for _, pr := range dp.DirectPlayProfiles {
		if pr.Type != "" && !strings.EqualFold(pr.Type, "Video") {
			continue
		}

		containerOK := false

		for _, n := range names {
			if listHas(pr.Container, n) {
				containerOK = true
			}
		}

		if containerOK && listHas(pr.VideoCodec, p.VCodec) && (p.ACodec == "" || listHas(pr.AudioCodec, p.ACodec)) {
			return true
		}
	}

	return false
}

// jfQuality picks the conversion quality for a bitrate limit (bits per
// second, 0 none): "original" (the first profile, or the video copied when
// a browser plays it) without a limit, else the best profile within it.
func jfQuality(p playProbe, maxBps int64) string {
	if maxBps <= 0 || int64(p.Kbps)*1000 <= maxBps {
		return "original"
	}

	profiles := videoProfiles()
	best, bestKbps := "", 0

	for _, vp := range profiles {
		kbps := vp.VKbps + vp.AKbps
		if int64(kbps)*1000 <= maxBps && kbps > bestKbps && (p.Height == 0 || vp.Height <= max(p.Height, 240)) {
			best, bestKbps = vp.Name, kbps
		}
	}

	if best == "" && len(profiles) > 0 {
		// None fits: the smallest.
		low := profiles[0]
		for _, vp := range profiles {
			if vp.VKbps < low.VKbps {
				low = vp
			}
		}

		best = low.Name
	}

	if best == "" {
		return "original"
	}

	return best
}

// jfTranscodeInfo sets how a video source plays for an app: as it is, or
// through the transcoding URL.
func jfTranscodeInfo(c *gin.Context, ms *jfMediaSource, itemID, playSession string, p playProbe, req *jfPlaybackRequest) {
	if ms.VideoType == "" {
		return
	}

	direct := directPlayable(req.DeviceProfile, ms.Container, p) &&
		(req.MaxStreamingBitrate <= 0 || int64(p.Kbps)*1000 <= req.MaxStreamingBitrate) &&
		(req.EnableDirectPlay == nil || *req.EnableDirectPlay)

	ms.SupportsTranscoding = true

	// Like Jellyfin, the transcoding URL only comes with a file the app
	// can't play: the web client plays every source with one through its
	// HLS player, so a direct file fails there and is converted from the
	// start instead of resuming (an app that fails to play asks again
	// without direct play).
	if direct {
		return
	}

	q := url.Values{
		"MediaSourceId": {ms.ID}, "PlaySessionId": {playSession}, "api_key": {jfClientOf(c).Token},
		"VideoQuality": {jfQuality(p, req.MaxStreamingBitrate)},
	}

	if req.AudioStreamIndex != nil {
		q.Set("AudioStreamIndex", strconv.Itoa(*req.AudioStreamIndex))
	}

	ms.TranscodingURL = "/videos/" + itemID + "/master.m3u8?" + q.Encode()
	ms.TranscodingSubProtocol, ms.TranscodingContainer = "hls", "ts"
	ms.SupportsDirectPlay, ms.SupportsDirectStream = false, false
}

// jfHLSSessions are the conversions of the apps' player sessions.
var jfHLSSessions sync.Map // PlaySessionId -> HLS session ID

// jfMaster starts the conversion of an item's file and answers its master
// playlist (/videos/{id}/master.m3u8).
func jfMaster(c *gin.Context, p map[string]string) {
	if _, ok := jfLoad(c, jfGroups(c), p["id"]); !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	f, ok := jfFileFor(p["id"], jfParam(c, "mediaSourceId"))
	if !ok || (f.Kind != "movie" && f.Kind != "episode") {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	acc := accessOf(c)
	fileID := strconv.FormatUint(f.ID, 10)

	if streamBlocked(acc.Name, f.Kind, fileID) {
		c.AbortWithStatus(http.StatusGone)
		return
	}

	probe, err := probeMedia(c.Request.Context(), f.Location)
	if err != nil || probe.VCodec == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	sel := defaultSel

	// Stream indexes as jfStreamsOf numbers them: the video first.
	if a, err := strconv.Atoi(jfParam(c, "audioStreamIndex")); err == nil && a >= 1 && a-1 < max(len(probe.AudioTracks), 1) {
		sel.Audio = a - 1
	}

	quality := jfParam(c, "videoQuality")
	if quality == "" {
		quality = "original"
	}

	start := 0.0
	if ticks, err := strconv.ParseInt(jfParam(c, "startTimeTicks"), 10, 64); err == nil && ticks > 0 {
		start = float64(ticks) / 1e7
	}

	// A new start of the same player session replaces its conversion.
	playSession := jfParam(c, "playSessionId")
	if old, ok := jfHLSSessions.Load(playSession); ok && playSession != "" {
		hlsStop(old.(string))
	}

	id, _, status, err := startHLS(c.Request.Context(), acc.Name, f.Kind, fileID, f.Location, playKinds[f.Kind], probe, quality, start, sel)
	if err != nil {
		c.String(status, err.Error())
		return
	}

	if playSession != "" {
		jfHLSSessions.Store(playSession, id)
	}

	kbps := probe.Kbps
	for _, vp := range videoProfiles() {
		if vp.Name == quality {
			kbps = vp.VKbps + vp.AKbps
		}
	}

	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/vnd.apple.mpegurl", []byte("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-STREAM-INF:BANDWIDTH="+
		strconv.Itoa(max(kbps, 1)*1000)+"\n/jellyfin/hls/"+id+"/index.m3u8\n"))
}

// jfHLSFile serves a file of a conversion (/hls/{sid}/{file}): the
// session ID stands for the login.
func jfHLSFile(c *gin.Context, p map[string]string) {
	v, ok := hlsSessions.Load(p["sid"])
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.Set("session", &Session{UserID: v.(*hlsSession).owner})
	c.Params = gin.Params{{Key: "sid", Value: p["sid"]}, {Key: "file", Value: p["file"]}}
	HandlePlayerHLSFile(c)
}

// jfStopTranscode stops the conversion of a player session.
func jfStopTranscode(playSession string) {
	if playSession == "" {
		return
	}

	if id, ok := jfHLSSessions.LoadAndDelete(playSession); ok {
		go hlsStop(id.(string))
	}
}

func init() {
	for _, p := range []string{"videos/{id}/master.m3u8", "videos/{id}/main.m3u8"} {
		jfRoute2(http.MethodGet, p, false, jfMaster)
	}

	jfRoute2(http.MethodGet, "hls/{sid}/{file}", true, jfHLSFile)
	jfRoute2(http.MethodDelete, "videos/activeencodings", false, func(c *gin.Context, _ map[string]string) {
		jfStopTranscode(jfParam(c, "playSessionId"))
		c.Status(http.StatusNoContent)
	})
}

// jfBitrateMaxBytes caps the speed test.
const jfBitrateMaxBytes = 50 << 20

// jfBitrateTest sends size bytes, which apps (Swiftfin with automatic
// quality) time to pick the bitrate (/Playback/BitrateTest?size=).
func jfBitrateTest(c *gin.Context, _ map[string]string) {
	size := min(max(jfInt(c, "size"), 0), jfBitrateMaxBytes)
	if size == 0 {
		size = 1 << 20
	}

	c.Header("Content-Length", strconv.Itoa(size))
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)

	buf := make([]byte, 64<<10)
	for size > 0 {
		n, err := c.Writer.Write(buf[:min(size, len(buf))])
		if err != nil {
			return
		}

		size -= n
	}
}

func init() {
	jfRoute2(http.MethodGet, "playback/bitratetest", false, jfBitrateTest)
}
