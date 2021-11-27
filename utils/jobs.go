package utils

import (
	"io"
	"os"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/remeh/sizedwaitgroup"
)

func JobImportFileCheck(file string, dbtype string, wg *sizedwaitgroup.SizedWaitGroup) {
	defer wg.Done()
	if _, err := os.Stat(file); os.IsNotExist(err) {
		if strings.EqualFold(dbtype, "movie") {
			moviesf, _ := database.QueryMovieFiles(database.Query{Select: "id, movie_id", Where: "location=?", WhereArgs: []interface{}{file}})
			for idx := range moviesf {
				database.DeleteRow("movie_files", database.Query{Where: "id=?", WhereArgs: []interface{}{moviesf[idx].ID}})
				database.UpdateColumn("movies", "missing", true, database.Query{Where: "id=?", WhereArgs: []interface{}{moviesf[idx].MovieID}})
			}
		} else {
			seriefiles, _ := database.QuerySerieEpisodeFiles(database.Query{Select: "id, serie_episode_id", Where: "location=?", WhereArgs: []interface{}{file}})
			for idx := range seriefiles {
				database.DeleteRow("serie_episode_files", database.Query{Where: "id=?", WhereArgs: []interface{}{seriefiles[idx].ID}})
				database.UpdateColumn("serie_episodes", "missing", true, database.Query{Where: "id=?", WhereArgs: []interface{}{seriefiles[idx].SerieEpisodeID}})
			}
		}
	}
}

type feedResults struct {
	Series config.MainSerieConfig
	Movies []database.Dbmovie
}

type InputNotifier struct {
	Targetpath     string
	SourcePath     string
	Title          string
	Season         string
	Episode        string
	Identifier     string
	Series         string
	EpisodeTitle   string
	Tvdb           string
	Year           string
	Imdb           string
	Configuration  string
	Replaced       []string
	ReplacedPrefix string
}

func notifier(event string, noticonfig config.MediaNotificationConfig, notifierdata InputNotifier) {
	if !strings.EqualFold(noticonfig.Event, event) {
		return
	}
	messagetext := noticonfig.Message
	messagetext = strings.Replace(messagetext, "{Target_Path}", notifierdata.Targetpath, -1)
	messagetext = strings.Replace(messagetext, "{Source_Path}", notifierdata.SourcePath, -1)
	messagetext = strings.Replace(messagetext, "{Title}", notifierdata.Title, -1)
	messagetext = strings.Replace(messagetext, "{Year}", notifierdata.Year, -1)
	messagetext = strings.Replace(messagetext, "{Imdb}", notifierdata.Imdb, -1)
	messagetext = strings.Replace(messagetext, "{Tvdb}", notifierdata.Tvdb, -1)
	messagetext = strings.Replace(messagetext, "{Season}", notifierdata.Season, -1)
	messagetext = strings.Replace(messagetext, "{Episode}", notifierdata.Episode, -1)
	messagetext = strings.Replace(messagetext, "{Identifier}", notifierdata.Identifier, -1)
	messagetext = strings.Replace(messagetext, "{EpisodeTitle}", notifierdata.EpisodeTitle, -1)
	messagetext = strings.Replace(messagetext, "{Configuration}", notifierdata.Configuration, -1)
	if len(notifierdata.Replaced) >= 1 {
		replacedstr := notifierdata.Replaced[0]
		if notifierdata.ReplacedPrefix != "" {
			replacedstr = notifierdata.ReplacedPrefix + " " + replacedstr
		}
		messagetext = strings.Replace(messagetext, "{Replaced}", " "+replacedstr, -1)
	} else {
		messagetext = strings.Replace(messagetext, "{Replaced}", "", -1)
	}
	MessageTitle := noticonfig.Title
	MessageTitle = strings.Replace(MessageTitle, "{Target_Path}", notifierdata.Targetpath, -1)
	MessageTitle = strings.Replace(MessageTitle, "{Source_Path}", notifierdata.SourcePath, -1)
	MessageTitle = strings.Replace(MessageTitle, "{Title}", notifierdata.Title, -1)
	MessageTitle = strings.Replace(MessageTitle, "{Year}", notifierdata.Year, -1)
	MessageTitle = strings.Replace(MessageTitle, "{Imdb}", notifierdata.Imdb, -1)
	MessageTitle = strings.Replace(MessageTitle, "{Tvdb}", notifierdata.Tvdb, -1)
	MessageTitle = strings.Replace(MessageTitle, "{Season}", notifierdata.Season, -1)
	MessageTitle = strings.Replace(MessageTitle, "{Episode}", notifierdata.Episode, -1)
	MessageTitle = strings.Replace(MessageTitle, "{Identifier}", notifierdata.Identifier, -1)
	MessageTitle = strings.Replace(MessageTitle, "{EpisodeTitle}", notifierdata.EpisodeTitle, -1)
	MessageTitle = strings.Replace(MessageTitle, "{Configuration}", notifierdata.Configuration, -1)

	if !config.ConfigCheck("notification_" + noticonfig.Map_notification) {
		return
	}
	var cfg_notification config.NotificationConfig
	config.ConfigGet("notification_"+noticonfig.Map_notification, &cfg_notification)

	if strings.EqualFold(cfg_notification.Type, "pushover") {
		if apiexternal.PushoverApi.ApiKey != cfg_notification.Apikey {
			apiexternal.NewPushOverClient(cfg_notification.Apikey)
		}

		err := apiexternal.PushoverApi.SendMessage(messagetext, MessageTitle, cfg_notification.Recipient)
		if err != nil {
			logger.Log.Error("Error sending pushover", err)
		} else {
			logger.Log.Info("Pushover message sent")
		}
	}
	if strings.EqualFold(cfg_notification.Type, "csv") {
		f, errf := os.OpenFile(cfg_notification.Outputto,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if errf != nil {
			logger.Log.Error("Error opening csv to write", errf)
			return
		}
		defer f.Close()
		if errf == nil {
			_, errc := io.WriteString(f, messagetext+"\n")
			if errc != nil {
				logger.Log.Error("Error writing to csv", errc)
			} else {
				logger.Log.Info("csv written")
			}
		}
	}
}

func Feeds(configEntry config.MediaTypeConfig, list config.MediaListsConfig) feedResults {
	if !list.Enabled {
		logger.Log.Debug("Error - Group list not enabled")
		return feedResults{}
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		logger.Log.Debug("Error - list not found")
		return feedResults{}
	}
	var cfg_list config.ListsConfig
	config.ConfigGet("list_"+list.Template_list, &cfg_list)

	if !cfg_list.Enabled {
		logger.Log.Debug("Error - list not enabled")
		return feedResults{}
	}

	if strings.EqualFold(cfg_list.Type, "seriesconfig") {
		return feedResults{Series: config.LoadSerie(cfg_list.Series_config_file)}
	}
	if strings.EqualFold(cfg_list.Type, "traktpublicshowlist") {
		return feedResults{Series: GetTraktUserPublicShowList(configEntry, list)}
	}

	if strings.EqualFold(cfg_list.Type, "imdbcsv") {
		return feedResults{Movies: getMissingIMDBMoviesV2(configEntry, list)}
	}
	if strings.EqualFold(cfg_list.Type, "traktpublicmovielist") {
		return feedResults{Movies: GetTraktUserPublicMovieList(configEntry, list)}
	}
	if strings.EqualFold(cfg_list.Type, "traktmoviepopular") {
		traktpopular, err := apiexternal.TraktApi.GetMoviePopular(cfg_list.Limit)
		if err == nil {
			d := make([]database.Dbmovie, 0, len(traktpopular))

			for idx := range traktpopular {
				if len(traktpopular[idx].Ids.Imdb) == 0 {
					continue
				}
				if cfg_list.MinVotes != 0 {
					countergenre, _ := database.ImdbCountRows("imdb_ratings", database.Query{Where: "tconst = ? COLLATE NOCASE and num_votes < ?", WhereArgs: []interface{}{traktpopular[idx].Ids.Imdb, cfg_list.MinVotes}})
					if countergenre >= 1 {
						continue
					}
				}
				if cfg_list.MinRating != 0 {
					countergenre, _ := database.ImdbCountRows("imdb_ratings", database.Query{Where: "tconst = ? COLLATE NOCASE and average_rating < ?", WhereArgs: []interface{}{traktpopular[idx].Ids.Imdb, cfg_list.MinRating}})
					if countergenre >= 1 {
						continue
					}
				}
				if len(cfg_list.Excludegenre) >= 1 {
					excludebygenre := false
					for idxgenre := range cfg_list.Excludegenre {
						countergenre, _ := database.ImdbCountRows("imdb_genres", database.Query{Where: "tconst = ? COLLATE NOCASE and genre = ? COLLATE NOCASE", WhereArgs: []interface{}{traktpopular[idx].Ids.Imdb, cfg_list.Excludegenre[idxgenre]}})
						if countergenre >= 1 {
							excludebygenre = true
							break
						}
					}
					if excludebygenre {
						continue
					}
				}
				if len(cfg_list.Includegenre) >= 1 {
					includebygenre := false
					for idxgenre := range cfg_list.Includegenre {
						countergenre, _ := database.ImdbCountRows("imdb_genres", database.Query{Where: "tconst = ? COLLATE NOCASE and genre = ? COLLATE NOCASE", WhereArgs: []interface{}{traktpopular[idx].Ids.Imdb, cfg_list.Includegenre[idxgenre]}})
						if countergenre >= 1 {
							includebygenre = true
							break
						}
					}
					if !includebygenre {
						continue
					}
				}
				dbentry := database.Dbmovie{ImdbID: traktpopular[idx].Ids.Imdb, Title: traktpopular[idx].Title, Year: traktpopular[idx].Year}
				d = append(d, dbentry)
			}
			return feedResults{Movies: d}
		}
	}
	if strings.EqualFold(cfg_list.Type, "traktmovieanticipated") {
		traktpopular, err := apiexternal.TraktApi.GetMovieAnticipated(cfg_list.Limit)
		if err == nil {
			d := make([]database.Dbmovie, 0, len(traktpopular))

			for idx := range traktpopular {
				if len(traktpopular[idx].Movie.Ids.Imdb) == 0 {
					continue
				}
				if cfg_list.MinVotes != 0 {
					countergenre, _ := database.ImdbCountRows("imdb_ratings", database.Query{Where: "tconst = ? COLLATE NOCASE and num_votes < ?", WhereArgs: []interface{}{traktpopular[idx].Movie.Ids.Imdb, cfg_list.MinVotes}})
					if countergenre >= 1 {
						continue
					}
				}
				if cfg_list.MinRating != 0 {
					countergenre, _ := database.ImdbCountRows("imdb_ratings", database.Query{Where: "tconst = ? COLLATE NOCASE and average_rating < ?", WhereArgs: []interface{}{traktpopular[idx].Movie.Ids.Imdb, cfg_list.MinRating}})
					if countergenre >= 1 {
						continue
					}
				}
				if len(cfg_list.Excludegenre) >= 1 {
					excludebygenre := false
					for idxgenre := range cfg_list.Excludegenre {
						countergenre, _ := database.ImdbCountRows("imdb_genres", database.Query{Where: "tconst = ? COLLATE NOCASE and genre = ? COLLATE NOCASE", WhereArgs: []interface{}{traktpopular[idx].Movie.Ids.Imdb, cfg_list.Excludegenre[idxgenre]}})
						if countergenre >= 1 {
							excludebygenre = true
							break
						}
					}
					if excludebygenre {
						continue
					}
				}
				if len(cfg_list.Includegenre) >= 1 {
					includebygenre := false
					for idxgenre := range cfg_list.Includegenre {
						countergenre, _ := database.ImdbCountRows("imdb_genres", database.Query{Where: "tconst = ? COLLATE NOCASE and genre = ? COLLATE NOCASE", WhereArgs: []interface{}{traktpopular[idx].Movie.Ids.Imdb, cfg_list.Includegenre[idxgenre]}})
						if countergenre >= 1 {
							includebygenre = true
							break
						}
					}
					if !includebygenre {
						continue
					}
				}
				dbentry := database.Dbmovie{ImdbID: traktpopular[idx].Movie.Ids.Imdb, Title: traktpopular[idx].Movie.Title, Year: traktpopular[idx].Movie.Year}
				d = append(d, dbentry)
			}
			return feedResults{Movies: d}
		}
	}
	if strings.EqualFold(cfg_list.Type, "traktmovietrending") {
		traktpopular, err := apiexternal.TraktApi.GetMovieTrending(cfg_list.Limit)
		if err == nil {
			d := make([]database.Dbmovie, 0, len(traktpopular))

			for idx := range traktpopular {
				if len(traktpopular[idx].Movie.Ids.Imdb) == 0 {
					continue
				}
				if cfg_list.MinVotes != 0 {
					countergenre, _ := database.ImdbCountRows("imdb_ratings", database.Query{Where: "tconst = ? COLLATE NOCASE and num_votes < ?", WhereArgs: []interface{}{traktpopular[idx].Movie.Ids.Imdb, cfg_list.MinVotes}})
					if countergenre >= 1 {
						continue
					}
				}
				if cfg_list.MinRating != 0 {
					countergenre, _ := database.ImdbCountRows("imdb_ratings", database.Query{Where: "tconst = ? COLLATE NOCASE and average_rating < ?", WhereArgs: []interface{}{traktpopular[idx].Movie.Ids.Imdb, cfg_list.MinRating}})
					if countergenre >= 1 {
						continue
					}
				}
				if len(cfg_list.Excludegenre) >= 1 {
					excludebygenre := false
					for idxgenre := range cfg_list.Excludegenre {
						countergenre, _ := database.ImdbCountRows("imdb_genres", database.Query{Where: "tconst = ? COLLATE NOCASE and genre = ? COLLATE NOCASE", WhereArgs: []interface{}{traktpopular[idx].Movie.Ids.Imdb, cfg_list.Excludegenre[idxgenre]}})
						if countergenre >= 1 {
							excludebygenre = true
							break
						}
					}
					if excludebygenre {
						continue
					}
				}
				if len(cfg_list.Includegenre) >= 1 {
					includebygenre := false
					for idxgenre := range cfg_list.Includegenre {
						countergenre, _ := database.ImdbCountRows("imdb_genres", database.Query{Where: "tconst = ? COLLATE NOCASE and genre = ? COLLATE NOCASE", WhereArgs: []interface{}{traktpopular[idx].Movie.Ids.Imdb, cfg_list.Includegenre[idxgenre]}})
						if countergenre >= 1 {
							includebygenre = true
							break
						}
					}
					if !includebygenre {
						continue
					}
				}
				dbentry := database.Dbmovie{ImdbID: traktpopular[idx].Movie.Ids.Imdb, Title: traktpopular[idx].Movie.Title, Year: traktpopular[idx].Movie.Year}
				d = append(d, dbentry)
			}
			return feedResults{Movies: d}
		}
	}

	if strings.EqualFold(cfg_list.Type, "traktseriepopular") {
		traktpopular, err := apiexternal.TraktApi.GetSeriePopular(cfg_list.Limit)
		if err == nil {
			d := make([]config.SerieConfig, 0, len(traktpopular))

			for idx := range traktpopular {
				if traktpopular[idx].Ids.Tvdb == 0 {
					continue
				}
				dbentry := config.SerieConfig{Name: traktpopular[idx].Title, TvdbID: traktpopular[idx].Ids.Tvdb}
				d = append(d, dbentry)
			}
			return feedResults{Series: config.MainSerieConfig{Serie: d}}
		}
	}
	if strings.EqualFold(cfg_list.Type, "traktserieanticipated") {
		traktpopular, err := apiexternal.TraktApi.GetSerieAnticipated(cfg_list.Limit)
		if err == nil {
			d := make([]config.SerieConfig, 0, len(traktpopular))

			for idx := range traktpopular {
				if traktpopular[idx].Serie.Ids.Tvdb == 0 {
					continue
				}
				dbentry := config.SerieConfig{Name: traktpopular[idx].Serie.Title, TvdbID: traktpopular[idx].Serie.Ids.Tvdb}
				d = append(d, dbentry)
			}
			return feedResults{Series: config.MainSerieConfig{Serie: d}}
		}
	}
	if strings.EqualFold(cfg_list.Type, "traktserietrending") {
		traktpopular, err := apiexternal.TraktApi.GetSerieTrending(cfg_list.Limit)
		if err == nil {
			d := make([]config.SerieConfig, 0, len(traktpopular))

			for idx := range traktpopular {
				if traktpopular[idx].Serie.Ids.Tvdb == 0 {
					continue
				}
				dbentry := config.SerieConfig{Name: traktpopular[idx].Serie.Title, TvdbID: traktpopular[idx].Serie.Ids.Tvdb}
				d = append(d, dbentry)
			}
			return feedResults{Series: config.MainSerieConfig{Serie: d}}
		}
	}
	logger.Log.Error("Feed Config not found - template: ", list.Template_list, " - type: ", cfg_list, " - name: ", cfg_list.Name)
	return feedResults{}
}
