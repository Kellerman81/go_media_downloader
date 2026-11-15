package database

import (
	"slices"
	"strconv"
	"testing"
	"time"
)

func TestReleaseWords(t *testing.T) {
	for name, want := range map[string][]string{
		"Terra.X.Der.grosse.Bluff.-.Meisterbetrueger.der.Geschichte.2018.GERMAN.DOKU.720p.HDTV.x264-TSCC": {"terra", "x", "der", "grosse", "bluff", "meisterbetrueger", "der", "geschichte"},
		"Terra.X.Serengeti.2019.E02.Zeit.der.Wanderung.German.DOKU.WS.BDRiP.x264-TV4A":                    {"terra", "x", "serengeti", "zeit", "der", "wanderung"},
		"/data/dl/Terra X Planet der Vulkane 2019/Terra.X.Planet.der.Vulkane.2019.mkv":                    {"terra", "x", "planet", "der", "vulkane"},
	} {
		if got, _ := releaseWords(name); !slices.Equal(got, want) {
			t.Errorf("releaseWords(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestMatchEpisodeByTitle uses real Terra X releases and episode titles,
// with the regular episodes and specials that name the same film.
func TestMatchEpisodeByTitle(t *testing.T) {
	db := useMigratedMainDB(t)

	resetTitleCaches(t)

	mustExec(t, db, "insert into dbseries (id, seriename, identifiedby) values (1, 'Terra X', 'title'), (2, 'Planet', 'ep')")
	mustExec(t, db, "insert into dbserie_alternates (dbserie_id, title, slug) values (1, 'Faszination Erde', 'faszination-erde')")

	eps := [][5]string{
		{"S2013E17", "2013", "17", "2013-04-28", "Wilder Planet (12) - Vulkane"},
		{"S2017E41", "2017", "41", "2017-10-29", "Die Europa-Saga (2): Woran wir glauben – was wir denken"},
		{"S2017E43", "2017", "43", "2017-11-12", "Die Europa-Saga (4): Was wir erschaffen – was wir uns leisten"},
		{"S2018E16", "2018", "16", "2018-04-02", "Die Alpen – Eine große Geschichte"},
		{"S2018E32", "2018", "32", "2018-08-12", "Der große Bluff – Meisterbetrüger der Geschichte"},
		{"S2019E18", "2019", "18", "2019-04-21", "Serengeti (1): Zeit des Anfangs"},
		{"S2019E19", "2019", "19", "2019-04-22", "Serengeti (2): Zeit der Wanderung"},
		{"S2019E29", "2019", "29", "2019-07-14", "Planet der Vulkane"},
		{"S2019E43", "2019", "43", "2019-11-03", "Das Eiszeit-Rätsel - Warum starben die Riesentiere aus?"},
		{"S2023E08", "2023", "8", "2023-02-26", "Ungelöste Fälle der Archäologie (9): Verschollen"},
		{"S2021E07", "2021", "7", "2021-02-07", "Ungelöste Fälle der Archäologie (5): Mumien"},
		{"S2023E56", "2023", "56", "2023-11-26", "Der große Terra X-Jahresrückblick"},
		{"S0E22", "0", "22", "2023-11-26", "Der große Terra X-Jahresrückblick - mit Harald Lesch, Jasmina Neudecker und Mirko Drotschmann"},
		{"S0E27", "0", "27", "2025-11-30", "Der große Terra X-Jahresrückblick 2025"},
		{"S0E36", "0", "36", "2026-10-11", "Eine Menschheitsgeschichte – Ozeane - Pazifik – Der weltumspannende Ozean"},
		{"S0E38", "0", "38", "2026-10-25", "Eine Menschheitsgeschichte – Ozeane - Ozeanien – Welt aus Wasser"},
		{"S2026E39", "2026", "39", "2026-10-25", "Ozeane (5): Ozeanien – Welt aus Wasser"},
		{"S2026E38", "2026", "38", "2026-10-18", "Ozeane (4): Indischer Ozean – Im Auge des Monsuns"},
	}
	for i, e := range eps {
		mustExec(t, db, "insert into dbserie_episodes (id, dbserie_id, identifier, season, episode, first_aired, title) values (?, 1, ?, ?, ?, ?, ?)",
			i+1, e[0], e[1], e[2], e[3], e[4])
	}

	for release, want := range map[string]string{
		"Terra X Ozeane Eine Menschheitsgeschichte Ozeanien GERMAN DOKU HDTVRip x264-TMSF":                    "S0E38",
		"Terra.X.Der.grosse.Bluff.-.Meisterbetrueger.der.Geschichte.2018.GERMAN.DOKU.HDTVRiP.x264-TSCC":       "S2018E32",
		"Terra X-Die Europa-Saga 4 - Was wir erschaft \u0096  - 2017 -  German - ac3hd720pmkv by Waldorf":     "S2017E43",
		"Terra.X.Das.Eiszeitraetsel.-.Warum.starben.die.Riesentiere.aus.2019.GERMAN.DOKU.720p.HDTV.x264-TSCC": "S2019E43",
		"Terra.X.Der.grosse.Terra.X.Jahresrueckblick.2023.GERMAN.DOKU.720p.HDTV.x264-TMSF":                    "S2023E56",
		"Terra.X.Planet.der.Vulkane.2019.GERMAN.DOKU.720p.HDTV.x264-TSCC":                                     "S2019E29",
		"Terra.X.Serengeti.2019.E02.Zeit.der.Wanderung.German.DOKU.WS.BDRiP.x264-TV4A":                        "S2019E19",
		"Terra.X.Ungeloeste.Faelle.der.Archaeologie.Verschollen.GERMAN.DOKU.HDTVRip.x264.REPACK-TMSF-2023":    "S2023E08",
		"Faszination.Erde.Planet.der.Vulkane.2019.GERMAN.DOKU.720p.HDTV.x264-TSCC":                            "S2019E29",
		// Not good enough: too few words in common, or two equal candidates.
		"Terra.X.Ozeane.GERMAN.DOKU.720p.HDTV.x264-TMSF":                  "",
		"Terra.X.Die.Geschichte.der.Mode.2019.GERMAN.DOKU.HDTV.x264-TSCC": "",
		// Series identified by S01E01 are left alone.
		"Planet.der.Vulkane.2019.GERMAN.DOKU.720p.HDTV.x264-TSCC": "",
	} {
		m := ParseInfo{File: release, ListID: -1}

		ok := m.MatchEpisodeByTitle()
		if want == "" {
			if ok {
				t.Errorf("%s: matched %s, want none", release, m.Identifier)
			}

			continue
		}

		if !ok || m.Identifier != want || m.DbserieID != 1 || !m.TitleMatched || m.DbserieEpisodeID == 0 {
			t.Errorf("%s: ok=%v identifier=%q dbserie=%d, want %s", release, ok, m.Identifier, m.DbserieID, want)
		}
	}

	// With the series known (e.g. from an alternate title the parser found),
	// only its episodes count.
	m := ParseInfo{File: "Terra.X.Monster.und.Mythen.2018.GERMAN.DOKU.x264-TSCC", DbserieID: 2, ListID: -1}
	if m.MatchEpisodeByTitle() {
		t.Errorf("matched episode of another series: %+v", m)
	}
}

// TestEpisodeTitleFits: an episode number is only trusted when the title the
// release also names fits that episode (real Rosamunde Pilcher and Tatort
// cases: the release numbering of Pilcher differs from the database's).
func TestEpisodeTitleFits(t *testing.T) {
	db := useMigratedMainDB(t)

	resetTitleCaches(t)

	mustExec(t, db, "insert into dbseries (id, seriename, identifiedby) values (1, 'Rosamunde Pilcher', 'title'), (2, 'Tatort', 'title'), (3, 'Show', 'ep')")
	mustExec(t, db, `insert into dbserie_episodes (id, dbserie_id, identifier, season, episode, first_aired, title) values
		(1, 1, 'S01E139', '1', '139', '2014-01-01', 'Schutzengel'),
		(2, 1, 'S01E149', '1', '149', '2018-01-01', 'Geerbtes Glück'),
		(3, 1, 'S2018E01', '2018', '1', '2018-01-01', 'Geerbtes Glück'),
		(4, 2, 'S2003E01', '2003', '1', '2003-01-05', 'Odenthal - 27 - Romeo und Julia'),
		(5, 2, 'S2025E36', '2025', '36', '2025-12-28', 'Murot - 14 - Murot und der Elefant im Raum'),
		(6, 2, 'S2019E29', '2019', '29', '2019-06-01', 'Flückiger - 17 - Der Elefant im Raum'),
		(7, 3, 'S01E01', '1', '1', '2020-01-01', 'Pilot')`)

	for _, tc := range []struct {
		release, season string
		found           uint
		fits            bool
		byTitle         string
	}{
		// The number names another film: the title decides, the season breaks the tie.
		{"Rosamunde.Pilcher.S01E139.Geerbtes.Glueck.GERMAN.WS.HDTV.x264-aWake", "01", 1, false, "S01E149"},
		// Number and title agree.
		{"Tatort.S2003E01.Odenthal.27.Romeo.und.Julia.GERMAN.720p.WEB.H264-M00NBull", "2003", 4, true, ""},
		// One word that isn't in the numbered episode's title.
		{"Rosamunde.Pilcher.S01E139.Herzensfragen.GERMAN.WS.HDTV.XviD-aWake", "01", 1, false, ""},
		// Only a number: trusted.
		{"Rosamunde.Pilcher.S01E139.GERMAN.WS.HDTV.x264-aWake", "01", 1, true, ""},
		// An absolute number the parser can't use, with the title.
		{"Tatort.E1322.Murot.und.der.Elefant.im.Raum.2025.GERMAN.720p.WEB.h264-TMSF", "", 0, true, "S2025E36"},
		// Other series are not checked.
		{"Show.S01E01.Something.Else.Entirely.720p.WEB.x264-X", "01", 7, true, ""},
	} {
		dbserie := uint(1)
		switch {
		case tc.release[0] == 'T':
			dbserie = 2
		case tc.release[0] == 'S':
			dbserie = 3
		}

		m := ParseInfo{File: tc.release, DbserieID: dbserie, DbserieEpisodeID: tc.found, SeasonStr: tc.season, ListID: -1}
		if got := m.EpisodeTitleFits(); got != tc.fits {
			t.Errorf("%s: fits = %v, want %v", tc.release, got, tc.fits)
		}

		if tc.byTitle == "" {
			continue
		}

		m.DbserieEpisodeID = 0
		if !m.MatchEpisodeByTitle() || m.Identifier != tc.byTitle {
			t.Errorf("%s: by title %q, want %s", tc.release, m.Identifier, tc.byTitle)
		}
	}
}

// TestMatchEpisodeByAbsolute: Tatort releases carry the absolute number
// (E0705, "- 1151 -", S01E1311); it decides unless the title contradicts it.
func TestMatchEpisodeByAbsolute(t *testing.T) {
	db := useMigratedMainDB(t)

	resetTitleCaches(t)

	mustExec(t, db, "insert into dbseries (id, seriename, identifiedby) values (1, 'Tatort', 'title')")
	mustExec(t, db, `insert into dbserie_episodes (id, dbserie_id, identifier, season, episode, first_aired, title, absolute_episode) values
		(1, 1, 'S2008E20', '2008', '20', '2008-05-01', 'Lannert - 05 - Liebeswirren', 705),
		(2, 1, 'S2025E10', '2025', '10', '2025-03-01', 'Faber - 30 - Der feine Geist', 1151),
		(3, 1, 'S2025E30', '2025', '30', '2025-09-01', 'Borowski - 40 - Ein anderer Fall', 1311),
		(4, 1, 'S2009E01', '2009', '1', '2009-01-01', 'Odenthal - 40 - Altlasten', 0),
		(5, 1, 'S2015E04', '2015', '4', '2015-02-01', 'Ein einziger Kuss Teil 2', 133),
		(6, 1, 'S2015E05', '2015', '5', '2015-02-08', 'Ein einziger Kuss Teil 1', 0)`)

	for release, want := range map[string]string{
		"Tatort.2008.E0705.Liebeswirren.GERMAN.HDTVRip.x264-hrs": "S2008E20",
		"Tatort - 1151 - Der feine Geist":                        "S2025E10",
		"Tatort.2025.S01E1311.GERMAN.720p.WEB.AVC-hrs":           "S2025E30",
		// S01E133 is a season's episode, not absolute 133 (real Pilcher file).
		"Tatort - S01E133 - Ein einziger Kuss Teil 1 - German [720p hdtv h264 ac3] (tvdb83214).mkv": "S2015E05",
		// The number names another film than the title: no match.
		"Tatort.E1311.Liebeswirren.GERMAN.HDTV.x264-X": "",
		// No absolute number in the database, one word: too little.
		"Tatort.E750.Altlasten.German.2009.WS.dTV.XviD-KLASSiGERTV": "",
	} {
		m := ParseInfo{File: release, ListID: -1}

		ok := m.MatchEpisodeByTitle()
		if (want == "") == ok || m.Identifier != want {
			t.Errorf("%s: ok=%v identifier=%q, want %q", release, ok, m.Identifier, want)
		}
	}
}

// BenchmarkMatchEpisodeByTitle matches a release of a series with as many
// episodes as Tatort.
func BenchmarkMatchEpisodeByTitle(b *testing.B) {
	db := useMigratedMainDB(b)

	resetTitleCaches(b)

	mustExec(b, db, "insert into dbseries (id, seriename, identifiedby) values (1, 'Tatort', 'title')")

	for i := 1; i <= 1300; i++ {
		n := strconv.Itoa(i)
		mustExec(b, db, "insert into dbserie_episodes (id, dbserie_id, identifier, season, episode, first_aired, title, absolute_episode) values (?, 1, ?, '1', ?, '2020-01-01', ?, ?)",
			i, "S01E"+n, n, "Kommissar "+n+" und der lange Fall am Rhein", i)
	}

	b.ReportAllocs()

	for b.Loop() {
		m := ParseInfo{File: "Tatort.Kommissar.700.und.der.lange.Fall.am.Rhein.GERMAN.720p.HDTV.x264-GRP", ListID: -1}
		if !m.MatchEpisodeByTitle() {
			b.Fatal("no match")
		}
	}
}

// resetTitleCaches empties the series and episode caches, before and after
// the test: test databases reuse the same ids.
func resetTitleCaches(t testing.TB) {
	reset := func() {
		titleSeriesCache.at = time.Time{}
		titleEpisodesCache.series = nil
	}

	reset()
	t.Cleanup(reset)
}
