package models

import "github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"

// Generators is a map of table
//
// The key of Generators is the prefix of table info url.
// The corresponding value is the Form and Table data.
//
// http://{{config.Domain}}:{{Port}}/{{config.Prefix}}/info/{{key}}
//
// example:
//
// example end
var Generators = map[string]table.Generator{
	"dbmovies":                getDbmoviesTable,
	"dbmovie_titles":          getDbmovieTitlesTable,
	"dbserie_alternates":      getDbserieAlternatesTable,
	"dbserie_episodes":        getDbserieEpisodesTable,
	"dbseries":                getDbseriesTable,
	"movie_files":             getMovieFilesTable,
	"movie_histories":         getMovieHistoriesTable,
	"movies":                  getMoviesTable,
	"qualities":               getQualitiesTable,
	"serie_episode_files":     getSerieEpisodeFilesTable,
	"serie_episode_histories": getSerieEpisodeHistoriesTable,
	"serie_episodes":          getSerieEpisodesTable,
	"series":                  getSeriesTable,
	"movie_file_unmatcheds":   getMovieFileUnmatchedsTable,
	"job_histories":           getJobHistoriesTable,
	"indexer_fails":           getIndexerFailsTable,
	"serie_file_unmatcheds":   getSerieFileUnmatchedsTable,
	"r_sshistories":           getRSshistoriesTable,
	"stats":                   getStatsTable,
	"scheduler":               getSchedulerTable,
	"queue":                   getQueueTable,
	// generators end
}
