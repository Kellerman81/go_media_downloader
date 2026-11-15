package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/gin-gonic/gin"
)

// TestWizardRecommendationsValid: every suggestion works as it is.
func TestWizardRecommendationsValid(t *testing.T) {
	types := map[string]string{"movies": "movie", "series": "series", "music": "music", "audiobooks": "audiobook", "books": "book"}

	for _, d := range wizardMediaGroupDefs {
		for _, l := range wizardLangs {
			req, rej := wizardRecommendedRegex(d.key, l[0])
			if bad := wizardCheckPatterns(append(req, rej...)); len(bad) > 0 {
				t.Errorf("%s/%s: invalid patterns %v", d.key, l[0], bad)
			}

			if out, err := structure.TestParsertype(wizardRecommendedNaming(d.key, l[0]), types[d.key]); err != nil || out == "" {
				t.Errorf("%s/%s naming: %v %q", d.key, l[0], err, out)
			}
		}

		s := wizardRecommendedSchedule(d.key)
		for _, f := range wizardScheduleFields {
			if _, err := wizardCronParser.Parse(s.get(f.key)); err != nil {
				t.Errorf("%s %s: %v", d.key, f.key, err)
			}
		}
	}
}

// TestWizardRecommendedRegexLanguages checks the language rules on real
// release names.
func TestWizardRecommendedRegexLanguages(t *testing.T) {
	passes := func(key, lang, release string) bool {
		// Like the searcher: one required pattern must match, none rejected.
		req, rej := wizardRecommendedRegex(key, lang)
		if len(req) > 0 {
			found := false
			for _, p := range req {
				found = found || regexp.MustCompile(p).MatchString(release)
			}

			if !found {
				return false
			}
		}

		for _, p := range rej {
			if regexp.MustCompile(p).MatchString(release) {
				return false
			}
		}

		return true
	}

	for _, tc := range []struct {
		key, lang, release string
		want               bool
	}{
		{"movies", "en", "Inception.2010.1080p.BluRay.x264-GRP", true},
		{"movies", "en", "Inception.2010.GERMAN.1080p.BluRay.x264-GRP", false},
		{"movies", "de", "Inception.2010.GERMAN.1080p.BluRay.x264-GRP", true},
		{"movies", "de", "Inception.2010.1080p.BluRay.x264-GRP", false},
		{"movies", "any", "Inception.2010.FRENCH.1080p.BluRay.x264-GRP", true},
		{"movies", "en", "Inception.2010.FRENCH.1080p.BluRay.x264-GRP", false},
		{"movies", "any", "Inception.2010.CAM.x264-GRP", false},
		{"movies", "any", "Inception.2010.Sample.1080p.x264-GRP", false},
		{"movies", "any", "Some.Show.S01E02.1080p.x264-GRP", false},
		{"series", "any", "Some.Show.S01E02.1080p.x264-GRP", true},
		{"series", "de", "Some.Show.S01E02.German.1080p.WEB.x264-GRP", true},
		{"music", "any", "Artist-Album-2020-FLAC", true},
		{"music", "any", "Karaoke.Hits.Vol.3-2020", false},
	} {
		if got := passes(tc.key, tc.lang, tc.release); got != tc.want {
			t.Errorf("%s/%s %s: passes=%v, want %v", tc.key, tc.lang, tc.release, got, tc.want)
		}
	}
}

func TestWizardBuildScheduler(t *testing.T) {
	s := wizardRecommendedSchedule("music")

	music := wizardBuildScheduler("music", "M", s)
	if music.CronIndexerMissingTitle != s.Missing || music.CronIndexerMissing != "" || music.CronIndexerRssArtists == "" || music.CronIndexerRssAuthors != "" {
		t.Errorf("music: %+v", music)
	}

	movies := wizardBuildScheduler("movies", "F", wizardRecommendedSchedule("movies"))
	if movies.CronIndexerMissing == "" || movies.CronIndexerMissingTitle != "" || movies.CronImdb == "" {
		t.Errorf("movies: %+v", movies)
	}

	series := wizardBuildScheduler("series", "S", wizardRecommendedSchedule("series"))
	if series.CronIndexerRssSeasons == "" || series.CronIndexerMissingTitle == "" {
		t.Errorf("series: %+v", series)
	}
}

func TestWizardSaveFinalizeRecommended(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})
	t.Chdir(t.TempDir()) // the config backup goes to ./backup

	post := func(form url.Values) string {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		state := wizardState{Groups: map[string]wizardGroupState{
			"movies": {Enabled: true, Name: "FilmDE", LibraryPath: "alpha"},
			"music":  {Enabled: true, Name: "Songs", LibraryPath: "alpha"},
		}}

		return wizardSaveFinalizeStep(c, state)
	}

	// Invalid input: nothing is saved.
	msg := post(url.Values{"wizard_rec_movies_shown": {"1"}, "wizard_rec_movies_lang": {"de"},
		"wizard_rec_movies_rejected": {"(unclosed"}, "wizard_rec_movies_cron_rss": {"every now and then"}})
	if !strings.Contains(msg, "Nothing was saved") || !strings.Contains(msg, "(unclosed") || !strings.Contains(msg, "every now and then") {
		t.Fatalf("invalid: %s", msg)
	}

	if _, ok := config.GetSettingsQualityOk("FilmDE-Quality"); ok {
		t.Fatal("saved despite errors")
	}

	// Edited movie settings; music not shown: its recommendation.
	msg = post(url.Values{"wizard_rec_movies_shown": {"1"}, "wizard_rec_movies_lang": {"de"},
		"media_main_movies_wizard_naming": {"{{.Dbmovie.Title}}"}, "wizard_rec_movies_required": {" (?i)german "},
		"wizard_rec_movies_cron_rss": {"0 */20 * * * *"}, "wizard_rec_movies_cron_scan": {""}})
	if !strings.Contains(msg, "saved") {
		t.Fatalf("save: %s", msg)
	}

	film := config.GetSettingsMedia("movie_FilmDE")
	if film == nil || film.Naming != "{{.Dbmovie.Title}}" || film.TemplateScheduler != "FilmDE-Schedule" {
		t.Fatalf("movie group: %+v", film)
	}

	var regex *config.RegexConfig

	for _, r := range config.GetSettingsRegexAll() {
		if r.Name == "FilmDE-Regex" {
			regex = &r
		}
	}

	if regex == nil || len(regex.Required) != 1 || regex.Required[0] != "(?i)german" || len(regex.Rejected) != 0 {
		t.Fatalf("regex: %+v", regex)
	}

	if s := config.GetSettingsScheduler("FilmDE-Schedule"); s == nil || s.CronIndexerRss != "0 */20 * * * *" || s.CronScanData != "" || s.CronIndexerMissing == "" {
		t.Fatalf("schedule: %+v", s)
	}

	if q, _ := config.GetSettingsQualityOk("FilmDE-Quality"); !q.BackupSearchForAlternateTitle {
		t.Fatalf("quality for German: %+v", q)
	}

	songs := config.GetSettingsMedia("music_Songs")
	if songs == nil || songs.Naming != wizardRecommendedNaming("music", "any") {
		t.Fatalf("music group: %+v", songs)
	}

	if s := config.GetSettingsScheduler("Songs-Schedule"); s == nil || s.CronIndexerRssArtists == "" {
		t.Fatalf("music schedule: %+v", s)
	}

	// Organizing on, searches in batches, metadata in the group's language.
	if !film.Structure || film.SearchmissingIncremental != 100 || film.SearchupgradeIncremental != 20 || film.MetadataLanguage != "de" {
		t.Errorf("movie group settings: structure %v incremental %d/%d language %q",
			film.Structure, film.SearchmissingIncremental, film.SearchupgradeIncremental, film.MetadataLanguage)
	}

	if !songs.Structure || songs.SearchmissingIncremental != 50 || songs.MetadataLanguage != "" || len(songs.Data) != 1 {
		t.Fatalf("music group settings: %+v", songs)
	}

	if d := songs.Data[0]; !d.EnableUnpacking || !d.WriteRenameLog || !d.EmbedArt || !d.AllowAlternativeReleases ||
		d.PerTrackToleranceSeconds != 10 || d.PerTrackToleranceSecondsMax != 30 || !d.DiscoverSeriesAlbums ||
		strings.Join(d.AllowedReleaseTypes, ",") != "Album,Compilation" || d.AllowCountMismatchIfTotalMatch {
		t.Errorf("music library path: %+v", d)
	}
}

func TestWizardSaveFinalizeAudioOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})
	t.Chdir(t.TempDir()) // the config backup goes to ./backup

	// Shown and edited: art off, tolerance out of range.
	form := url.Values{"wizard_rec_audiobooks_shown": {"1"}, "wizard_rec_audiobooks_lang": {"en"},
		"wizard_rec_audiobooks_renamelog": {"1"}, "wizard_rec_audiobooks_altreleases": {"1"},
		"wizard_rec_audiobooks_countmismatch": {"1"}, "wizard_rec_audiobooks_tolerance": {"500"}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	state := wizardState{Groups: map[string]wizardGroupState{
		"audiobooks": {Enabled: true, Name: "Books", LibraryPath: "alpha", ImportPath: "alpha"},
	}}
	if msg := wizardSaveFinalizeStep(c, state); !strings.Contains(msg, "saved") {
		t.Fatalf("save: %s", msg)
	}

	g := config.GetSettingsMedia("audiobook_Books")
	if g == nil || len(g.Data) != 1 || len(g.DataImport) != 1 {
		t.Fatalf("group: %+v", g)
	}

	if d := g.Data[0]; !d.WriteRenameLog || d.EmbedArt || !d.AllowAlternativeReleases || !d.AllowCountMismatchIfTotalMatch ||
		d.ReplaceEmbeddedChapters || d.PerTrackToleranceSeconds != 120 || !d.SkipSeriesTrackMatch || !d.EnableUnpacking {
		t.Errorf("library path: %+v", d)
	}

	if i := g.DataImport[0]; !i.AllowCountMismatchIfTotalMatch || i.ReplaceEmbeddedChapters || i.PerTrackToleranceSeconds != 120 || !i.EnableUnpacking {
		t.Errorf("import path: %+v", i)
	}

	// The checkboxes and the tolerance are on the Finalize page.
	html := renderComponentToString(renderWizardAudioOpts("audiobooks", "wizard_rec_audiobooks_"))
	for _, want := range []string{`name="wizard_rec_audiobooks_chapters"`, `name="wizard_rec_audiobooks_tolerance"`, `value="3"`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s", want)
		}
	}

	if renderWizardAudioOpts("movies", "x_") != nil {
		t.Error("audio options for movies")
	}
}

func TestWizardRecommendedCategories(t *testing.T) {
	for _, tc := range []struct{ key, lang, cats, dlcat string }{
		{"movies", "de", "2010", "MoviesDE"},
		{"movies", "en", "2030,2035,2040,2045", "Movies"},
		{"movies", "any", "2000", "Movies"},
		{"series", "de", "5020", "SeriesDE"},
		{"series", "any", "5000", "Series"},
		{"music", "any", "3010,3040", "Music"}, // not 3030: audiobooks
		{"audiobooks", "any", "3030", "Audiobooks"},
		{"books", "any", "7020", "Books"},
	} {
		if got := wizardRecommendedCategories(tc.key, tc.lang); got != tc.cats || !wizardCheckCategories(got) {
			t.Errorf("%s/%s categories: %q", tc.key, tc.lang, got)
		}

		if got := wizardRecommendedDlCategory(tc.key, tc.lang); got != tc.dlcat {
			t.Errorf("%s/%s client category: %q", tc.key, tc.lang, got)
		}
	}

	for _, bad := range []string{"2000,", "abc", "20 00"} {
		if wizardCheckCategories(bad) {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestWizardNotification(t *testing.T) {
	csv := wizardNotification("series", "log", "csv", "added_data")
	if csv.Title != "" || !strings.Contains(csv.Message, "{{.Season}};{{.Episode}}") || csv.MapNotification != "log" {
		t.Errorf("csv: %+v", csv)
	}

	push := wizardNotification("movies", "pushover", "pushover", "upgraded_data")
	if push.Title != "Replaced Movie in {{.Configuration}}" || push.ReplacedPrefix != "replaced" || push.Event != "upgraded_data" {
		t.Errorf("pushover: %+v", push)
	}

	dl := wizardNotification("music", "pushover", "pushover", "added_download")
	if !strings.Contains(dl.Message, "{{.Nzb.NZB.Title}}") {
		t.Errorf("download: %+v", dl)
	}
}

// TestWizardSaveFinalizeDownloadAndNotify: categories and notifications reach
// the config, a backup is made first, and the review shows the plan
// without writing.
func TestWizardSaveFinalizeDownloadAndNotify(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useTestConfig(t, config.MainConfig{
		Media:        config.MediaConfig{Movies: mediaGroup("EN")},
		Notification: []config.NotificationConfig{{Name: "phone", NotificationType: "pushover"}},
	})
	t.Chdir(t.TempDir())

	request := func(form url.Values) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		return c
	}
	state := wizardState{Groups: map[string]wizardGroupState{"series": {Enabled: true, Name: "TV", LibraryPath: "alpha"}}}
	form := url.Values{"wizard_rec_series_shown": {"1"}, "wizard_rec_series_lang": {"de"},
		"wizard_rec_series_cats": {"5020, 5030"}, "wizard_rec_series_dlcat": {" Fernsehen "},
		"wizard_rec_series_notify": {"phone", "unknown"}, "wizard_rec_series_ev_added_data": {"1"}}

	// Bad categories: nothing planned.
	bad := url.Values{"wizard_rec_series_shown": {"1"}, "wizard_rec_series_cats": {"TV"}}
	if plan, msg := wizardPlanFinalize(request(bad), state); plan != nil || !strings.Contains(msg, "indexer categories") {
		t.Fatalf("bad categories: %d %s", len(plan), msg)
	}

	plan, msg := wizardPlanFinalize(request(form), state)
	if len(plan) != 1 {
		t.Fatalf("plan: %s", msg)
	}

	review := renderComponentToString(renderWizardReview(plan, "tok"))
	for _, want := range []string{"TV-Quality", "categories 5020,5030", "category Fernsehen", "phone (added_data)", "Write configuration"} {
		if !strings.Contains(review, want) {
			t.Errorf("review lacks %q", want)
		}
	}

	if config.GetSettingsMedia("serie_TV") != nil {
		t.Fatal("the plan wrote the group")
	}

	if msg := wizardSaveFinalizeStep(request(form), state); !strings.Contains(msg, "backed up to") {
		t.Fatalf("save: %s", msg)
	}

	if backups, _ := filepath.Glob("backup/config.toml.*"); len(backups) != 1 {
		t.Errorf("backups: %v", backups)
	}

	q, _ := config.GetSettingsQualityOk("TV-Quality")
	if len(q.Indexer) != 1 || q.Indexer[0].CategoriesIndexer != "5020,5030" || q.Indexer[0].CategoryDownloader != "Fernsehen" {
		t.Fatalf("quality indexer: %+v", q.Indexer)
	}

	g := config.GetSettingsMedia("serie_TV")
	if g == nil || len(g.Notification) != 1 || g.Notification[0].MapNotification != "phone" || g.Notification[0].Event != "added_data" {
		t.Fatalf("notifications: %+v", g)
	}
}
