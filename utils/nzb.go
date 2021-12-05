package utils

import (
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/newznab"
)

type Nzbwithprio struct {
	Prio             int
	Indexer          string
	ParseInfo        ParseInfo
	NZB              newznab.NZB
	Nzbmovie         database.Movie
	Nzbepisode       database.SerieEpisode
	WantedTitle      string
	WantedAlternates []string
	Quality          config.QualityConfig
	MinimumPriority  int
	Denied           bool
	Reason           string
}
type NzbwithprioJson struct {
	Prio             int
	Indexer          string
	ParseInfo        ParseInfo
	NZB              newznab.NZB
	Nzbmovie         database.MovieJson
	Nzbepisode       database.SerieEpisodeJson
	WantedTitle      string
	WantedAlternates []string
	Quality          config.QualityConfig
	Denied           bool
	Reason           string
}

func filter_size_nzbs(configEntry config.MediaTypeConfig, indexer config.QualityIndexerConfig, rownzb newznab.NZB) bool {
	for idx := range configEntry.DataImport {

		if indexer.Skip_empty_size && rownzb.Size == 0 {
			logger.Log.Debug("Skipped - Size missing: ", rownzb.Title)
			return true
		}
		if !config.ConfigCheck("path_" + configEntry.DataImport[idx].Template_path) {
			return false
		}
		var cfg_path config.PathsConfig
		config.ConfigGet("path_"+configEntry.DataImport[idx].Template_path, &cfg_path)

		if cfg_path.MinSize != 0 {
			if rownzb.Size < int64(cfg_path.MinSize*1024*1024) && rownzb.Size != 0 {
				logger.Log.Debug("Skipped - MinSize not matched: ", rownzb.Title)
				return true
			}
		}

		if cfg_path.MaxSize != 0 {
			if rownzb.Size > int64(cfg_path.MaxSize*1024*1024) {
				logger.Log.Debug("Skipped - MaxSize not matched: ", rownzb.Title)
				return true
			}
		}
	}
	return false
}
func filter_test_quality_wanted(quality config.QualityConfig, m *ParseInfo, rownzb newznab.NZB) bool {
	wanted_release_resolution := false
	for idxqual := range quality.Wanted_resolution {
		if strings.EqualFold(quality.Wanted_resolution[idxqual], m.Resolution) {
			wanted_release_resolution = true
			break
		}
	}
	if len(quality.Wanted_resolution) >= 1 && !wanted_release_resolution {
		logger.Log.Debug("Skipped - unwanted resolution: ", rownzb.Title)
		return false
	}
	wanted_release_quality := false
	for idxqual := range quality.Wanted_quality {
		if !strings.EqualFold(quality.Wanted_quality[idxqual], m.Quality) {
			wanted_release_quality = true
			break
		}
	}
	if len(quality.Wanted_quality) >= 1 && !wanted_release_quality {
		logger.Log.Debug("Skipped - unwanted quality: ", rownzb.Title)
		return false
	}
	wanted_release_audio := false
	for idxqual := range quality.Wanted_audio {
		if strings.EqualFold(quality.Wanted_audio[idxqual], m.Audio) {
			wanted_release_audio = true
			break
		}
	}
	if len(quality.Wanted_audio) >= 1 && !wanted_release_audio {
		logger.Log.Debug("Skipped - unwanted audio: ", rownzb.Title)
		return false
	}
	wanted_release_codec := false
	for idxqual := range quality.Wanted_codec {
		if strings.EqualFold(quality.Wanted_codec[idxqual], m.Codec) {
			wanted_release_codec = true
			break
		}
	}
	if len(quality.Wanted_codec) >= 1 && !wanted_release_codec {
		logger.Log.Debug("Skipped - unwanted codec: ", rownzb.Title)
		return false
	}
	return true
}
func filter_regex_nzbs(regexconfig config.RegexConfig, title string, wantedtitle string, wantedalternates []string) (bool, string) {
	for _, rowtitle := range regexconfig.Rejected {
		//rowrejected := regexp.MustCompile(rowtitle)
		rowrejected := regexconfig.RejectedRegex[rowtitle]
		teststrwanted := rowrejected.FindStringSubmatch(wantedtitle)
		if len(teststrwanted) >= 1 {
			continue
		}
		breakfor := false
		for idx := range wantedalternates {
			teststrwanted := rowrejected.FindStringSubmatch(wantedalternates[idx])
			if len(teststrwanted) >= 1 {
				breakfor = true
				break
			}
		}
		if breakfor {
			break
		}
		teststr := rowrejected.FindStringSubmatch(title)
		if len(teststr) >= 1 {
			logger.Log.Debug("Skipped - Regex rejected: ", title, " Regex ", rowtitle)
			return true, rowtitle
		}
	}
	requiredmatched := false
	for _, rowtitle := range regexconfig.Required {
		//rowrequired := regexp.MustCompile(rowtitle)
		rowrequired := regexconfig.RequiredRegex[rowtitle]

		teststr := rowrequired.FindStringSubmatch(title)
		if len(teststr) >= 1 {
			logger.Log.Debug("Regex required matched: ", title, " Regex ", rowtitle)
			requiredmatched = true
			break
		}
	}
	if len(regexconfig.Required) >= 1 && !requiredmatched {
		logger.Log.Debug("Skipped - required not matched: ", title)
		return true, ""
	}
	return false, ""
}

func checknzbtitle(movietitle string, nzbtitle string) bool {
	logger.Log.Debug("check ", movietitle, " against ", nzbtitle)
	if strings.EqualFold(movietitle, nzbtitle) {
		return true
	}
	movietitle = logger.StringToSlug(movietitle)
	nzbtitle = logger.StringToSlug(nzbtitle)
	logger.Log.Debug("check ", movietitle, " against ", nzbtitle)
	return strings.EqualFold(movietitle, nzbtitle)
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func downloadFile(saveIn string, fileprefix string, filename string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	if len(filename) == 0 {
		filename = path.Base(resp.Request.URL.String())
	}
	var filepath string
	if len(fileprefix) >= 1 {
		filename = fileprefix + filename
	}
	filepath = path.Join(saveIn, filename)
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
