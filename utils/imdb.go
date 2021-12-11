package utils

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/types"
	_ "github.com/mattn/go-sqlite3"
	"github.com/remeh/sizedwaitgroup"
)

func csvsetdefault(instr string, def string) string {
	if instr == `\N` {
		instr = def
	}
	return instr
}
func InitFillImdb() {
	var cfg_imdb config.ImdbConfig
	config.ConfigGet("imdb", &cfg_imdb)
	logger.Log.Info("Started Imdb Import")
	titlemap := make(map[string]bool, 10)
	for _, row := range cfg_imdb.Indexedtypes {
		titlemap[row] = true
	}
	akamap := make(map[string]bool, 10)
	for _, row := range cfg_imdb.Indexedlanguages {
		akamap[row] = true
	}
	os.Remove("./imdbtemp.db")
	database.DBImdb.Close()
	dbget := database.InitImdbdb("info", "imdbtemp")
	database.DBImdb = dbget

	m, err := migrate.New(
		"file://./schema/imdbdb",
		"sqlite3://imdbtemp.db?cache=shared&_fk=1&_mutex=no&_cslike=0",
	)
	if err != nil {
		logger.Log.Errorf("migration failed... %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Log.Errorf("An error occurred while syncing the database.. %v", err)
	}

	downloadimdbfiles()

	cacherowlimit := 1999

	logger.Log.Info("Opening titles..")
	filetitle, err := os.Open("./title.basics.tsv")
	cachetconst := make(map[string]bool, 9000000)
	if err != nil {
		logger.Log.Errorf("An error occurred while opening titles.. %v", err)
	} else {
		parsertitle := csv.NewReader(filetitle)
		parsertitle.Comma = '\t'
		parsertitle.LazyQuotes = true
		parsertitle.ReuseRecord = true
		parsertitle.TrimLeadingSpace = true
		_, _ = parsertitle.Read() //skip header

		titlesshort := make([]database.ImdbTitle, 0, cacherowlimit)
		genres := make([]database.ImdbGenres, 0, cacherowlimit)
		swtitle := sizedwaitgroup.New(4)
		// namedtitle, _ := database.DBImdb.PrepareNamed("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES (:tconst, :title_type, :primary_title, :slug, :original_title, :is_adult, :start_year, :end_year, :runtime_minutes, :genres)")
		// namedgenre, _ := database.DBImdb.PrepareNamed("insert into imdb_genres (tconst, genre) VALUES (:tconst, :genre)")

		for {
			record, err := parsertitle.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				logger.Log.Errorf("An error occurred while parsing title.. %v", err)
				continue
			}
			swtitle.Add()
			if len(titlesshort) >= cacherowlimit {
				database.ReadWriteMu.Lock()
				_, err := database.DBImdb.NamedExec("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES (:tconst, :title_type, :primary_title, :slug, :original_title, :is_adult, :start_year, :end_year, :runtime_minutes, :genres)", titlesshort)
				database.ReadWriteMu.Unlock()
				if err != nil {
					logger.Log.Errorf("An error occurred while inserting titles.. %v", err)
					break
				}
				titlesshort = make([]database.ImdbTitle, 0, cacherowlimit)
			}
			if len(genres) >= cacherowlimit {
				database.ReadWriteMu.Lock()
				_, err := database.DBImdb.NamedExec("insert into imdb_genres (tconst, genre) VALUES (:tconst, :genre)", genres)
				database.ReadWriteMu.Unlock()
				if err != nil {
					logger.Log.Errorf("An error occurred while inserting genres.. %v", err)
					break
				}
				genres = make([]database.ImdbGenres, 0, cacherowlimit)
			}
			if _, ok := titlemap[record[1]]; ok {
				database.ReadWriteMu.Lock()
				cachetconst[record[0]] = true
				database.ReadWriteMu.Unlock()
				startYear, _ := strconv.Atoi(csvsetdefault(record[5], "0"))
				stringtitletype := csvsetdefault(record[1], "")
				stringtitleprimary := html.UnescapeString(csvsetdefault(record[2], ""))
				stringtitleprimaryslug := logger.StringToSlug(stringtitleprimary)
				if !cfg_imdb.Indexfull {
					titlesshort = append(titlesshort, database.ImdbTitle{
						Tconst:       record[0],
						TitleType:    stringtitletype,
						PrimaryTitle: stringtitleprimary,
						Slug:         stringtitleprimaryslug,
						StartYear:    startYear,
					})
				} else {
					stringtitleoriginal := html.UnescapeString(csvsetdefault(record[2], ""))
					stringgenre := csvsetdefault(record[8], "")
					endYear, errdate := strconv.Atoi(csvsetdefault(record[6], "0"))
					if errdate != nil {
						logger.Log.Error("Date error: ", record[6], " ", errdate)
						continue
					}
					runtimeMinutes, errrun := strconv.Atoi(csvsetdefault(record[7], "0"))
					if errrun != nil {
						logger.Log.Error("Runtime error: ", record[7], " ", errrun)
						continue
					}
					isAdult, erradu := strconv.ParseBool(csvsetdefault(record[4], "0"))
					if erradu != nil {
						logger.Log.Error("Adult error: ", record[4], " ", erradu)
						continue
					}
					titlesshort = append(titlesshort, database.ImdbTitle{
						Tconst:         record[0],
						TitleType:      stringtitletype,
						PrimaryTitle:   stringtitleprimary,
						Slug:           stringtitleprimaryslug,
						OriginalTitle:  stringtitleoriginal,
						Genres:         stringgenre,
						IsAdult:        isAdult,
						StartYear:      startYear,
						RuntimeMinutes: runtimeMinutes,
						EndYear:        endYear,
					})
					var genrearray []string
					if strings.Contains(stringgenre, ",") {
						genrearray = strings.Split(stringgenre, ",")
					} else if len(stringgenre) >= 1 {
						genrearray = []string{stringgenre}
					}
					for idxgenre := range genrearray {
						genreentry := database.ImdbGenres{
							Tconst: record[0],
							Genre:  genrearray[idxgenre],
						}
						genres = append(genres, genreentry)
					}
				}
			}
			swtitle.Done()
		}
		swtitle.Wait()
		if len(titlesshort) >= 1 {
			database.ReadWriteMu.Lock()
			database.DBImdb.NamedExec("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES (:tconst, :title_type, :primary_title, :slug, :original_title, :is_adult, :start_year, :end_year, :runtime_minutes, :genres)", titlesshort)
			database.ReadWriteMu.Unlock()
		}
		if len(genres) >= 1 {
			database.ReadWriteMu.Lock()
			database.DBImdb.NamedExec("insert into imdb_genres (tconst, genre) VALUES (:tconst, :genre)", genres)
			database.ReadWriteMu.Unlock()
		}
	}

	logger.Log.Info("Opening akas..")
	fileaka, err := os.Open("./title.akas.tsv")
	if err != nil {
		logger.Log.Errorf("An error occurred while opening akas.. %v", err)
	} else {

		parseraka := csv.NewReader(fileaka)
		parseraka.Comma = '\t'
		parseraka.LazyQuotes = true
		parseraka.ReuseRecord = true
		parseraka.TrimLeadingSpace = true
		_, _ = parseraka.Read() //skip header
		akasshort := make([]database.ImdbAka, 0, cacherowlimit)

		swaka := sizedwaitgroup.New(4)
		for {
			record, err := parseraka.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				logger.Log.Errorf("An error occurred while parsing aka.. %v", err)
				continue
			}
			swaka.Add()
			if len(akasshort) >= cacherowlimit {
				database.ReadWriteMu.Lock()
				_, err := database.DBImdb.NamedExec("insert into imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES (:tconst, :ordering, :title, :slug, :region, :language, :types, :attributes, :is_original_title)", akasshort)
				database.ReadWriteMu.Unlock()
				if err != nil {
					logger.Log.Errorf("An error occurred while inserting aka.. %v", err)
					break
				}
				akasshort = make([]database.ImdbAka, 0, cacherowlimit)
			}

			if _, ok := akamap[record[3]]; ok || len(record[3]) == 0 {
				//titlecount, _ := database.ImdbCountRows("imdb_titles", database.Query{Where: "tconst = ?", WhereArgs: []interface{}{record[0]}})
				if _, ok := cachetconst[record[0]]; ok {
					stringtitle := html.UnescapeString(csvsetdefault(record[2], ""))
					stringtitleslug := logger.StringToSlug(stringtitle)
					stringregion := csvsetdefault(record[3], "")
					if !cfg_imdb.Indexfull {
						akasshort = append(akasshort, database.ImdbAka{
							Tconst: record[0],
							Title:  stringtitle,
							Slug:   stringtitleslug,
							Region: stringregion,
						})
					} else {
						stringlanguage := csvsetdefault(record[4], "")
						stringtypes := csvsetdefault(record[5], "")
						stringattributes := csvsetdefault(record[6], "0")
						ordering, _ := strconv.Atoi(csvsetdefault(record[1], "0"))
						isOriginalTitle, _ := strconv.ParseBool(csvsetdefault(record[7], "0"))
						akasshort = append(akasshort, database.ImdbAka{
							Tconst:          record[0],
							Ordering:        ordering,
							Title:           stringtitle,
							Slug:            stringtitleslug,
							Region:          stringregion,
							Language:        stringlanguage,
							Types:           stringtypes,
							Attributes:      stringattributes,
							IsOriginalTitle: isOriginalTitle,
						})
					}
				}
			}
			swaka.Done()
		}
		swaka.Wait()
		if len(akasshort) >= 1 {
			database.ReadWriteMu.Lock()
			database.DBImdb.NamedExec("insert into imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES (:tconst, :ordering, :title, :slug, :region, :language, :types, :attributes, :is_original_title)", akasshort)
			database.ReadWriteMu.Unlock()
		}
	}

	logger.Log.Info("Opening ratings..")
	filerating, err := os.Open("./title.ratings.tsv")
	if err != nil {
		logger.Log.Errorf("An error occurred while opening ratings.. %v", err)
	} else {

		parserrating := csv.NewReader(filerating)
		parserrating.Comma = '\t'
		parserrating.LazyQuotes = true
		parserrating.ReuseRecord = true
		parserrating.TrimLeadingSpace = true
		_, _ = parserrating.Read() //skip header
		ratings := make([]database.ImdbRatings, 0, cacherowlimit)

		//namedrating, _ := database.DBImdb.PrepareNamed("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (:tconst, :num_votes, :average_rating)")

		swrating := sizedwaitgroup.New(4)
		for {
			record, err := parserrating.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				logger.Log.Errorf("An error occurred while parsing rating.. %v", err)
				continue
			}
			swrating.Add()
			if len(ratings) >= cacherowlimit {
				database.ReadWriteMu.Lock()
				_, err := database.DBImdb.NamedExec("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (:tconst, :num_votes, :average_rating)", ratings)
				database.ReadWriteMu.Unlock()
				if err != nil {
					logger.Log.Errorf("An error occurred while inserting rating.. %v", err)
					break
				}
				ratings = make([]database.ImdbRatings, 0, cacherowlimit)
			}
			//titlecount, _ := database.ImdbCountRows("imdb_titles", database.Query{Where: "tconst = ?", WhereArgs: []interface{}{record[0]}})
			if _, ok := cachetconst[record[0]]; ok {
				numvotes, _ := strconv.Atoi(csvsetdefault(record[2], "0"))
				AverageRating, _ := strconv.ParseFloat(csvsetdefault(record[1], "0"), 32)
				ratings = append(ratings, database.ImdbRatings{
					Tconst:        record[0],
					AverageRating: float32(AverageRating),
					NumVotes:      numvotes,
				})
			}
			swrating.Done()
		}
		swrating.Wait()
		if len(ratings) >= 1 {
			database.ReadWriteMu.Lock()
			database.DBImdb.NamedExec("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (:tconst, :num_votes, :average_rating)", ratings)
			database.ReadWriteMu.Unlock()
		}
	}
	database.DBImdb.Close()
	os.Remove("./imdb.db")
	os.Rename("./imdbtemp.db", "./imdb.db")
	dbnew := database.InitImdbdb("info", "imdb")
	dbnew.SetMaxOpenConns(5)
	database.DBImdb = dbnew
	logger.Log.Info("Ended Imdb Import")
}

func downloadimdbfiles() {
	downloadFile("./", "", "title.basics.tsv.gz", "https://datasets.imdbws.com/title.basics.tsv.gz")
	gunzip("./title.basics.tsv.gz", "title.basics.tsv")
	scanner.RemoveFile("./title.basics.tsv.gz")

	downloadFile("./", "", "title.akas.tsv.gz", "https://datasets.imdbws.com/title.akas.tsv.gz")
	gunzip("./title.akas.tsv.gz", "title.akas.tsv")
	scanner.RemoveFile("./title.akas.tsv.gz")

	downloadFile("./", "", "title.ratings.tsv.gz", "https://datasets.imdbws.com/title.ratings.tsv.gz")
	gunzip("./title.ratings.tsv.gz", "title.ratings.tsv")
	scanner.RemoveFile("./title.ratings.tsv.gz")
}

func gunzip(source string, target string) {
	data, _ := ioutil.ReadFile(source)
	buffer := bytes.NewBuffer(data)
	gz(buffer, target)
}

func gz(body io.Reader, location string) error {
	reader, err := gzip.NewReader(body)
	if err != nil {
		fmt.Println("err1. ", err)
		return err
	}

	body, _, err = match(reader)
	if err != nil {
		fmt.Println("err2. ", err)
		return err
	}

	err = copy(location, 0666, body)
	if err != nil {
		fmt.Println("err3. ", err)
		return err
	}
	return nil
}

func copy(path string, mode os.FileMode, src io.Reader) error {
	// We add the execution permission to be able to create files inside it
	err := os.MkdirAll(filepath.Dir(path), mode|os.ModeDir|100)
	if err != nil {
		fmt.Println("err4. ", err)
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		fmt.Println("err5. ", err)
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, src)
	return err
}

// match reads the first 512 bytes, calls types.Match and returns a reader
// for the whole stream
func match(r io.Reader) (io.Reader, types.Type, error) {
	buffer := make([]byte, 512)

	n, err := r.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, types.Unknown, err
	}

	r = io.MultiReader(bytes.NewBuffer(buffer[:n]), r)

	typ, err := filetype.Match(buffer)

	return r, typ, err
}
