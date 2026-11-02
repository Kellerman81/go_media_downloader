package deezer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

func TestDeezerErrorPayloadWithStatus200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/album/1" {
			_, _ = w.Write([]byte(`{"error":{"type":"DataException","message":"no data","code":800}}`))
			return
		}

		_, _ = w.Write([]byte(`{"error":{"type":"Exception","message":"Quota limit exceeded","code":4}}`))
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL})

	_, err := p.GetAlbumByID(context.Background(), 1)
	if !errors.Is(err, logger.ErrNotFound) {
		t.Errorf("GetAlbumByID err = %v, want ErrNotFound", err)
	}

	if _, err := p.SearchAlbums(context.Background(), "x", 1); err == nil {
		t.Error("SearchAlbums: expected error for error payload")
	}
}

func TestGetAlbumByIDUsesTracksEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/album/5":
			_, _ = w.Write([]byte(`{"id":5,"title":"A","tracks":{"data":[{"title":"x"},{"title":"y"},{"title":"z"}]}}`))
		case r.URL.Path == "/album/5/tracks" && r.URL.Query().Get("index") == "":
			next := "http://" + r.Host + "/album/5/tracks?limit=500&index=2"
			_, _ = w.Write([]byte(`{"data":[{"title":"x","track_position":1,"disk_number":1,"isrc":"I1"},{"title":"y","track_position":1,"disk_number":2}],"total":3,"next":"` + next + `"}`))
		case r.URL.Path == "/album/5/tracks":
			_, _ = w.Write([]byte(`{"data":[{"title":"z","track_position":2,"disk_number":2}],"total":3}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL})

	d, err := p.GetAlbumByID(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}

	if len(d.Tracks) != 3 || d.Tracks[0].ISRC != "I1" ||
		d.Tracks[2].DiscNumber != 2 || d.Tracks[2].TrackNumber != 2 || d.Tracks[1].DiscNumber != 2 {
		t.Errorf("tracks = %+v", d.Tracks)
	}
}
