package api

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// The pages show metadata texts (titles, overviews) in the account's
// language (requestLanguage) where metadata_translations has them, else as
// stored.

// libTranslate puts the translations into lang of their titles and
// overviews into grid rows.
func libTranslate(k *libKind, rows []libRow, lang string) {
	mediaType := database.TranslationMediaType(k.DbTable)
	if mediaType == "" || len(rows) == 0 {
		return
	}

	ids := make([]uint, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].DbID)
	}

	trs := database.TranslationsIn(mediaType, lang, ids)

	for i := range rows {
		tr := trs[rows[i].DbID]
		if v := tr["title"]; v != "" {
			rows[i].Title, rows[i].NoTitle = v, false
		}

		if v := firstText(tr["overview"], tr["description"]); v != "" {
			rows[i].Overview = v
		}
	}
}

// libTranslateEpisodes translates the episode view's rows: the episode's
// title and overview, the series name.
func libTranslateEpisodes(rows []libEpRow, lang string) {
	if len(rows) == 0 {
		return
	}

	eps, series := make([]uint, 0, len(rows)), make([]uint, 0, len(rows))
	for i := range rows {
		eps = append(eps, rows[i].EpDbID)
		series = append(series, rows[i].DbID)
	}

	epTr := database.TranslationsIn("dbserie_episode", lang, eps)
	serTr := database.TranslationsIn("dbserie", lang, series)

	for i := range rows {
		r := &rows[i]
		r.Title = firstText(epTr[r.EpDbID]["title"], r.Title)
		r.Overview = firstText(epTr[r.EpDbID]["overview"], r.Overview)
		r.Sub = firstText(serTr[r.DbID]["title"], r.Sub)
	}
}

// translateCalendarEvents puts the translations into lang into calendar
// events: movies, series (name, episode title and overview), audiobooks
// and books.
func translateCalendarEvents(events []calendarEvent, lang string) {
	ids := map[string][]uint{}
	key := func(s string) uint {
		n, _ := strconv.ParseUint(s, 10, 64)
		return uint(n)
	}

	types := map[string]string{"movie": "dbmovie", "audiobook": "dbaudiobook", "book": "dbbook"}

	for i := range events {
		e := &events[i]
		if e.Type == "series" {
			ids["dbserie_episode"] = append(ids["dbserie_episode"], key(e.dbKey))
			ids["dbserie"] = append(ids["dbserie"], key(e.coverKey))
		} else if mt := types[e.Type]; mt != "" {
			ids[mt] = append(ids[mt], key(e.dbKey))
		}
	}

	if len(ids) == 0 {
		return
	}

	trs := make(map[string]map[uint]map[string]string, len(ids))
	for mt, list := range ids {
		trs[mt] = database.TranslationsIn(mt, lang, list)
	}

	for i := range events {
		e := &events[i]

		if e.Type != "series" {
			tr := trs[types[e.Type]][key(e.dbKey)]
			e.Title = firstText(tr["title"], e.Title)
			e.Overview = firstText(tr["overview"], tr["description"], e.Overview)

			continue
		}

		name := e.Title
		if e.EpTitle != "" {
			name = e.Show
		}

		ep := trs["dbserie_episode"][key(e.dbKey)]
		name = firstText(trs["dbserie"][key(e.coverKey)]["title"], name)
		e.Overview = firstText(ep["overview"], e.Overview)

		if e.EpTitle = firstText(ep["title"], e.EpTitle); e.EpTitle != "" {
			e.Show, e.Title = name, name+" - "+e.EpTitle
		} else {
			e.Title = name
		}
	}
}

// firstText is the first text that isn't empty.
func firstText(texts ...string) string {
	for _, s := range texts {
		if s != "" {
			return s
		}
	}

	return ""
}

// translatePlayMeta puts the translations into lang into the player's
// metadata of a file: dbID is its dbmovies, dbseries or dbaudiobooks row,
// epID its dbserie_episodes row.
func translatePlayMeta(m *playMeta, kind, lang, dbID, epID string) {
	id, _ := strconv.ParseUint(dbID, 10, 64)
	if lang == "" || id == 0 {
		return
	}

	// rename swaps the entry's name in Sub (series · S01E02, author · book).
	rename := func(old *string, tr string) {
		if tr == "" || tr == *old {
			return
		}

		if *old != "" {
			m.Sub = strings.Replace(m.Sub, *old, tr, 1)
		}

		*old = tr
	}

	switch kind {
	case "movie":
		tr := database.TranslationsIn("dbmovie", lang, []uint{uint(id)})[uint(id)]
		m.Title = firstText(tr["title"], m.Title)
		m.Overview = firstText(tr["overview"], m.Overview)
	case "episode":
		tr := database.TranslationsIn("dbserie", lang, []uint{uint(id)})[uint(id)]
		rename(&m.Artist, tr["title"])
		m.Overview = firstText(tr["overview"], m.Overview)

		if ep, _ := strconv.ParseUint(epID, 10, 64); ep > 0 {
			etr := database.TranslationsIn("dbserie_episode", lang, []uint{uint(ep)})[uint(ep)]
			if v := etr["title"]; v != "" {
				if m.Title == m.EpisodeTitle {
					m.Title = v
				}

				m.EpisodeTitle = v
			}

			m.EpisodeOverview = firstText(etr["overview"], m.EpisodeOverview)
		}
	case "audiobook":
		tr := database.TranslationsIn("dbaudiobook", lang, []uint{uint(id)})[uint(id)]
		rename(&m.Album, tr["title"])
		m.Overview = firstText(tr["description"], m.Overview)
	}
}
