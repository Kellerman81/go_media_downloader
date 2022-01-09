package apiexternal

import (
	"errors"

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
	Customrsscategory       string
	RssDownloadAll          bool
	OutputAsJson            bool
	Limitercalls            int
	Limiterseconds          int
	MaxAge                  int
}

func myappendincrease(original []newznab.NZB, increaseby int) []newznab.NZB {
	l := len(original)
	if cap(original) < len(original)+increaseby {
		if len(original)+increaseby > len(original)*2 {
			target := original
			target = make([]newznab.NZB, l, l+increaseby)
			copy(target, original)
			return target
		} else {
			return original
		}
	} else {
		return original
	}
}

// QueryNewznabMovieImdb searches Indexers for imbid - strip tt at beginning!
func QueryNewznabMovieImdb(indexers []NzbIndexer, imdbid string, categories []int) ([]newznab.NZB, []string, error) {
	results := []newznab.NZB{}
	failedindexers := []string{}
	var err error
	if imdbid == "" {
		return results, failedindexers, errors.New("no imdbid")
	}
	for _, row := range indexers {
		var client *newznab.Client
		if _, ok := NewznabClients[row.URL]; ok {
			client = NewznabClients[row.URL]
		} else {
			client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
			NewznabClients[row.URL] = client
		}
		resultsadd, erradd := client.SearchWithIMDB(categories, imdbid, row.Additional_query_params, row.Customurl, row.MaxAge, row.OutputAsJson)

		if erradd != nil {
			err = erradd
			failedindexers = append(failedindexers, row.URL)
		} else {
			if len(indexers) == 1 {
				return resultsadd, failedindexers, err
			}
			results = append(results, resultsadd...)
		}
	}
	return results, failedindexers, err
}

// QueryNewznabTvTvdb searches Indexers for tvdbid using season and episodes
func QueryNewznabTvTvdb(indexers []NzbIndexer, tvdbid int, categories []int, season int, episode int) ([]newznab.NZB, []string, error) {
	results := []newznab.NZB{}
	failedindexers := []string{}
	var err error
	if tvdbid == 0 {
		return results, failedindexers, errors.New("no tvdbid")
	}
	for _, row := range indexers {
		var client *newznab.Client
		if _, ok := NewznabClients[row.URL]; ok {
			client = NewznabClients[row.URL]
		} else {
			client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
			NewznabClients[row.URL] = client
		}
		resultsadd, erradd := client.SearchWithTVDB(categories, tvdbid, season, episode, row.Additional_query_params, row.Customurl, row.MaxAge, row.OutputAsJson)
		if erradd != nil {
			err = erradd
			failedindexers = append(failedindexers, row.URL)
		} else {
			if len(indexers) == 1 {
				return resultsadd, failedindexers, err
			}
			results = append(results, resultsadd...)
		}
	}
	return results, failedindexers, err
}

// QueryNewznabQuery searches Indexers for string
func QueryNewznabQuery(indexers []NzbIndexer, query string, categories []int, searchtype string) ([]newznab.NZB, []string, error) {
	results := []newznab.NZB{}
	failedindexers := []string{}
	var err error
	for _, row := range indexers {
		var client *newznab.Client
		if _, ok := NewznabClients[row.URL]; ok {
			client = NewznabClients[row.URL]
		} else {
			client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
			NewznabClients[row.URL] = client
		}
		resultsadd, erradd := client.SearchWithQuery(categories, query, searchtype, row.Addquotesfortitlequery, row.Additional_query_params, row.Customurl, row.MaxAge, row.OutputAsJson)
		if erradd != nil {
			err = erradd
			failedindexers = append(failedindexers, row.URL)
		} else {
			if len(indexers) == 1 {
				return resultsadd, failedindexers, err
			}
			results = append(results, resultsadd...)
		}
	}
	return results, failedindexers, err
}

var NewznabClients map[string]*newznab.Client

// QueryNewznabRSS returns x entries of given category
func QueryNewznabRSS(indexers []NzbIndexer, maxitems int, categories []int) ([]newznab.NZB, []string, error) {
	results := []newznab.NZB{}
	failedindexers := []string{}
	var err error
	for _, row := range indexers {
		var client *newznab.Client
		if _, ok := NewznabClients[row.URL]; ok {
			client = NewznabClients[row.URL]
		} else {
			client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
			NewznabClients[row.URL] = client
		}
		resultsadd, erradd := client.LoadRSSFeed(categories, maxitems, row.Additional_query_params, row.Customapi, row.Customrssurl, row.Customrsscategory, 0, row.OutputAsJson)
		if erradd != nil {
			err = erradd
			failedindexers = append(failedindexers, row.URL)
		}
		if len(indexers) == 1 {
			return resultsadd, failedindexers, err
		}
		results = append(results, resultsadd...)

	}
	return results, failedindexers, err
}

// QueryNewznabRSS returns entries of given category up to id
func QueryNewznabRSSLast(indexers []NzbIndexer, maxitems int, categories []int, maxrequests int) ([]newznab.NZB, []string, map[string]string, error) {
	results := []newznab.NZB{}
	lastindexerids := make(map[string]string, 1)
	failedindexers := []string{}
	var err error
	for _, row := range indexers {
		var client *newznab.Client
		if _, ok := NewznabClients[row.URL]; ok {
			client = NewznabClients[row.URL]
		} else {
			client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
			NewznabClients[row.URL] = client
		}
		resultsadd, erradd := client.LoadRSSFeedUntilNZBID(categories, maxitems, row.LastRssId, maxrequests, row.Additional_query_params, row.Customapi, row.Customrssurl, row.Customrsscategory, 0, row.OutputAsJson)
		if erradd != nil {
			err = erradd
			failedindexers = append(failedindexers, row.URL)
		} else {
			if len(indexers) == 1 {
				if len(resultsadd) >= 1 {
					lastindexerids[row.URL] = resultsadd[0].ID
				}
				return resultsadd, failedindexers, lastindexerids, err
			}
			if len(resultsadd) >= 1 {
				results = append(results, resultsadd...)
			}
		}
	}
	return results, failedindexers, lastindexerids, err
}
