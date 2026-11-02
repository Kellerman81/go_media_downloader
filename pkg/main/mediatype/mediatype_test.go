package mediatype_test

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/audiobooks"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/books"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/movies"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/music"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/series"
)

// TestCapabilityMatrix pins what each media type supports; searcher,
// structure and the importers branch on these.
func TestCapabilityMatrix(t *testing.T) {
	type caps struct {
		idSearch, season, year, absolute, video, oldPrio, imdb, grouped bool
		table, category, unmatched, files                               string
	}

	want := map[uint]caps{
		config.MediaTypeMovie:     {true, false, true, false, true, true, true, false, "movies", logger.StrMovie, logger.CacheUnmatchedMovie, logger.CacheFilesMovie},
		config.MediaTypeSeries:    {true, true, false, true, true, false, false, false, "series", logger.StrSeries, logger.CacheUnmatchedSeries, logger.CacheFilesSeries},
		config.MediaTypeMusic:     {false, false, false, false, false, false, false, true, "albums", "music", logger.CacheUnmatchedAlbum, logger.CacheFilesAlbum},
		config.MediaTypeAudiobook: {false, false, false, false, false, false, false, true, "audiobooks", "audiobook", logger.CacheUnmatchedAudiobook, logger.CacheFilesAudiobook},
		config.MediaTypeBook:      {false, false, false, false, false, false, false, false, "books", "book", logger.CacheUnmatchedBook, logger.CacheFilesBook},
	}

	for mt, w := range want {
		got := caps{
			mediatype.SupportsIDSearch(mt), mediatype.SupportsSeasonSearch(mt), mediatype.RequiresYearCheck(mt),
			mediatype.SupportsAbsoluteEpisode(mt), mediatype.SupportsVideoFile(mt), mediatype.ShouldCheckOldFilePriority(mt),
			mediatype.IsExternalIDImdb(mt), mediatype.UsesGroupedFileProcessing(mt),
			mediatype.GetTableName(mt), mediatype.GetCategoryName(mt), mediatype.GetCacheUnmatchedKey(mt), mediatype.GetCacheFilesKey(mt),
		}

		if got != w {
			t.Errorf("type %d:\n got %+v\nwant %+v", mt, got, w)
		}

		if mediatype.UsesListNameAsQualityProfile(mt) {
			t.Errorf("type %d uses the list name as quality profile", mt)
		}
	}
}

func TestUnregisteredType(t *testing.T) {
	const unknown = 9999

	if mediatype.Get(unknown) != nil || mediatype.SupportsIDSearch(unknown) || mediatype.GetTableName(unknown) != "" ||
		mediatype.GetCacheFilesKey(unknown) != "" || mediatype.HasConfiguredExtensions(unknown, &config.PathsConfig{}) {
		t.Error("unregistered type returned a value")
	}

	if mediatype.ForType(unknown, func(mediatype.Handler) { t.Error("called for unregistered type") }) {
		t.Error("ForType reported a handler")
	}

	defer func() {
		if recover() == nil {
			t.Error("MustGet did not panic for an unregistered type")
		}
	}()

	mediatype.MustGet(unknown)
}

func TestRegistryIteration(t *testing.T) {
	n := 0

	mediatype.ForEach(func(mediatype.Handler) { n++ })

	if n != 5 {
		t.Errorf("ForEach visited %d handlers, want 5", n)
	}

	if !mediatype.ForType(config.MediaTypeMovie, func(h mediatype.Handler) {
		if h.GetTableName() != "movies" {
			t.Errorf("ForType handed %q", h.GetTableName())
		}
	}) {
		t.Error("ForType(movie) found no handler")
	}

	if mediatype.MustGet(config.MediaTypeSeries) == nil {
		t.Error("MustGet(series) = nil")
	}
}

func TestExtensionChecks(t *testing.T) {
	pc := &config.PathsConfig{
		AllowedVideoExtensions: []string{".mkv"}, AllowedVideoExtensionsLen: 1,
		AllowedVideoExtensionsNoRename: []string{".iso"}, AllowedVideoExtensionsNoRenameLen: 1,
		AllowedAudioExtensions: []string{".flac"}, AllowedAudioExtensionsLen: 1,
		AllowedBookExtensions: []string{".epub"}, AllowedBookExtensionsLen: 1,
		AllowedBookExtensionsNoRename: []string{".pdf"}, AllowedBookExtensionsNoRenameLen: 1,
		AllowedOtherExtensions: []string{".srt"}, AllowedOtherExtensionsLen: 1,
	}

	type res struct{ ok, norename bool }

	check := func(name string, got res, want res) {
		t.Helper()

		if got != want {
			t.Errorf("%s = %+v, want %+v", name, got, want)
		}
	}

	r := func(ok, nr bool) res { return res{ok, nr} }

	check("video .MKV", r(mediatype.CheckVideoExtensions(pc, ".MKV")), res{true, false})
	check("video .iso", r(mediatype.CheckVideoExtensions(pc, ".iso")), res{true, true})
	check("video .avi", r(mediatype.CheckVideoExtensions(pc, ".avi")), res{false, false})
	check("audio .flac", r(mediatype.CheckAudioExtensions(pc, ".flac")), res{true, false})
	check("audio .mp3", r(mediatype.CheckAudioExtensions(pc, ".mp3")), res{false, false})
	check("book .pdf", r(mediatype.CheckBookExtensions(pc, ".pdf")), res{true, true})
	check("other .srt", r(mediatype.CheckOtherExtensions(pc, ".srt")), res{true, false})
	check("other .nfo", r(mediatype.CheckOtherExtensions(pc, ".nfo")), res{false, false})

	// Nothing configured: everything passes and nothing is renamed.
	empty := &config.PathsConfig{}
	check("empty video", r(mediatype.CheckVideoExtensions(empty, ".x")), res{true, true})
	check("empty audio", r(mediatype.CheckAudioExtensions(empty, ".x")), res{true, true})
	check("empty book", r(mediatype.CheckBookExtensions(empty, ".x")), res{true, true})
	check("empty other", r(mediatype.CheckOtherExtensions(empty, ".x")), res{true, true})

	// Per type, the handler picks the matching list; checkother uses the other list.
	check("movie .mkv", r(mediatype.CheckExtensions(config.MediaTypeMovie, false, pc, ".mkv")), res{true, false})
	check("music .mkv", r(mediatype.CheckExtensions(config.MediaTypeMusic, false, pc, ".mkv")), res{false, false})
	check("audiobook .flac", r(mediatype.CheckExtensions(config.MediaTypeAudiobook, false, pc, ".flac")), res{true, false})
	check("book .epub", r(mediatype.CheckExtensions(config.MediaTypeBook, false, pc, ".epub")), res{true, false})
	check("series other .srt", r(mediatype.CheckExtensions(config.MediaTypeSeries, true, pc, ".srt")), res{true, false})
	check("unknown type", r(mediatype.CheckExtensions(9999, false, pc, ".mkv")), res{false, false})

	if !mediatype.HasConfiguredExtensions(config.MediaTypeMovie, pc) || mediatype.HasConfiguredExtensions(config.MediaTypeMusic, empty) {
		t.Error("HasConfiguredExtensions gave a wrong answer")
	}
}

func TestSchedulerJobNames(t *testing.T) {
	mediatype.ForEach(func(h mediatype.Handler) {
		seen := map[string]bool{}

		for _, pair := range h.GetSchedulerJobNames() {
			if pair[0] == "" || pair[1] == "" {
				t.Errorf("%s: empty job name in %v", h.GetTableName(), pair)
			}

			if seen[pair[0]] {
				t.Errorf("%s: duplicate scheduler job %q", h.GetTableName(), pair[0])
			}

			seen[pair[0]] = true
		}

		if !seen[logger.StrDataFull] {
			t.Errorf("%s: no %q job", h.GetTableName(), logger.StrDataFull)
		}
	})
}
