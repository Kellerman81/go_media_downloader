package api

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// metaCover renders a search result's cover; nil without one. Album and
// audiobook covers are square, posters 2:3.
func metaCover(src string, square bool) gomponents.Node {
	if !strings.HasPrefix(src, "https://") && !strings.HasPrefix(src, "http://") {
		return nil
	}

	height := "96px"
	if square {
		height = "64px"
	}

	return html.Img(html.Src(src), html.Alt(""), gomponents.Attr("loading", "lazy"),
		html.Class("rounded shadow-sm flex-shrink-0 bg-light"),
		html.Style("width:64px;height:"+height+";object-fit:cover"),
		gomponents.Attr("onerror", "this.remove()"))
}

// metaPosterURL completes TMDB's relative poster paths; the other providers
// return full URLs.
func metaPosterURL(path, provider string) string {
	if strings.HasPrefix(path, "/") && provider == "tmdb" {
		return "https://image.tmdb.org/t/p/w154" + path
	}

	return path
}

// metaLinks renders a search result's external pages as small buttons.
func metaLinks(links []calendarExtLink) gomponents.Node {
	if len(links) == 0 {
		return nil
	}

	nodes := make([]gomponents.Node, 0, len(links))
	for _, l := range links {
		nodes = append(nodes, html.A(html.Class("btn btn-sm btn-outline-secondary py-0 px-2"), html.Href(l.URL),
			html.Target("_blank"), html.Rel("noopener"),
			html.I(html.Class(l.Icon+" me-1"), gomponents.Attr("aria-hidden", "true")), gomponents.Text(l.Label)))
	}

	return html.Div(html.Class("d-flex flex-wrap gap-1 mt-2"), gomponents.Group(nodes))
}

func imdbLink(id string) []calendarExtLink {
	if !strings.HasPrefix(id, "tt") {
		return nil
	}

	return []calendarExtLink{{Label: "IMDb", Icon: "fa-brands fa-imdb", URL: "https://www.imdb.com/title/" + id}}
}

func movieResultLinks(m *apiexternal_v2.MovieSearchResult) []calendarExtLink {
	var links []calendarExtLink

	switch {
	case m.ID <= 0:
	case m.ProviderName == "tmdb":
		links = append(links, calendarExtLink{Label: "TMDB", Icon: "fa-solid fa-film", URL: "https://www.themoviedb.org/movie/" + strconv.Itoa(m.ID)})
	case m.ProviderName == "trakt":
		links = append(links, calendarExtLink{Label: "Trakt", Icon: "fa-solid fa-circle-play", URL: "https://trakt.tv/movies/" + strconv.Itoa(m.ID)})
	}

	return append(links, imdbLink(m.IMDbID)...)
}

func seriesResultLinks(s *apiexternal_v2.SeriesSearchResult) []calendarExtLink {
	var links []calendarExtLink

	add := func(label, icon, u string) { links = append(links, calendarExtLink{Label: label, Icon: icon, URL: u}) }

	if s.ID > 0 {
		switch s.ProviderName {
		case "tmdb":
			add("TMDB", "fa-solid fa-film", "https://www.themoviedb.org/tv/"+strconv.Itoa(s.ID))
		case "tvmaze":
			add("TVmaze", "fa-solid fa-tv", "https://www.tvmaze.com/shows/"+strconv.Itoa(s.ID))
		case "trakt":
			add("Trakt", "fa-solid fa-circle-play", "https://trakt.tv/shows/"+strconv.Itoa(s.ID))
		}
	}

	if s.TVDbID > 0 {
		add("TheTVDB", "fa-solid fa-tv", "https://thetvdb.com/dereferrer/series/"+strconv.Itoa(s.TVDbID))
	}

	return append(links, imdbLink(s.IMDbID)...)
}

// releaseResultLinks reuses the calendar's album links: same IDs, same pages.
func releaseResultLinks(r *apiexternal_v2.ReleaseSearchResult) []calendarExtLink {
	itoa := func(i int) string {
		if i <= 0 {
			return ""
		}

		return strconv.Itoa(i)
	}

	artist := ""
	if len(r.Artists) > 0 {
		artist = r.Artists[0].Name
	}

	return albumLinks(&AlbumCalendarQuery{
		Title: r.Title, Artist: artist,
		MBGroup: r.ReleaseGroupID, MBRelease: r.MusicBrainzID,
		DiscogsMstr: itoa(r.MasterID), DiscogsRel: itoa(r.DiscogsID),
		Deezer: itoa(r.DeezerID), Itunes: itoa(r.ITunesID),
	})
}

func bookResultLinks(b *apiexternal_v2.BookSearchResult) []calendarExtLink {
	return bookLinks(&BookCalendarQuery{Title: b.Title, ISBN: b.ISBN13, OpenLibrary: strings.TrimPrefix(openLibraryWorkID(b.ID), "/works/")},
		strings.Join(b.Authors, ", "))
}

// audiobookResultLinks links Audible on the region of the first audiobook
// media group (the search isn't tied to a group yet).
func audiobookResultLinks(a *apiexternal_v2.AudiobookSearchResult) []calendarExtLink {
	region := ""
	if media := config.GetSettingsMediaAll(); media != nil {
		for i := range media.AudioBooks {
			if media.AudioBooks[i].AudibleRegion != "" {
				region = media.AudioBooks[i].AudibleRegion
				break
			}
		}
	}

	author := ""
	if len(a.Authors) > 0 {
		author = a.Authors[0]
	}

	return audiobookLinks(&AudiobookCalendarQuery{Title: a.Title, Author: author, ASIN: a.ASIN}, region)
}
