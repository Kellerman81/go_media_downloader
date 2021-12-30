package main

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/types"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/remeh/sizedwaitgroup"
)

func csvsetdefault(instr string, def string) string {
	if instr == `\N` {
		instr = def
	}
	return instr
}

const configfile string = "config.toml"

type imdbConfig struct {
	Indexedtypes     []string `koanf:"indexed_types"`
	Indexedlanguages []string `koanf:"indexed_languages"`
	Indexfull        bool     `koanf:"index_full"`
}
type imdbTitle struct {
	Tconst         string
	TitleType      string `db:"title_type"`
	PrimaryTitle   string `db:"primary_title"`
	Slug           string
	OriginalTitle  string `db:"original_title"`
	IsAdult        bool   `db:"is_adult"`
	StartYear      int    `db:"start_year"`
	EndYear        int    `db:"end_year"`
	RuntimeMinutes int    `db:"runtime_minutes"`
	Genres         string
}
type imdbAka struct {
	ID              uint
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
	Tconst          string
	Ordering        int
	Title           string
	Slug            string
	Region          string
	Language        string
	Types           string
	Attributes      string
	IsOriginalTitle bool `db:"is_original_title"`
}

type imdbRatings struct {
	ID            uint
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
	Tconst        string
	NumVotes      int     `db:"num_votes"`
	AverageRating float32 `db:"average_rating"`
}
type imdbGenres struct {
	ID        uint
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	Tconst    string
	Genre     string
}

func LoadCfgDataDB() imdbConfig {
	var k = koanf.New(".")
	f := file.Provider(configfile)
	// if strings.Contains(parser, "json") {
	// 	err := k.Load(f, json.Parser())
	// 	if err != nil {
	// 		fmt.Println("Error loading config. ", err)
	// 		return Cfg{}
	// 	}
	// }
	if strings.Contains(configfile, "toml") {
		err := k.Load(f, toml.Parser())
		if err != nil {
			fmt.Println("Error loading config. ", err)
		}
	}
	// if strings.Contains(parser, "yaml") {
	// 	err := k.Load(f, yaml.Parser())
	// 	if err != nil {
	// 		fmt.Println("Error loading config. ", err)
	// 		return Cfg{}
	// 	}
	// }

	if k.Sprint() == "" {
		fmt.Println("Error loading config. Config Empty")
	}
	var outim imdbConfig
	errimdb := k.Unmarshal("imdbindexer", &outim)
	if errimdb == nil {
		return outim
	}
	return imdbConfig{}
}

var dbimdb *sqlx.DB

func initImdbdb(dbloglevel string, dbfile string) *sqlx.DB {
	if _, err := os.Stat("./" + dbfile + ".db"); os.IsNotExist(err) {
		_, err := os.Create("./" + dbfile + ".db") // Create SQLite file
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	db, err := sqlx.Connect("sqlite3", "file:"+dbfile+".db?_fk=1&_mutex=no&_cslike=0")
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(1)
	return db
}

func main() {
	var cfg_imdb imdbConfig = LoadCfgDataDB()
	fmt.Println("Started Imdb Import")
	titlemap := make(map[string]bool, 10)
	for _, row := range cfg_imdb.Indexedtypes {
		titlemap[row] = true
	}
	akamap := make(map[string]bool, 10)
	for _, row := range cfg_imdb.Indexedlanguages {
		akamap[row] = true
	}
	os.Remove("./imdbtemp.db")
	dbget := initImdbdb("info", "imdbtemp")
	dbimdb = dbget

	readWriteMu := &sync.Mutex{}
	m, err := migrate.New(
		"file://./schema/imdbdb",
		"sqlite3://imdbtemp.db?cache=shared&_fk=1&_mutex=no&_cslike=0",
	)
	if err != nil {
		fmt.Println(fmt.Errorf("migration failed... %v", err))
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		fmt.Println(fmt.Errorf("an error occurred while syncing the database.. %v", err))
	}

	downloadimdbfiles()

	cacherowlimit := 1999

	fmt.Println("Opening titles..")
	filetitle, err := os.Open("./title.basics.tsv")
	cachetconst := make(map[string]bool, 9000000)
	if err != nil {
		fmt.Println(fmt.Errorf("an error occurred while opening titles.. %v", err))
	} else {
		parsertitle := csv.NewReader(filetitle)
		parsertitle.Comma = '\t'
		parsertitle.LazyQuotes = true
		parsertitle.ReuseRecord = true
		parsertitle.TrimLeadingSpace = true
		_, _ = parsertitle.Read() //skip header

		titlesshort := make([]imdbTitle, 0, cacherowlimit)
		genres := make([]imdbGenres, 0, cacherowlimit)
		swtitle := sizedwaitgroup.New(4)
		// namedtitle, _ := database.dbimdb.PrepareNamed("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES (:tconst, :title_type, :primary_title, :slug, :original_title, :is_adult, :start_year, :end_year, :runtime_minutes, :genres)")
		// namedgenre, _ := database.dbimdb.PrepareNamed("insert into imdb_genres (tconst, genre) VALUES (:tconst, :genre)")

		for {
			record, err := parsertitle.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Println(fmt.Errorf("an error occurred while parsing title.. %v", err))
				continue
			}
			swtitle.Add()
			if len(titlesshort) >= cacherowlimit {
				readWriteMu.Lock()
				_, err := dbimdb.NamedExec("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES (:tconst, :title_type, :primary_title, :slug, :original_title, :is_adult, :start_year, :end_year, :runtime_minutes, :genres)", titlesshort)
				readWriteMu.Unlock()
				if err != nil {
					fmt.Println(fmt.Errorf("an error occurred while inserting titles.. %v", err))
					break
				}
				titlesshort = make([]imdbTitle, 0, cacherowlimit)
			}
			if len(genres) >= cacherowlimit {
				readWriteMu.Lock()
				_, err := dbimdb.NamedExec("insert into imdb_genres (tconst, genre) VALUES (:tconst, :genre)", genres)
				readWriteMu.Unlock()
				if err != nil {
					fmt.Println(fmt.Errorf("an error occurred while inserting genres.. %v", err))
					break
				}
				genres = make([]imdbGenres, 0, cacherowlimit)
			}
			if _, ok := titlemap[record[1]]; ok {
				readWriteMu.Lock()
				cachetconst[record[0]] = true
				readWriteMu.Unlock()
				startYear, _ := strconv.Atoi(csvsetdefault(record[5], "0"))
				stringtitletype := csvsetdefault(record[1], "")
				stringtitleprimary := html.UnescapeString(csvsetdefault(record[2], ""))
				stringtitleprimaryslug := logger.StringToSlug(stringtitleprimary)
				if !cfg_imdb.Indexfull {
					titlesshort = append(titlesshort, imdbTitle{
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
						fmt.Println("Date error: ", record[6], " ", errdate)
						continue
					}
					runtimeMinutes, errrun := strconv.Atoi(csvsetdefault(record[7], "0"))
					if errrun != nil {
						fmt.Println("Runtime error: ", record[7], " ", errrun)
						continue
					}
					isAdult, erradu := strconv.ParseBool(csvsetdefault(record[4], "0"))
					if erradu != nil {
						fmt.Println("Adult error: ", record[4], " ", erradu)
						continue
					}
					titlesshort = append(titlesshort, imdbTitle{
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
						genreentry := imdbGenres{
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
			readWriteMu.Lock()
			dbimdb.NamedExec("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES (:tconst, :title_type, :primary_title, :slug, :original_title, :is_adult, :start_year, :end_year, :runtime_minutes, :genres)", titlesshort)
			readWriteMu.Unlock()
		}
		if len(genres) >= 1 {
			readWriteMu.Lock()
			dbimdb.NamedExec("insert into imdb_genres (tconst, genre) VALUES (:tconst, :genre)", genres)
			readWriteMu.Unlock()
		}
	}

	fmt.Println("Opening akas..")
	fileaka, err := os.Open("./title.akas.tsv")
	if err != nil {
		fmt.Println(fmt.Errorf("an error occurred while opening akas.. %v", err))
	} else {

		parseraka := csv.NewReader(fileaka)
		parseraka.Comma = '\t'
		parseraka.LazyQuotes = true
		parseraka.ReuseRecord = true
		parseraka.TrimLeadingSpace = true
		_, _ = parseraka.Read() //skip header
		akasshort := make([]imdbAka, 0, cacherowlimit)

		swaka := sizedwaitgroup.New(4)
		for {
			record, err := parseraka.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Println(fmt.Errorf("an error occurred while parsing aka.. %v", err))
				continue
			}
			swaka.Add()
			if len(akasshort) >= cacherowlimit {
				readWriteMu.Lock()
				_, err := dbimdb.NamedExec("insert into imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES (:tconst, :ordering, :title, :slug, :region, :language, :types, :attributes, :is_original_title)", akasshort)
				readWriteMu.Unlock()
				if err != nil {
					fmt.Println(fmt.Errorf("an error occurred while inserting aka.. %v", err))
					break
				}
				akasshort = make([]imdbAka, 0, cacherowlimit)
			}

			if _, ok := akamap[record[3]]; ok || len(record[3]) == 0 {
				//titlecount, _ := database.ImdbCountRows("imdb_titles", database.Query{Where: "tconst = ?", WhereArgs: []interface{}{record[0]}})
				if _, ok := cachetconst[record[0]]; ok {
					stringtitle := html.UnescapeString(csvsetdefault(record[2], ""))
					stringtitleslug := logger.StringToSlug(stringtitle)
					stringregion := csvsetdefault(record[3], "")
					if !cfg_imdb.Indexfull {
						akasshort = append(akasshort, imdbAka{
							Tconst: record[0],
							Title:  stringtitle,
							Slug:   stringtitleslug,
							Region: stringregion,
						})
					} else {
						stringlanguage := csvsetdefault(record[4], "")
						stringtypes := csvsetdefault(record[5], "")
						stringattributes := csvsetdefault(record[6], "")
						ordering, _ := strconv.Atoi(csvsetdefault(record[1], "0"))
						isOriginalTitle, _ := strconv.ParseBool(csvsetdefault(record[7], "0"))
						akasshort = append(akasshort, imdbAka{
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
			readWriteMu.Lock()
			dbimdb.NamedExec("insert into imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES (:tconst, :ordering, :title, :slug, :region, :language, :types, :attributes, :is_original_title)", akasshort)
			readWriteMu.Unlock()
		}
	}

	fmt.Println("Opening ratings..")
	filerating, err := os.Open("./title.ratings.tsv")
	if err != nil {
		fmt.Println(fmt.Errorf("an error occurred while opening ratings.. %v", err))
	} else {

		parserrating := csv.NewReader(filerating)
		parserrating.Comma = '\t'
		parserrating.LazyQuotes = true
		parserrating.ReuseRecord = true
		parserrating.TrimLeadingSpace = true
		_, _ = parserrating.Read() //skip header
		ratings := make([]imdbRatings, 0, cacherowlimit)

		//namedrating, _ := database.dbimdb.PrepareNamed("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (:tconst, :num_votes, :average_rating)")

		swrating := sizedwaitgroup.New(4)
		for {
			record, err := parserrating.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Println(fmt.Errorf("an error occurred while parsing rating.. %v", err))
				continue
			}
			swrating.Add()
			if len(ratings) >= cacherowlimit {
				readWriteMu.Lock()
				_, err := dbimdb.NamedExec("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (:tconst, :num_votes, :average_rating)", ratings)
				readWriteMu.Unlock()
				if err != nil {
					fmt.Println(fmt.Errorf("an error occurred while inserting rating.. %v", err))
					break
				}
				ratings = make([]imdbRatings, 0, cacherowlimit)
			}
			//titlecount, _ := database.ImdbCountRows("imdb_titles", database.Query{Where: "tconst = ?", WhereArgs: []interface{}{record[0]}})
			if _, ok := cachetconst[record[0]]; ok {
				numvotes, _ := strconv.Atoi(csvsetdefault(record[2], "0"))
				AverageRating, _ := strconv.ParseFloat(csvsetdefault(record[1], "0"), 32)
				ratings = append(ratings, imdbRatings{
					Tconst:        record[0],
					AverageRating: float32(AverageRating),
					NumVotes:      numvotes,
				})
			}
			swrating.Done()
		}
		swrating.Wait()
		if len(ratings) >= 1 {
			readWriteMu.Lock()
			dbimdb.NamedExec("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (:tconst, :num_votes, :average_rating)", ratings)
			readWriteMu.Unlock()
		}
	}

	rows, err := dbimdb.Query("Select count(*) from imdb_titles")
	if err != nil {
		dbimdb.Close()
		os.Remove("./imdbtemp.db")
		return
	}
	defer rows.Close()
	rows.Next()
	var counter int
	rows.Scan(&counter)
	if counter == 0 {
		dbimdb.Close()
		os.Remove("./imdbtemp.db")
		return
	}
	dbimdb.Close()
	fmt.Println("Ended Imdb Import")
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

func RemoveFile(file string) error {
	var err error
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		err := os.Remove(file)
		if err != nil {
			fmt.Println("File could not be removed: ", file, " Error: ", err)
		} else {
			fmt.Println("File removed: ", file)
		}
	} else {
		fmt.Println("File not found: ", file)
	}
	return err
}

func downloadimdbfiles() {
	downloadFile("./", "", "title.basics.tsv.gz", "https://datasets.imdbws.com/title.basics.tsv.gz")
	gunzip("./title.basics.tsv.gz", "title.basics.tsv")
	RemoveFile("./title.basics.tsv.gz")

	downloadFile("./", "", "title.akas.tsv.gz", "https://datasets.imdbws.com/title.akas.tsv.gz")
	gunzip("./title.akas.tsv.gz", "title.akas.tsv")
	RemoveFile("./title.akas.tsv.gz")

	downloadFile("./", "", "title.ratings.tsv.gz", "https://datasets.imdbws.com/title.ratings.tsv.gz")
	gunzip("./title.ratings.tsv.gz", "title.ratings.tsv")
	RemoveFile("./title.ratings.tsv.gz")
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
