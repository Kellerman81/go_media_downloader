package metadata

import (
	"context"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// Translations of the metadata texts (database.SetTranslation) into the
// metadata_language of the media groups of a type, fetched with the
// metadata of an entry. A text the source has no translation of, or only
// the original, gets no row: the pages show the original then.

// TranslateMovie fetches a movie's title, overview and tagline from TMDB.
func TranslateMovie(ctx context.Context, m *database.Dbmovie) {
	langs := config.MetadataLanguages(config.MediaTypeMovie)
	if len(langs) == 0 || m.ID == 0 || m.MoviedbID == 0 {
		return
	}

	p := providers.GetTMDB()
	if p == nil {
		return
	}

	trs, err := p.GetMovieTranslations(ctx, m.MoviedbID)
	if err != nil {
		logger.Logtype("warn", 1).Err(err).Int("tmdb", m.MoviedbID).Msg("Get movie translations")
		return
	}

	for _, lang := range langs {
		tr := findTranslation(trs, lang)
		database.SetTranslation("dbmovie", m.ID, "title", lang, tr.Title, m.Title)
		database.SetTranslation("dbmovie", m.ID, "overview", lang, tr.Overview, m.Overview)
		database.SetTranslation("dbmovie", m.ID, "tagline", lang, tr.Tagline, m.Tagline)
	}
}

// TranslateSerie fetches a series' name and overview and its episodes'
// titles and overviews from TVDB.
func TranslateSerie(ctx context.Context, s *database.Dbserie) {
	langs := config.MetadataLanguages(config.MediaTypeSeries)
	if len(langs) == 0 || s.ID == 0 || s.ThetvdbID == 0 {
		return
	}

	p := providers.GetTVDB()
	if p == nil {
		return
	}

	episodes := database.StructscanT[struct {
		ID       uint   `db:"id"`
		Season   string `db:"season"`
		Episode  string `db:"episode"`
		Title    string `db:"title"`
		Overview string `db:"overview"`
	}](false, 100, "SELECT id, season, episode, COALESCE(title, '') AS title, COALESCE(overview, '') AS overview FROM dbserie_episodes WHERE dbserie_id = ?", &s.ID)

	for _, lang := range langs {
		tr, err := p.GetSeriesTranslation(ctx, s.ThetvdbID, lang)
		if err != nil {
			logger.Logtype("warn", 1).Err(err).Int(logger.StrTvdb, s.ThetvdbID).Str("language", lang).Msg("Get series translation")
			continue
		}

		database.SetTranslation("dbserie", s.ID, "title", lang, tr.Title, s.Seriename)
		database.SetTranslation("dbserie", s.ID, "overview", lang, tr.Overview, s.Overview)

		if len(episodes) == 0 {
			continue
		}

		eps, err := p.GetEpisodeTranslations(ctx, s.ThetvdbID, lang)
		if err != nil {
			logger.Logtype("warn", 1).Err(err).Int(logger.StrTvdb, s.ThetvdbID).Str("language", lang).Msg("Get episode translations")
			continue
		}

		byNumber := make(map[string]apiexternal_v2.EpisodeTranslation, len(eps))
		for _, e := range eps {
			byNumber[strconv.Itoa(e.Season)+"x"+strconv.Itoa(e.Episode)] = e
		}

		for i := range episodes {
			e := &episodes[i]
			season, err1 := strconv.Atoi(e.Season)
			episode, err2 := strconv.Atoi(e.Episode)

			if err1 != nil || err2 != nil {
				continue
			}

			tr := byNumber[strconv.Itoa(season)+"x"+strconv.Itoa(episode)]
			database.SetTranslation("dbserie_episode", e.ID, "title", lang, tr.Title, e.Title)
			database.SetTranslation("dbserie_episode", e.ID, "overview", lang, tr.Overview, e.Overview)
		}
	}
}

// audnexRegions are the Audible marketplaces of the languages.
var audnexRegions = map[string]string{"en": "us", "de": "de", "fr": "fr", "es": "es", "it": "it", "ja": "jp"}

// TranslateAudiobook fetches an audiobook's title and description from the
// Audible marketplace of each language (Audnex); only an edition in that
// language counts.
func TranslateAudiobook(ctx context.Context, a *database.Dbaudiobook) {
	langs := config.MetadataLanguages(config.MediaTypeAudiobook)
	if len(langs) == 0 || a.ID == 0 || a.ASIN == "" {
		return
	}

	p := providers.GetAudnex()
	if p == nil {
		return
	}

	for _, lang := range langs {
		region := audnexRegions[lang]
		if region == "" {
			continue
		}

		var title, description string

		d, err := p.GetBookByASIN(ctx, a.ASIN, region)
		if err == nil && d != nil && config.LanguageCode(d.Language) == lang {
			title, description = d.Title, d.Description
			if description == "" {
				description = d.Summary
			}
		}

		database.SetTranslation("dbaudiobook", a.ID, "title", lang, title, a.Title)
		database.SetTranslation("dbaudiobook", a.ID, "description", lang, description, a.Description)
	}
}

// TranslateBook fetches a book's title in the group languages from the
// editions of its OpenLibrary work ("Die Säulen der Erde"), and returns
// them by language: they are also the titles its releases carry. OpenLibrary
// has no translated descriptions.
func TranslateBook(ctx context.Context, b *database.Dbbook) map[string]string {
	langs := config.MetadataLanguages(config.MediaTypeBook)
	if len(langs) == 0 || b.ID == 0 || !strings.HasSuffix(b.OpenlibraryID, "W") {
		return nil
	}

	p := getOpenLibraryProvider()
	if p == nil {
		return nil
	}

	titles, err := p.WorkTitles(ctx, b.OpenlibraryID, langs)
	if err != nil {
		logger.Logtype("debug", 1).Err(err).Str("work", b.OpenlibraryID).Msg("book titles")
		return nil
	}

	for _, lang := range langs {
		database.SetTranslation("dbbook", b.ID, "title", lang, titles[lang], b.Title)
	}

	return titles
}

// findTranslation is the translation into lang; empty when there is none.
func findTranslation(trs []apiexternal_v2.Translation, lang string) apiexternal_v2.Translation {
	for _, tr := range trs {
		if tr.Language == lang {
			return tr
		}
	}

	return apiexternal_v2.Translation{}
}
