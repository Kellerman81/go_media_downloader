package utils

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"log"
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
		"sqlite3://imdbtemp.db?cache=shared&_fk=1&_txlock=immediate&_mutex=full&_cslike=0",
	)
	if err != nil {
		log.Fatalf("migration failed... %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("An error occurred while syncing the database.. %v", err)
	}
	dbget2 := database.InitImdbdb("info", "imdbtemp")
	database.DBImdb = dbget2
	// db.Debug().AutoMigrate(&database.ImdbTitle{})
	// db.Debug().AutoMigrate(&database.ImdbAka{})
	// db.Debug().AutoMigrate(&database.ImdbGenres{})
	// db.Debug().AutoMigrate(&database.ImdbRatings{})

	downloadimdbfiles()

	filetitle, err := os.Open("./title.basics.tsv")
	if err != nil {
		log.Fatal(err)
	}

	parsertitle := csv.NewReader(filetitle)
	parsertitle.Comma = '\t'
	parsertitle.LazyQuotes = true
	_, _ = parsertitle.Read() //skip header
	titlesshort := []database.ImdbTitle{}
	genres := []database.ImdbGenres{}
	swtitle := sizedwaitgroup.New(4)

	for {
		record, err := parsertitle.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("error: ", err, record)
			continue
		}
		swtitle.Add()
		if len(titlesshort) >= 1000 {
			database.ReadWriteMu.Lock()
			database.DBImdb.NamedExec("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES (:tconst, :title_type, :primary_title, :slug, :original_title, :is_adult, :start_year, :end_year, :runtime_minutes, :genres)", titlesshort)
			database.ReadWriteMu.Unlock()
			// for idxc := range titlesshort {
			// 	database.ImdbInsertArray(
			// 		"imdb_titles",
			// 		[]string{"Tconst", "title_type", "primary_title", "slug", "original_title", "is_adult", "start_year", "end_year", "runtime_minutes", "genres"},
			// 		[]interface{}{titlesshort[idxc].Tconst, titlesshort[idxc].TitleType, titlesshort[idxc].PrimaryTitle, titlesshort[idxc].Slug, titlesshort[idxc].OriginalTitle, titlesshort[idxc].IsAdult, titlesshort[idxc].StartYear, titlesshort[idxc].EndYear, titlesshort[idxc].RuntimeMinutes, titlesshort[idxc].Genres})
			// }
			titlesshort = []database.ImdbTitle{}
		}
		if len(genres) >= 1000 {
			database.ReadWriteMu.Lock()
			database.DBImdb.NamedExec("insert into imdb_genres (tconst, genre) VALUES (:tconst, :genre)", genres)
			database.ReadWriteMu.Unlock()
			// for idxc := range genres {
			// 	database.ImdbInsertArray(
			// 		"imdb_genres",
			// 		[]string{"Tconst", "genre"},
			// 		[]interface{}{genres[idxc].Tconst, genres[idxc].Genre})
			// }
			genres = []database.ImdbGenres{}
		}
		if _, ok := titlemap[record[1]]; ok {
			// logger.Log.Info("1 error: ", record[0])
			// logger.Log.Info("2 error: ", record[1], " ")
			// logger.Log.Info("3 error: ", record[2], " ")
			// logger.Log.Info("4 error: ", record[3], " ")
			// logger.Log.Info("5 error: ", record[4], " ")
			// logger.Log.Info("6 error: ", record[5], " ")
			// logger.Log.Info("7 error: ", record[6], " ")
			// logger.Log.Info("8 error: ", record[7], " ")
			// logger.Log.Info("9 error: ", record[8], " ")
			// swtitle.Done()
			// break
			if record[5] == `\N` || record[5] == `\\N` {
				record[5] = "0"
			}
			startYear, _ := strconv.Atoi(record[5])
			stringtitletype := record[1]
			if stringtitletype == `\N` || record[1] == `\\N` {
				stringtitletype = ""
			}
			stringtitleprimary := record[2]
			if stringtitleprimary == `\N` || record[2] == `\\N` {
				stringtitleprimary = ""
			}
			if !cfg_imdb.Indexfull {
				titlesshort = append(titlesshort, database.ImdbTitle{
					Tconst:       record[0],
					TitleType:    stringtitletype,
					PrimaryTitle: stringtitleprimary,
					Slug:         logger.StringToSlug(stringtitleprimary),
					StartYear:    startYear,
				})
			} else {
				if record[6] == `\N` || record[6] == `\\N` {
					record[6] = "0"
				}
				if record[7] == `\N` || record[7] == `\\N` {
					record[7] = "0"
				}
				if record[4] == `\N` || record[4] == `\\N` {
					record[4] = "0"
				}
				stringtitleoriginal := record[2]
				if stringtitleoriginal == `\N` || record[2] == `\\N` {
					stringtitleoriginal = ""
				}
				stringgenre := record[8]
				if stringgenre == `\N` || record[8] == `\\N` {
					stringgenre = ""
				}
				endYear, errdate := strconv.Atoi(record[6])
				if errdate != nil {
					logger.Log.Error("Date error: ", record[6], " ", errdate)
				}
				runtimeMinutes, errrun := strconv.Atoi(record[7])
				if errrun != nil {
					logger.Log.Error("Runtime error: ", record[7], " ", errrun)
				}
				isAdult, erradu := strconv.ParseBool(record[4])
				if erradu != nil {
					logger.Log.Error("Adult error: ", record[4], " ", erradu)
				}

				titlesshort = append(titlesshort, database.ImdbTitle{
					Tconst:         record[0],
					TitleType:      stringtitletype,
					PrimaryTitle:   stringtitleprimary,
					Slug:           logger.StringToSlug(stringtitleprimary),
					OriginalTitle:  stringtitleoriginal,
					Genres:         stringgenre,
					IsAdult:        isAdult,
					StartYear:      startYear,
					RuntimeMinutes: runtimeMinutes,
					EndYear:        endYear,
				})
				if strings.Contains(stringgenre, ",") {
					genrearray := strings.Split(stringgenre, ",")
					for idxgenre := range genrearray {
						genres = append(genres, database.ImdbGenres{
							Tconst: record[0],
							Genre:  genrearray[idxgenre],
						})
					}
				} else if len(stringgenre) >= 1 {
					genres = append(genres, database.ImdbGenres{
						Tconst: record[0],
						Genre:  stringgenre,
					})
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
		// for idxc := range titlesshort {
		// 	database.ImdbInsertArray(
		// 		"imdb_titles",
		// 		[]string{"Tconst", "title_type", "primary_title", "slug", "original_title", "is_adult", "start_year", "end_year", "runtime_minutes", "genres"},
		// 		[]interface{}{titlesshort[idxc].Tconst, titlesshort[idxc].TitleType, titlesshort[idxc].PrimaryTitle, titlesshort[idxc].Slug, titlesshort[idxc].OriginalTitle, titlesshort[idxc].IsAdult, titlesshort[idxc].StartYear, titlesshort[idxc].EndYear, titlesshort[idxc].RuntimeMinutes, titlesshort[idxc].Genres})
		// }
	}
	if len(genres) >= 1 {
		database.ReadWriteMu.Lock()
		database.DBImdb.NamedExec("insert into imdb_genres (tconst, genre) VALUES (:tconst, :genre)", genres)
		database.ReadWriteMu.Unlock()
		// for idxc := range genres {
		// 	database.ImdbInsertArray(
		// 		"imdb_genres",
		// 		[]string{"Tconst", "genre"},
		// 		[]interface{}{genres[idxc].Tconst, genres[idxc].Genre})
		// }
	}

	fileaka, err := os.Open("./title.akas.tsv")
	if err != nil {
		log.Fatal(err)
	}

	parseraka := csv.NewReader(fileaka)
	parseraka.Comma = '\t'
	parseraka.LazyQuotes = true
	_, _ = parseraka.Read() //skip header
	akasshort := []database.ImdbAka{}
	swaka := sizedwaitgroup.New(4)
	for {
		record, err := parseraka.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("error: ", err, record)
			continue
		}
		swaka.Add()
		if len(akasshort) >= 1000 {
			database.ReadWriteMu.Lock()
			database.DBImdb.NamedExec("insert into imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES (:tconst, :ordering, :title, :slug, :region, :language, :types, :attributes, :is_original_title)", akasshort)
			database.ReadWriteMu.Unlock()
			// for idxc := range akasshort {
			// 	database.ImdbInsertArray(
			// 		"imdb_akas",
			// 		[]string{"Tconst", "ordering", "title", "slug", "region", "language", "types", "attributes", "is_original_title"},
			// 		[]interface{}{akasshort[idxc].Tconst, akasshort[idxc].Ordering, akasshort[idxc].Title, akasshort[idxc].Slug, akasshort[idxc].Region, akasshort[idxc].Language, akasshort[idxc].Types, akasshort[idxc].Attributes, akasshort[idxc].IsOriginalTitle})
			// }
			akasshort = []database.ImdbAka{}
		}

		if _, ok := akamap[record[3]]; ok || len(record[3]) == 0 {
			if record[1] == `\N` || record[1] == `\\N` {
				record[1] = "0"
			}
			if record[7] == `\N` || record[7] == `\\N` {
				record[7] = "0"
			}
			_, dbtitleerr := database.GetImdbTitle(database.Query{Select: "tconst", Where: "Tconst = ?", WhereArgs: []interface{}{record[0]}})
			if dbtitleerr == nil {
				stringtitle := record[2]
				if stringtitle == `\N` || record[2] == `\\N` {
					stringtitle = ""
				}
				stringregion := record[3]
				if stringregion == `\N` || record[3] == `\\N` {
					stringregion = ""
				}
				stringlanguage := record[4]
				if stringlanguage == `\N` || record[4] == `\\N` {
					stringlanguage = ""
				}
				stringtypes := record[5]
				if stringtypes == `\N` || record[5] == `\\N` {
					stringtypes = ""
				}
				stringattributes := record[6]
				if stringattributes == `\N` || record[6] == `\\N` {
					stringattributes = ""
				}
				if !cfg_imdb.Indexfull {
					akasshort = append(akasshort, database.ImdbAka{
						Tconst: record[0],
						Title:  stringtitle,
						Slug:   logger.StringToSlug(stringtitle),
						Region: stringregion,
					})
				} else {
					ordering, _ := strconv.Atoi(record[1])
					isOriginalTitle, _ := strconv.ParseBool(record[7])
					akasshort = append(akasshort, database.ImdbAka{
						Tconst:          record[0],
						Ordering:        ordering,
						Title:           stringtitle,
						Slug:            logger.StringToSlug(stringtitle),
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
		// for idxc := range akasshort {
		// 	database.ImdbInsertArray(
		// 		"imdb_akas",
		// 		[]string{"Tconst", "ordering", "title", "slug", "region", "language", "types", "attributes", "is_original_title"},
		// 		[]interface{}{akasshort[idxc].Tconst, akasshort[idxc].Ordering, akasshort[idxc].Title, akasshort[idxc].Slug, akasshort[idxc].Region, akasshort[idxc].Language, akasshort[idxc].Types, akasshort[idxc].Attributes, akasshort[idxc].IsOriginalTitle})
		// }
	}

	filerating, err := os.Open("./title.ratings.tsv")
	if err != nil {
		log.Fatal(err)
	}

	parserrating := csv.NewReader(filerating)
	parserrating.Comma = '\t'
	parserrating.LazyQuotes = true
	_, _ = parserrating.Read() //skip header
	ratings := []database.ImdbRatings{}
	swrating := sizedwaitgroup.New(4)
	for {
		record, err := parserrating.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("error: ", err, record)
			continue
		}
		swrating.Add()
		if len(ratings) >= 1000 {
			database.ReadWriteMu.Lock()
			database.DBImdb.NamedExec("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (:tconst, :num_votes, :average_rating)", ratings)
			database.ReadWriteMu.Unlock()
			// for idxc := range ratings {
			// 	database.ImdbInsertArray(
			// 		"imdb_ratings",
			// 		[]string{"Tconst", "num_votes", "average_rating"},
			// 		[]interface{}{ratings[idxc].Tconst, ratings[idxc].NumVotes, ratings[idxc].AverageRating})
			// }
			ratings = []database.ImdbRatings{}
		}

		_, dbtitleerr := database.GetImdbTitle(database.Query{Select: "tconst", Where: "Tconst = ?", WhereArgs: []interface{}{record[0]}})

		if dbtitleerr == nil {
			if record[2] == `\N` || record[2] == `\\N` {
				record[2] = "0"
			}
			if record[1] == `\N` || record[1] == `\\N` {
				record[1] = "0"
			}
			numvotes, _ := strconv.Atoi(record[2])
			AverageRating, _ := strconv.ParseFloat(record[1], 32)
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
		// for idxc := range ratings {
		// 	database.ImdbInsertArray(
		// 		"imdb_ratings",
		// 		[]string{"Tconst", "num_votes", "average_rating"},
		// 		[]interface{}{ratings[idxc].Tconst, ratings[idxc].NumVotes, ratings[idxc].AverageRating})
		// }
	}

	database.DBImdb.Close()
	os.Remove("./imdb.db")
	os.Rename("./imdbtemp.db", "./imdb.db")
	dbnew := database.InitImdbdb("info", "imdb")
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
