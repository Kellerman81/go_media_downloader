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

// QueryNewznabMovieImdb searches Indexers for imbid - strip tt at beginning!
func QueryNewznabMovieImdb(row NzbIndexer, imdbid string, categories []int) (*[]newznab.NZB, []string, error) {
	results := []newznab.NZB{}
	failedindexers := []string{}
	var err error
	if imdbid == "" {
		return &results, failedindexers, errors.New("no imdbid")
	}
	var client *newznab.Client
	if checkclient(row.URL) {
		client = getclient(row.URL)
	} else {
		client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
		NewznabClients = append(NewznabClients, Clients{Name: row.URL, Client: *client})
	}
	resultsadd, erradd := client.SearchWithIMDB(categories, imdbid, row.Additional_query_params, row.Customurl, row.MaxAge, row.OutputAsJson)

	if erradd != nil {
		err = erradd
		failedindexers = append(failedindexers, row.URL)
	} else {
		return resultsadd, failedindexers, err
	}
	return &results, failedindexers, err
}

// QueryNewznabTvTvdb searches Indexers for tvdbid using season and episodes
func QueryNewznabTvTvdb(row NzbIndexer, tvdbid int, categories []int, season int, episode int) (*[]newznab.NZB, []string, error) {
	results := []newznab.NZB{}
	failedindexers := []string{}
	var err error
	if tvdbid == 0 {
		return &results, failedindexers, errors.New("no tvdbid")
	}
	var client *newznab.Client
	if checkclient(row.URL) {
		client = getclient(row.URL)
	} else {
		client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
		NewznabClients = append(NewznabClients, Clients{Name: row.URL, Client: *client})
	}
	resultsadd, erradd := client.SearchWithTVDB(categories, tvdbid, season, episode, row.Additional_query_params, row.Customurl, row.MaxAge, row.OutputAsJson)
	if erradd != nil {
		err = erradd
		failedindexers = append(failedindexers, row.URL)
	} else {
		return resultsadd, failedindexers, err
	}
	return &results, failedindexers, err
}

// QueryNewznabQuery searches Indexers for string
func QueryNewznabQuery(row NzbIndexer, query string, categories []int, searchtype string) (*[]newznab.NZB, []string, error) {
	results := []newznab.NZB{}
	failedindexers := []string{}
	var err error
	var client *newznab.Client
	if checkclient(row.URL) {
		client = getclient(row.URL)
	} else {
		client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
		NewznabClients = append(NewznabClients, Clients{Name: row.URL, Client: *client})
	}
	resultsadd, erradd := client.SearchWithQuery(categories, query, searchtype, row.Addquotesfortitlequery, row.Additional_query_params, row.Customurl, row.MaxAge, row.OutputAsJson)
	if erradd != nil {
		err = erradd
		failedindexers = append(failedindexers, row.URL)
	} else {
		return resultsadd, failedindexers, err
	}
	return &results, failedindexers, err
}

type Clients struct {
	Name   string
	Client newznab.Client
}

var NewznabClients []Clients

func checkclient(find string) bool {
	for idx := range NewznabClients {
		if NewznabClients[idx].Name == find {
			return true
		}
	}
	return false
}

func getclient(find string) *newznab.Client {
	for idx := range NewznabClients {
		if NewznabClients[idx].Name == find {
			return &NewznabClients[idx].Client
		}
	}
	return nil
}

// QueryNewznabRSS returns x entries of given category
func QueryNewznabRSS(row NzbIndexer, maxitems int, categories []int) (*[]newznab.NZB, []string, error) {
	failedindexers := []string{}
	var err error
	var client *newznab.Client
	if checkclient(row.URL) {
		client = getclient(row.URL)
	} else {
		client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
		NewznabClients = append(NewznabClients, Clients{Name: row.URL, Client: *client})
	}
	resultsadd, erradd := client.LoadRSSFeed(categories, maxitems, row.Additional_query_params, row.Customapi, row.Customrssurl, row.Customrsscategory, 0, row.OutputAsJson)
	if erradd != nil {
		err = erradd
		failedindexers = append(failedindexers, row.URL)
	}
	return resultsadd, failedindexers, err
}

// QueryNewznabRSS returns entries of given category up to id
func QueryNewznabRSSLast(row NzbIndexer, maxitems int, categories []int, maxrequests int) (*[]newznab.NZB, []string, string, error) {
	results := []newznab.NZB{}
	failedindexers := []string{}
	var err error
	var client *newznab.Client
	if checkclient(row.URL) {
		client = getclient(row.URL)
	} else {
		client = newznab.New(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
		NewznabClients = append(NewznabClients, Clients{Name: row.URL, Client: *client})
	}
	resultsadd, erradd := client.LoadRSSFeedUntilNZBID(categories, maxitems, row.LastRssId, maxrequests, row.Additional_query_params, row.Customapi, row.Customrssurl, row.Customrsscategory, 0, row.OutputAsJson)
	if erradd != nil {
		err = erradd
		failedindexers = append(failedindexers, row.URL)
	} else {
		lastid := ""
		if len((*resultsadd)) >= 1 {
			lastid = (*resultsadd)[0].ID
		}
		return resultsadd, failedindexers, lastid, err
	}
	return &results, failedindexers, "", err
}
