package utils

import (
	"context"
	"errors"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
)

func TestListJobEnabled(t *testing.T) {
	tests := []struct {
		name string
		list config.MediaListsConfig
		want bool
	}{
		{"disabled media list", config.MediaListsConfig{Enabled: false, CfgList: &config.ListsConfig{Enabled: true}}, false},
		{"no template list", config.MediaListsConfig{Enabled: true}, false},
		{"disabled template list", config.MediaListsConfig{Enabled: true, CfgList: &config.ListsConfig{Enabled: false}}, false},
		{"enabled", config.MediaListsConfig{Enabled: true, CfgList: &config.ListsConfig{Enabled: true}}, true},
	}

	for _, tt := range tests {
		if got := listJobEnabled(&tt.list); got != tt.want {
			t.Errorf("%s: listJobEnabled = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestRunjobAllListsSkipsDisabledAndHonoursCancel(t *testing.T) {
	cfgp := &config.MediaTypeConfig{
		Lists: []config.MediaListsConfig{
			{Name: "a", Enabled: false, CfgList: &config.ListsConfig{Enabled: true}},
			{Name: "b", Enabled: true},
		},
	}

	// Every list is skipped, so no per-list job (and no DB access) runs.
	if err := runjobAllLists(context.Background(), "checkmissing", cfgp, 0); err != nil {
		t.Fatalf("all lists disabled: got %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := runjobAllLists(ctx, "checkmissing", cfgp, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context: got %v, want context.Canceled", err)
	}
}

func TestMBTitleCompatible(t *testing.T) {
	tests := []struct {
		local, mb string
		want      bool
	}{
		{"Greatest Hits", "Greatest Hits", true},
		{"Greatest Hits", "greatest hits (Deluxe Edition)", true},
		{"Album [Remastered]", "Album", true},
		{"Greatest Hits", "Greatest Hits II", false},
		{"Greatest Hits", "Greatest Hits Live", false},
		{"", "Anything", false},
	}

	for _, tt := range tests {
		if got := mbTitleCompatible(tt.local, tt.mb); got != tt.want {
			t.Errorf("mbTitleCompatible(%q, %q) = %v; want %v", tt.local, tt.mb, got, tt.want)
		}
	}
}

// Filename fallback numbers per disc, so a two-disc album sorted by name
// validates instead of putting disc 2 at 4..6.
func TestVerifyAndSortTracksFilenameFallbackPerDisc(t *testing.T) {
	album := &parser_v2.AlbumInfo{Tracks: []parser_v2.TrackInfo{
		{Filepath: "/a/CD2/b.flac", DiscNumber: 2, TrackNumber: 9},
		{Filepath: "/a/CD1/a.flac", DiscNumber: 1, TrackNumber: 7},
		{Filepath: "/a/CD1/b.flac", DiscNumber: 1, TrackNumber: 7},
		{Filepath: "/a/CD2/a.flac", DiscNumber: 2, TrackNumber: 9},
	}}

	if !verifyAndSortTracks(album) {
		t.Fatal("verifyAndSortTracks = false; want per-disc positional numbering to validate")
	}

	want := []struct{ disc, track int }{{1, 1}, {1, 2}, {2, 1}, {2, 2}}
	for i, w := range want {
		if album.Tracks[i].DiscNumber != w.disc || album.Tracks[i].TrackNumber != w.track {
			t.Errorf("track %d = disc %d track %d; want %d/%d", i,
				album.Tracks[i].DiscNumber, album.Tracks[i].TrackNumber, w.disc, w.track)
		}
	}
}

// When no strategy validates, the tagged numbers must survive - the music path
// imports anyway and would otherwise store positional guesses.
func TestVerifyAndSortTracksRestoresTaggedNumbers(t *testing.T) {
	album := &parser_v2.AlbumInfo{Tracks: []parser_v2.TrackInfo{
		{Filepath: "/a/x.flac", DiscNumber: 1, TrackNumber: 3},
		{Filepath: "/a/y.flac", DiscNumber: 1, TrackNumber: 3},
		{Filepath: "/a/z.flac", DiscNumber: 0, TrackNumber: 3},
	}}

	if verifyAndSortTracks(album) {
		t.Fatal("verifyAndSortTracks = true; want false for duplicate tagged numbers")
	}

	for i := range album.Tracks {
		if album.Tracks[i].TrackNumber != 3 {
			t.Errorf("track %d number = %d; want the tagged 3", i, album.Tracks[i].TrackNumber)
		}
	}
}
