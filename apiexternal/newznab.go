package apiexternal

import (
	"github.com/Kellerman81/go_media_downloader/newznab"
)

// NzbIndexer defines the Indexers to query
type NzbIndexer struct {
	Name                    string
	URL                     string
	Apikey                  string
	UserID                  int
	SkipSslCheck            bool
	Addquotesfortitlequery  bool
	Additional_query_params string
	LastRssId               string
	Customapi               string
	Customurl               string
	Customrssurl            string
	Limitercalls            int
	Limiterseconds          int
}

// QueryNewznabMovieImdb searches Indexers for imbid - strip tt at beginning!
func QueryNewznabMovieImdb(indexers []NzbIndexer, imdbid string, categories []int) (results []newznab.NZB, failedindexers []string, err error) {
	if imdbid == "" {
		return
	}
	for _, row := range indexers {
		var client newznab.Client
		if _, ok := NewznabClients[row.URL]; ok {
			client = NewznabClients[row.URL]
		} else {
			client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
			NewznabClients[row.URL] = client
		}
		resultsadd, erradd := nzbQueryImdb(client, categories, imdbid, row.Additional_query_params, row.Customurl)
		results = append(results, resultsadd...)
		if erradd != nil {
			err = erradd
			failedindexers = append(failedindexers, row.URL)
		}
	}
	return
}

// QueryNewznabTvTvdb searches Indexers for tvdbid using season and episodes
func QueryNewznabTvTvdb(indexers []NzbIndexer, tvdbid int, categories []int, season int, episode int) (results []newznab.NZB, failedindexers []string, err error) {
	if tvdbid == 0 {
		return
	}
	for _, row := range indexers {
		var client newznab.Client
		if _, ok := NewznabClients[row.URL]; ok {
			client = NewznabClients[row.URL]
		} else {
			client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
			NewznabClients[row.URL] = client
		}
		resultsadd, erradd := nzbQueryTvdb(client, categories, tvdbid, season, episode, row.Additional_query_params, row.Customurl)
		results = append(results, resultsadd...)
		if erradd != nil {
			err = erradd
			failedindexers = append(failedindexers, row.URL)
		}
	}
	return
}

// QueryNewznabQuery searches Indexers for string
func QueryNewznabQuery(indexers []NzbIndexer, query string, categories []int, searchtype string) (results []newznab.NZB, failedindexers []string, err error) {
	for _, row := range indexers {
		var client newznab.Client
		if _, ok := NewznabClients[row.URL]; ok {
			client = NewznabClients[row.URL]
		} else {
			client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
			NewznabClients[row.URL] = client
		}
		resultsadd, erradd := nzbQuery(client, categories, query, searchtype, row.Addquotesfortitlequery, row.Additional_query_params, row.Customurl)
		results = append(results, resultsadd...)
		if erradd != nil {
			err = erradd
			failedindexers = append(failedindexers, row.URL)
		}
	}
	return
}

var NewznabClients map[string]newznab.Client

func QueryNewznabQueryUntil(indexers []NzbIndexer, query string, categories []int, searchtype string) (results []newznab.NZB, failedindexers []string, lastindexerids map[string]string, err error) {
	lastindexerids = make(map[string]string, 5)
	for _, row := range indexers {
		var client newznab.Client
		if _, ok := NewznabClients[row.URL]; ok {
			client = NewznabClients[row.URL]
		} else {
			client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
			NewznabClients[row.URL] = client
		}
		resultsadd, erradd := nzbQueryUntil(client, categories, query, searchtype, row.Addquotesfortitlequery, row.LastRssId, row.Additional_query_params, row.Customurl)
		if erradd != nil {
			err = erradd
			failedindexers = append(failedindexers, row.URL)
		} else {
			if len(resultsadd) >= 1 {

				lastindexerids[row.URL] = resultsadd[0].ID
				results = append(results, resultsadd...)
			}
		}
	}
	return
}

// QueryNewznabRSS returns x entries of given category
func QueryNewznabRSS(indexers []NzbIndexer, maxitems int, categories []int) (results []newznab.NZB, failedindexers []string, err error) {
	for _, row := range indexers {
		var client newznab.Client
		if _, ok := NewznabClients[row.URL]; ok {
			client = NewznabClients[row.URL]
		} else {
			client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
			NewznabClients[row.URL] = client
		}
		resultsadd, erradd := client.LoadRSSFeed(categories, maxitems, row.Additional_query_params, row.Customapi, row.Customrssurl)
		results = append(results, resultsadd...)
		if erradd != nil {
			err = erradd
			failedindexers = append(failedindexers, row.URL)
		}
	}
	return
}

// QueryNewznabRSS returns entries of given category up to id
func QueryNewznabRSSLast(indexers []NzbIndexer, maxitems int, categories []int, maxrequests int) (results []newznab.NZB, failedindexers []string, lastindexerids map[string]string, err error) {
	lastindexerids = make(map[string]string, 5)
	results = []newznab.NZB{}
	for _, row := range indexers {
		var client newznab.Client
		if _, ok := NewznabClients[row.URL]; ok {
			client = NewznabClients[row.URL]
		} else {
			client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
			NewznabClients[row.URL] = client
		}
		resultsadd, erradd := client.LoadRSSFeedUntilNZBID(categories, maxitems, row.LastRssId, maxrequests, row.Additional_query_params, row.Customapi, row.Customrssurl)
		if erradd != nil {
			err = erradd
			failedindexers = append(failedindexers, row.URL)
		} else {
			if len(resultsadd) >= 1 {
				lastindexerids[row.URL] = resultsadd[0].ID
				results = append(results, resultsadd...)
			}
		}
	}
	return
}

func nzbQuery(client newznab.Client, categories []int, query string, searchtype string, addquotes bool, additional_query_params string, customurl string) ([]newznab.NZB, error) {
	resp, err := client.SearchWithQuery(categories, query, searchtype, addquotes, additional_query_params, customurl)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func nzbQueryUntil(client newznab.Client, categories []int, query string, searchtype string, addquotes bool, id string, additional_query_params string, customurl string) ([]newznab.NZB, error) {
	resp, err := client.SearchWithQueryUntilNZBID(categories, query, searchtype, addquotes, id, additional_query_params, customurl)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func nzbQueryTvdb(client newznab.Client, categories []int, tvdbid int, season int, episode int, additional_query_params string, customurl string) ([]newznab.NZB, error) {
	resp, err := client.SearchWithTVDB(categories, tvdbid, season, episode, additional_query_params, customurl)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func nzbQueryImdb(client newznab.Client, categories []int, imdbid string, additional_query_params string, customurl string) ([]newznab.NZB, error) {
	resp, err := client.SearchWithIMDB(categories, imdbid, additional_query_params, customurl)

	if err != nil {
		return nil, err
	}

	return resp, nil
}
