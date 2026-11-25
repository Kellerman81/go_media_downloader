package api

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// Add Media marks search results the media group already has ("Have
// already") and disables their add button: an entry in any list of the
// group the chosen list belongs to counts.

// haveLists are the lists a search result is looked up in: all lists of the
// group of the chosen list (have_list); without one, all lists of the
// kind's groups the account may see. kind as mediaEntriesForKind.
func haveLists(c *gin.Context, kind string) []string {
	acc := accessOf(c)

	if name := strings.TrimSpace(c.PostForm("have_list")); name != "" {
		cfgp, listid := findCfgpAndListID(name, kind+"_", mediaEntriesForKind(kind))
		if cfgp != nil && listid >= 0 && acc.AllowsGroup(cfgp.NamePrefix) {
			lists := make([]string, len(cfgp.Lists))
			for i := range cfgp.Lists {
				lists[i] = cfgp.Lists[i].Name
			}

			return lists
		}
	}

	var lists []string

	for _, media := range mediaEntriesForKind(kind) {
		if !acc.AllowsGroup(kind + "_" + media.Name) {
			continue
		}

		for i := range media.Lists {
			lists = append(lists, media.Lists[i].Name)
		}
	}

	return lists
}

// haveIn reports whether an entry of table (aliased e, joined to its
// metadata row d by join) matching cond is in one of the lists.
func haveIn(lists []string, from, cond string, args ...any) bool {
	if len(lists) == 0 {
		return false
	}

	in := make([]any, len(lists))
	for i := range lists {
		in[i] = &lists[i]
	}

	return database.Getdatarow[bool](false,
		"SELECT EXISTS(SELECT 1 FROM "+from+" WHERE ("+cond+") AND e.listname COLLATE NOCASE IN (?"+
			strings.Repeat(", ?", len(lists)-1)+"))",
		append(args, in...)...)
}

func haveMovie(lists []string, m *apiexternal_v2.MovieSearchResult) bool {
	if m.ID == 0 && m.IMDbID == "" {
		return false
	}

	return haveIn(lists, "movies e INNER JOIN dbmovies d ON d.id = e.dbmovie_id",
		"(? > 0 AND d.moviedb_id = ?) OR (? <> '' AND d.imdb_id = ?)", &m.ID, &m.ID, &m.IMDbID, &m.IMDbID)
}

// haveSeries matches by TVDB/IMDb id, else (TMDB results usually carry
// neither) by name and first air year.
func haveSeries(lists []string, s *apiexternal_v2.SeriesSearchResult) bool {
	from := "series e INNER JOIN dbseries d ON d.id = e.dbserie_id"
	if s.TVDbID > 0 || s.IMDbID != "" {
		return haveIn(lists, from, "(? > 0 AND d.thetvdb_id = ?) OR (? <> '' AND d.imdb_id = ?)",
			&s.TVDbID, &s.TVDbID, &s.IMDbID, &s.IMDbID)
	}

	year := ""
	if !s.FirstAirDate.IsZero() {
		year = strconv.Itoa(s.FirstAirDate.Year())
	}

	return haveIn(lists, from, "(d.seriename = ? COLLATE NOCASE OR d.seriename = ? COLLATE NOCASE) AND (? = '' OR substr(d.firstaired, 1, 4) = ?)",
		&s.Name, &s.OriginalName, &year, &year)
}

func haveBook(lists []string, b *apiexternal_v2.BookSearchResult) bool {
	if b.ID == "" {
		return false
	}

	return haveIn(lists, "books e INNER JOIN dbbooks d ON d.id = e.dbbook_id",
		"d.openlibrary_id = ? OR d.goodreads_id = ?", &b.ID, &b.ID)
}

func haveAudiobook(lists []string, a *apiexternal_v2.AudiobookSearchResult) bool {
	if a.ASIN == "" {
		return false
	}

	return haveIn(lists, "audiobooks e INNER JOIN dbaudiobooks d ON d.id = e.dbaudiobook_id", "d.asin = ?", &a.ASIN)
}

func haveAlbum(lists []string, r *apiexternal_v2.ReleaseSearchResult) bool {
	if r.MusicBrainzID == "" && r.ReleaseGroupID == "" && r.DeezerID == 0 && r.DiscogsID == 0 && r.ITunesID == 0 {
		return false
	}

	return haveIn(lists, "albums e INNER JOIN dbalbums d ON d.id = e.dbalbum_id",
		"(? <> '' AND d.musicbrainz_release_id = ?) OR (? <> '' AND d.musicbrainz_release_group_id = ?) OR "+
			"(? > 0 AND d.deezer_id = ?) OR (? > 0 AND d.discogs_release_id = ?) OR (? > 0 AND d.itunes_id = ?)",
		&r.MusicBrainzID, &r.MusicBrainzID, &r.ReleaseGroupID, &r.ReleaseGroupID,
		&r.DeezerID, &r.DeezerID, &r.DiscogsID, &r.DiscogsID, &r.ITunesID, &r.ITunesID)
}

// metaAddButton is a result's add button; one the group already has is a
// disabled "Have already" (without the add-*-btn class the page's click
// handler binds to).
func metaAddButton(have bool, class string, nodes ...gomponents.Node) gomponents.Node {
	if have {
		var keep []string

		for _, c := range strings.Fields(class) {
			if !strings.HasPrefix(c, "add-") && c != "btn-success" {
				keep = append(keep, c)
			}
		}

		return html.Button(html.Type("button"), html.Class(strings.Join(keep, " ")+" btn-outline-secondary"), html.Disabled(),
			html.Title(t("Already in a list of this media group")),
			html.I(html.Class("fas fa-check me-1"), gomponents.Attr("aria-hidden", "true")), gomponents.Text(t("Have already")))
	}

	return html.Button(append([]gomponents.Node{html.Type("button"), html.Class(class)}, nodes...)...)
}
