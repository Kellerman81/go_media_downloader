package lastfm

import (
	"encoding/json"
	"testing"
)

func TestBestImage(t *testing.T) {
	imgs := []lfmImage{{URL: "s", Size: "small"}, {URL: "", Size: "extralarge"}, {URL: "l", Size: "large"}}
	if got := bestImage(imgs); got != "l" {
		t.Errorf("bestImage = %q, want the large one (empty extralarge skipped)", got)
	}

	if got := bestImage([]lfmImage{{URL: "x", Size: "mega"}}); got != "x" {
		t.Errorf("unknown size fallback = %q", got)
	}

	if got := bestImage(nil); got != "" {
		t.Errorf("no images = %q", got)
	}
}

func TestConvertCharts(t *testing.T) {
	artists := convertChartArtists([]lfmChartArtist{
		{Name: "Queen", MBID: "q", Playcount: "1000", Listeners: "n/a", Image: []lfmImage{{URL: "img", Size: "medium"}}},
		{Name: "ABBA"},
	})
	if len(artists) != 2 || artists[0].Rank != 1 || artists[1].Rank != 2 || artists[0].Playcount != 1000 ||
		artists[0].Listeners != 0 || artists[0].ImageURL != "img" {
		t.Errorf("artists = %+v", artists)
	}

	var track lfmChartTrack
	track.Name = "Bohemian Rhapsody"
	track.Artist.Name = "Queen"
	track.Listeners = "42"

	tracks := convertChartTracks([]lfmChartTrack{track})
	if len(tracks) != 1 || tracks[0].Artist != "Queen" || tracks[0].Listeners != 42 || tracks[0].Rank != 1 {
		t.Errorf("tracks = %+v", tracks)
	}

	var album lfmTagAlbum
	album.Name = "A Night at the Opera"
	album.Artist.Name = "Queen"

	albums := convertTagAlbums([]lfmTagAlbum{album})
	if len(albums) != 1 || albums[0].Name != "A Night at the Opera" || albums[0].Artist != "Queen" {
		t.Errorf("albums = %+v", albums)
	}
}

func TestLastFMLenientJSON(t *testing.T) {
	var v struct {
		Tags     lfmTagList  `json:"tags"`
		Duration lfmDuration `json:"duration"`
	}

	// Last.fm sends "" for an empty tag list and durations as string or number.
	for _, in := range []string{`{"tags":"","duration":"327"}`, `{"tags":{"tag":[{"name":"rock"}]},"duration":327}`} {
		v.Tags, v.Duration = lfmTagList{}, 0
		if err := json.Unmarshal([]byte(in), &v); err != nil {
			t.Fatalf("%s: %v", in, err)
		}

		if v.Duration != 327 {
			t.Errorf("%s: duration = %d", in, v.Duration)
		}
	}

	if len(v.Tags.Tag) != 1 || v.Tags.Tag[0].Name != "rock" {
		t.Errorf("tags = %+v", v.Tags)
	}

	if err := json.Unmarshal([]byte(`{"duration":"n/a"}`), &v); err != nil || v.Duration != 0 {
		t.Errorf("bad duration = %d, %v", v.Duration, err)
	}
}
