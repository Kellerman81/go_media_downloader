package newznab

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

const (
	brss       = "/rss"
	bapi       = "/api"
	bqlimit    = "&limit="
	bmovieimdb = "&t=movie&imdbid="
	bquotes    = "%22"
)

var errQualityConfig = errors.New("error getting quality config")

// buildURL constructs the API URL to query the indexer: base URL, API key,
// custom URLs, categories of the quality profile, output format, etc.
func buildURL(
	rss bool,
	indexerid int,
	cfgqual *config.QualityConfig,
	row *config.IndexersConfig,
	addb []byte,
) string {
	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)

	// Everything appended below starts with '&'. A custom URL without a query
	// gets "?dl=1" here so the rest still forms a valid query string.
	dlWritten := false

	switch {
	case rss && row.Customrssurl != "":
		// Documented as a path below the indexer URL or a full URL.
		if !strings.HasPrefix(row.Customrssurl, "http://") && !strings.HasPrefix(row.Customrssurl, "https://") {
			bld.WriteString(strings.TrimRight(row.URL, "/"))

			if !strings.HasPrefix(row.Customrssurl, "/") {
				bld.WriteByte('/')
			}
		}

		bld.WriteString(row.Customrssurl)

		if !strings.Contains(row.Customrssurl, "?") {
			bld.WriteString("?dl=1")

			dlWritten = true
		}
	case !rss && row.Customurl != "":
		// Customurl may be a path below the indexer URL or a full URL.
		endpoint := CustomEndpoint(row.Customurl, row.Customapi, row.Apikey)
		if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
			bld.WriteString(strings.TrimRight(row.URL, "/"))

			if !strings.HasPrefix(endpoint, "/") {
				bld.WriteByte('/')
			}
		}

		bld.WriteString(endpoint)

		if !strings.Contains(endpoint, "?") {
			bld.WriteString("?dl=1")

			dlWritten = true
		}
	case row.Customapi != "":
		bld.WriteString(row.URL)

		if rss {
			bld.WriteString(brss)
		} else {
			bld.WriteString(bapi)
		}

		bld.WriteByte('?')
		bld.WriteString(row.Customapi)
		bld.WriteByte('=')
		bld.WriteString(row.Apikey)

		if rss && row.Userid != "" {
			bld.WriteString("&i=")
			bld.WriteString(row.Userid)
		}

	default:
		bld.WriteString(row.URL)

		if rss {
			bld.WriteString(brss)
			bld.WriteString("?r=")
			bld.WriteString(row.Apikey)
			bld.WriteString("&i=")
			bld.WriteString(row.Userid)
		} else {
			bld.WriteString(bapi)
			bld.WriteString("?apikey=")
			bld.WriteString(row.Apikey)
		}
	}

	if indexerid < 0 || indexerid >= len(cfgqual.Indexer) {
		logger.Logtype("warn", 1).
			Str("indexer", row.Name).
			Str(logger.StrQuality, cfgqual.Name).
			Msg("indexer not configured in quality profile - searching without category filter")
	} else if cfgqual.Indexer[indexerid].CategoriesIndexer != "" {
		if rss {
			if row.Customrsscategory != "" {
				bld.WriteByte('&')
				bld.WriteString(row.Customrsscategory)
				bld.WriteByte('=')
				bld.WriteString(cfgqual.Indexer[indexerid].CategoriesIndexer)
			} else {
				bld.WriteString("&t=")
				bld.WriteString(cfgqual.Indexer[indexerid].CategoriesIndexer)
			}
		} else {
			bld.WriteString("&cat=")
			bld.WriteString(cfgqual.Indexer[indexerid].CategoriesIndexer)
		}
	}

	if row.OutputAsJSON {
		bld.WriteString("&o=json")
	}

	if row.MaxAge != 0 {
		bld.WriteString("&maxage=")
		bld.WriteUInt16(row.MaxAge)
	}

	if !dlWritten {
		bld.WriteString("&dl=1")
	}

	for idx := range cfgqual.Indexer {
		if strings.EqualFold(cfgqual.Indexer[idx].TemplateIndexer, row.Name) {
			bld.WriteString(cfgqual.Indexer[idx].AdditionalQueryParams)
			break
		}
	}

	if addb != nil {
		bld.Write(addb)
	}

	return bld.String() // uses RAM
}

// process fetches one result page and adds its entries to results. It
// returns whether the page reached tillid (no further paging needed), the
// first ID on the page, and any error.
func (p *Provider) process(
	ctx context.Context,
	ind *config.IndexersConfig,
	qual *config.QualityConfig,
	urlv string,
	tillid string,
	results *apiexternal_v2.NzbSlice,
) (bool, string, error) {
	result, err := p.ExecuteRequest(ctx, urlv, tillid, ind, qual)
	if err != nil && !errors.Is(err, ErrBroke) {
		return false, "", err
	}

	var firstid string
	for idx := range result {
		if firstid == "" {
			firstid = result[idx].NZB.ID
		}

		results.Add(&result[idx])
	}

	if errors.Is(err, ErrBroke) {
		return true, firstid, nil
	}

	return false, firstid, err
}

// QueryMovieImdb searches the indexer for movies with the given IMDB ID.
func (p *Provider) QueryMovieImdb(
	ctx context.Context,
	cfgind *config.IndexersConfig,
	qual *config.QualityConfig,
	imdbid string,
	indexerid int,
	results *apiexternal_v2.NzbSlice,
) (bool, string, error) {
	if imdbid == "" {
		return false, "", logger.ErrNoID
	}

	b := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(b)

	b.WriteString(bmovieimdb)
	b.WriteString(imdbid)

	if cfgind.MaxEntries != 0 {
		b.WriteString(bqlimit)
		b.WriteString(cfgind.MaxEntriesStr)
	}

	return p.process(
		ctx,
		cfgind,
		qual,
		buildURL(false, indexerid, qual, cfgind, b.Bytes()),
		"",
		results,
	)
}

// QueryTvTvdb searches the indexer for TV episodes with the given TVDB ID,
// optionally restricted to a season and episode.
func (p *Provider) QueryTvTvdb(
	ctx context.Context,
	cfgind *config.IndexersConfig,
	qual *config.QualityConfig,
	tvdbid, indexerid int,
	season, episode string,
	useseason, useepisode bool,
	results *apiexternal_v2.NzbSlice,
) (bool, string, error) {
	if tvdbid == 0 {
		return false, "", logger.ErrNoID
	}

	b := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(b)

	b.WriteString("&t=tvsearch&tvdbid=")
	b.WriteInt(tvdbid)

	if !useepisode || !useseason {
		b.WriteString(bqlimit)
		b.WriteString("100")
	} else if cfgind.MaxEntries != 0 {
		b.WriteString(bqlimit)
		b.WriteUInt16(cfgind.MaxEntries)
	}

	if useseason && season != "" {
		b.WriteString("&season=")
		b.WriteString(season)
	}

	if useepisode && episode != "" {
		b.WriteString("&ep=")
		b.WriteString(episode)
	}

	return p.process(
		ctx,
		cfgind,
		qual,
		buildURL(false, indexerid, qual, cfgind, b.Bytes()),
		"",
		results,
	)
}

// getaddstr builds the title search string: movies get the year appended,
// series the identifier ("S01E05"); for books/audiobooks/music the title
// already holds "Title Author/Artist".
func getaddstr(cfgp *config.MediaTypeConfig, title string, e *apiexternal_v2.Nzbwithprio) string {
	switch cfgp.IsType {
	case config.MediaTypeMovie:
		{
			// Only append year for movies
			if e.Info.Year != 0 {
				return logger.JoinStrings(title, logger.StrSpace, logger.IntToString(e.Info.Year))
			}
		}

	case config.MediaTypeBook, config.MediaTypeAudiobook, config.MediaTypeMusic:
		{
			// For books, audiobooks, and music, the SearchFor field already contains
			// "Title Author/Artist" so don't append year here
			// The title passed in will be the SearchFor value which already includes author/artist
		}
	}

	if e.Info.Identifier != "" {
		return logger.JoinStrings(title, logger.StrSpace, e.Info.Identifier)
	}

	return title
}

// QueryTitle searches the indexer by title, quoting it when configured.
func (p *Provider) QueryTitle(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	e *apiexternal_v2.Nzbwithprio,
	cfgind *config.IndexersConfig,
	qual *config.QualityConfig,
	title string,
	indexerid int,
	results *apiexternal_v2.NzbSlice,
) (bool, string, error) {
	if title == "" {
		return false, "", nil
	}

	b := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(b)

	b.WriteString("&t=search&q=")

	if cfgind.Addquotesfortitlequery {
		b.WriteString(bquotes)
	}

	enhancedTitle := getaddstr(cfgp, title, e)
	b.WriteString(url.QueryEscape(enhancedTitle))

	if cfgind.Addquotesfortitlequery {
		b.WriteString(bquotes)
	}

	if cfgind.MaxEntries != 0 {
		b.WriteString(bqlimit)
		b.WriteUInt16(cfgind.MaxEntries)
	}

	return p.process(
		ctx,
		cfgind,
		qual,
		buildURL(false, indexerid, qual, cfgind, b.Bytes()),
		"",
		results,
	)
}

// QueryRSS reads the indexer's RSS feed down to tillid, paging up to
// RssEntriesloop times. It returns the first ID of the feed.
func (p *Provider) QueryRSS(
	ctx context.Context,
	ind *config.IndexersConfig,
	qual *config.QualityConfig,
	tillid string,
	indexerid int,
	results *apiexternal_v2.NzbSlice,
) (string, error) {
	if indexerid == -1 {
		return "", errQualityConfig
	}

	// Local default only: ind is the shared indexer config, writing to it
	// raced with concurrent searches and leaked &limit=100 into them.
	maxEntries := ind.MaxEntries
	if maxEntries == 0 {
		maxEntries = 100
	}

	b := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(b)

	b.WriteString(bqlimit)
	b.WriteUInt16(maxEntries)

	bld := buildURL(true, indexerid, qual, ind, b.Bytes())

	maxloop := ind.RssEntriesloop
	if maxloop == 0 {
		maxloop = 2
	}

	brokeloop, firstid, err := p.process(ctx, ind, qual, bld, tillid, results)
	if err != nil || results == nil || len(results.Arr) == 0 || brokeloop || maxloop == 1 {
		return firstid, err
	}

	buf := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(buf)

	for count := range maxloop - 1 {
		buf.WriteString(bld)
		buf.WriteString("&offset=")
		buf.WriteUInt16(maxEntries * (uint16(count) + 1))

		brokeloop, _, err = p.process(ctx, ind, qual, buf.String(), tillid, results)
		buf.Reset()

		if err != nil || brokeloop || len(results.Arr) == 0 {
			break
		}
	}

	return firstid, err
}
