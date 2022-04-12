package main

import (
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var version string
var buildstamp string
var githash string

func csvsetdefault(instr string, def string) string {
	if instr == `\N` {
		instr = def
	}
	return instr
}

func csvgetint(instr string) int {
	int, _ := strconv.Atoi(csvsetdefault(instr, "0"))
	return int
}
func csvgetfloat(instr string) float64 {
	flo, _ := strconv.ParseFloat(csvsetdefault(instr, "0"), 32)
	return flo
}

func csvgetbool(instr string) bool {
	bo, _ := strconv.ParseBool(csvsetdefault(instr, "0"))
	return bo
}

const configfile string = "./config/config.toml"

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
	k = nil
	f = nil
	if errimdb == nil {
		return outim
	}
	return imdbConfig{}
}

var dbimdb *sqlx.DB

func initImdbdb(dbloglevel string, dbfile string) *sqlx.DB {
	if _, err := os.Stat("./databases/" + dbfile + ".db"); os.IsNotExist(err) {
		_, err := os.Create("./databases/" + dbfile + ".db") // Create SQLite file
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	db, err := sqlx.Connect("sqlite3", "file:./databases/"+dbfile+".db?_fk=1&_mutex=no&_cslike=0")
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxIdleConns(15)
	db.SetMaxOpenConns(5)
	return db
}

// func findimdb(find uint32) bool {
// 	for idx := range cachetconst {
// 		if cachetconst[idx] == find {
// 			return true
// 		}
// 	}
// 	return false
// }

func main() {
	fmt.Println("Imdb Importer by kellerman81 - version " + version + " " + githash + " from " + buildstamp)
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
	os.Remove("./databases/imdbtemp.db")
	dbget := initImdbdb("info", "imdbtemp")
	dbimdb = dbget

	readWriteMu := &sync.Mutex{}
	m, err := migrate.New(
		"file://./schema/imdbdb",
		"sqlite3://./databases/imdbtemp.db?cache=shared&_fk=1&_mutex=no&_cslike=0",
	)
	if err != nil {
		fmt.Println(fmt.Errorf("migration failed... %v", err))
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		fmt.Println(fmt.Errorf("an error occurred while syncing the database.. %v", err))
	}
	m = nil

	downloadimdbfiles()

	cacherowlimit := 1999

	fmt.Println("Opening titles..")
	filetitle, err := os.Open("./temp/title.basics.tsv")
	cachetconst := make(map[uint32]interface{}, 1000000)
	if err != nil {
		fmt.Println(fmt.Errorf("an error occurred while opening titles.. %v", err))
	} else {
		parsertitle := csv.NewReader(bufio.NewReader(filetitle))
		parsertitle.Comma = '\t'
		parsertitle.LazyQuotes = true
		parsertitle.ReuseRecord = true
		parsertitle.TrimLeadingSpace = true
		_, _ = parsertitle.Read() //skip header

		titlesshort := make([]imdbTitle, 0, cacherowlimit)
		genres := make([]imdbGenres, 0, cacherowlimit)
		// namedtitle, _ := database.dbimdb.PrepareNamed("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES (:tconst, :title_type, :primary_title, :slug, :original_title, :is_adult, :start_year, :end_year, :runtime_minutes, :genres)")
		// namedgenre, _ := database.dbimdb.PrepareNamed("insert into imdb_genres (tconst, genre) VALUES (:tconst, :genre)")
		var record []string
		var err error
		for {
			record, err = parsertitle.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Println(fmt.Errorf("an error occurred while parsing title.. %v", err))
				continue
			}
			if len(titlesshort) >= cacherowlimit {
				readWriteMu.Lock()
				_, err := dbimdb.NamedExec("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES (:tconst, :title_type, :primary_title, :slug, :original_title, :is_adult, :start_year, :end_year, :runtime_minutes, :genres)", titlesshort)
				readWriteMu.Unlock()
				if err != nil {
					fmt.Println(fmt.Errorf("an error occurred while inserting titles.. %v", err))
					continue
				}
				titlesshort = nil
				titlesshort = make([]imdbTitle, 0, cacherowlimit)
			}
			if len(genres) >= cacherowlimit {
				readWriteMu.Lock()
				_, err := dbimdb.NamedExec("insert into imdb_genres (tconst, genre) VALUES (:tconst, :genre)", genres)
				readWriteMu.Unlock()
				if err != nil {
					fmt.Println(fmt.Errorf("an error occurred while inserting genres.. %v", err))
					continue
				}
				genres = nil
				genres = make([]imdbGenres, 0, cacherowlimit)
			}
			if _, ok := titlemap[record[1]]; ok {
				//readWriteMu.Lock()
				cachetconst[uint32(csvgetint(strings.Trim(record[0], "t")))] = nil
				//readWriteMu.Unlock()
				if !cfg_imdb.Indexfull {
					titlesshort = append(titlesshort, imdbTitle{
						Tconst:       record[0],
						TitleType:    csvsetdefault(record[1], ""),
						PrimaryTitle: UnescapeString(csvsetdefault(record[2], "")),
						Slug:         StringToSlug(UnescapeString(csvsetdefault(record[2], ""))),
						StartYear:    csvgetint(record[5]),
					})
				} else {
					titlesshort = append(titlesshort, imdbTitle{
						Tconst:         record[0],
						TitleType:      csvsetdefault(record[1], ""),
						PrimaryTitle:   UnescapeString(csvsetdefault(record[2], "")),
						Slug:           StringToSlug(UnescapeString(csvsetdefault(record[2], ""))),
						OriginalTitle:  UnescapeString(csvsetdefault(record[2], "")),
						Genres:         csvsetdefault(record[8], ""),
						IsAdult:        csvgetbool(record[4]),
						StartYear:      csvgetint(record[5]),
						RuntimeMinutes: csvgetint(record[7]),
						EndYear:        csvgetint(record[6]),
					})
					var genrearray []string
					if strings.Contains(csvsetdefault(record[8], ""), ",") {
						genrearray = strings.Split(csvsetdefault(record[8], ""), ",")
					} else if len(csvsetdefault(record[8], "")) >= 1 {
						genrearray = []string{csvsetdefault(record[8], "")}
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
		}
		for key := range titlemap {
			delete(titlemap, key)
		}
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

	filetitle.Close()

	fmt.Println("Opening akas..")
	fileaka, err := os.Open("./temp/title.akas.tsv")
	if err != nil {
		fmt.Println(fmt.Errorf("an error occurred while opening akas.. %v", err))
	} else {

		parseraka := csv.NewReader(bufio.NewReader(fileaka))
		parseraka.Comma = '\t'
		parseraka.LazyQuotes = true
		parseraka.ReuseRecord = true
		parseraka.TrimLeadingSpace = true
		_, _ = parseraka.Read() //skip header
		akasshort := make([]imdbAka, 0, cacherowlimit)

		var record []string
		var err error
		for {
			record, err = parseraka.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Println(fmt.Errorf("an error occurred while parsing aka.. %v", err))
				continue
			}
			if len(akasshort) >= cacherowlimit {
				readWriteMu.Lock()
				_, err = dbimdb.NamedExec("insert into imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES (:tconst, :ordering, :title, :slug, :region, :language, :types, :attributes, :is_original_title)", akasshort)
				readWriteMu.Unlock()
				if err != nil {
					fmt.Println(fmt.Errorf("an error occurred while inserting aka.. %v", err))
					continue
				}
				akasshort = nil
				akasshort = make([]imdbAka, 0, cacherowlimit)
			}

			if _, ok := akamap[record[3]]; ok || len(record[3]) == 0 {
				//titlecount, _ := database.ImdbCountRows("imdb_titles", database.Query{Where: "tconst = ?", WhereArgs: []interface{}{record[0]}})
				if _, ok := cachetconst[uint32(csvgetint(strings.Trim(record[0], "t")))]; ok {
					if !cfg_imdb.Indexfull {
						akasshort = append(akasshort, imdbAka{
							Tconst: record[0],
							Title:  UnescapeString(csvsetdefault(record[2], "")),
							Slug:   StringToSlug(UnescapeString(csvsetdefault(record[2], ""))),
							Region: csvsetdefault(record[3], ""),
						})
					} else {
						akasshort = append(akasshort, imdbAka{
							Tconst:          record[0],
							Ordering:        csvgetint(record[1]),
							Title:           UnescapeString(csvsetdefault(record[2], "")),
							Slug:            StringToSlug(UnescapeString(csvsetdefault(record[2], ""))),
							Region:          csvsetdefault(record[3], ""),
							Language:        csvsetdefault(record[4], ""),
							Types:           csvsetdefault(record[5], ""),
							Attributes:      csvsetdefault(record[6], ""),
							IsOriginalTitle: csvgetbool(record[7]),
						})
					}
				}
			}
		}
		if len(akasshort) >= 1 {
			readWriteMu.Lock()
			dbimdb.NamedExec("insert into imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES (:tconst, :ordering, :title, :slug, :region, :language, :types, :attributes, :is_original_title)", akasshort)
			readWriteMu.Unlock()
		}
		for key := range akamap {
			delete(akamap, key)
		}
	}
	fileaka.Close()

	fmt.Println("Opening ratings..")
	filerating, err := os.Open("./temp/title.ratings.tsv")
	if err != nil {
		fmt.Println(fmt.Errorf("an error occurred while opening ratings.. %v", err))
	} else {

		parserrating := csv.NewReader(bufio.NewReader(filerating))
		parserrating.Comma = '\t'
		parserrating.LazyQuotes = true
		parserrating.ReuseRecord = true
		parserrating.TrimLeadingSpace = true
		_, _ = parserrating.Read() //skip header
		ratings := make([]imdbRatings, 0, cacherowlimit)

		//namedrating, _ := database.dbimdb.PrepareNamed("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (:tconst, :num_votes, :average_rating)")

		var record []string
		var err error
		for {
			record, err = parserrating.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Println(fmt.Errorf("an error occurred while parsing rating.. %v", err))
				continue
			}
			if len(ratings) >= cacherowlimit {
				readWriteMu.Lock()
				_, err = dbimdb.NamedExec("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (:tconst, :num_votes, :average_rating)", ratings)
				readWriteMu.Unlock()
				if err != nil {
					fmt.Println(fmt.Errorf("an error occurred while inserting rating.. %v", err))
					continue
				}
				ratings = nil
				ratings = make([]imdbRatings, 0, cacherowlimit)
			}
			//titlecount, _ := database.ImdbCountRows("imdb_titles", database.Query{Where: "tconst = ?", WhereArgs: []interface{}{record[0]}})
			if _, ok := cachetconst[uint32(csvgetint(strings.Trim(record[0], "t")))]; ok {
				ratings = append(ratings, imdbRatings{
					Tconst:        record[0],
					AverageRating: float32(csvgetfloat(record[1])),
					NumVotes:      csvgetint(record[2]),
				})
			}
		}
		if len(ratings) >= 1 {
			readWriteMu.Lock()
			dbimdb.NamedExec("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (:tconst, :num_votes, :average_rating)", ratings)
			readWriteMu.Unlock()
		}
	}

	filerating.Close()

	for key := range cachetconst {
		delete(cachetconst, key)
	}

	rows, err := dbimdb.Query("Select count(*) from imdb_titles")
	if err != nil {
		dbimdb.Close()
		os.Remove("./databases/imdbtemp.db")
		return
	}
	rows.Next()
	var counter int
	rows.Scan(&counter)
	rows.Close()
	if counter == 0 {
		dbimdb.Close()
		os.Remove("./databases/imdbtemp.db")
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
	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	out.Sync()
	out.Close()
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
	downloadFile("./temp", "", "title.basics.tsv.gz", "https://datasets.imdbws.com/title.basics.tsv.gz")
	gunzip("./temp/title.basics.tsv.gz", "./temp/title.basics.tsv")
	RemoveFile("./temp/title.basics.tsv.gz")

	downloadFile("./temp", "", "title.akas.tsv.gz", "https://datasets.imdbws.com/title.akas.tsv.gz")
	gunzip("./temp/title.akas.tsv.gz", "./temp/title.akas.tsv")
	RemoveFile("./temp/title.akas.tsv.gz")

	downloadFile("./temp", "", "title.ratings.tsv.gz", "https://datasets.imdbws.com/title.ratings.tsv.gz")
	gunzip("./temp/title.ratings.tsv.gz", "./temp/title.ratings.tsv")
	RemoveFile("./temp/title.ratings.tsv.gz")
}

func gunzip(source string, target string) error {

	reader, err := os.Open(source)
	if err != nil {
		return err
	}
	defer reader.Close()

	archive, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer archive.Close()

	writer, err := os.Create(target)
	if err != nil {
		return err
	}
	defer writer.Close()

	_, err = io.Copy(writer, archive)
	writer.Sync()
	return err
}

func StringReplaceArray(instr string, what []string, with string) string {
	for _, line := range what {
		instr = strings.Replace(instr, line, with, -1)
	}
	return instr
}

var unavailableMapping = map[rune]rune{
	'\u0181': 'B',
	'\u1d81': 'd',
	'\u1d85': 'l',
	'\u1d89': 'r',
	'\u028b': 'v',
	'\u1d8d': 'x',
	'\u1d83': 'g',
	'\u0191': 'F',
	'\u0199': 'k',
	'\u019d': 'N',
	'\u0220': 'N',
	'\u01a5': 'p',
	'\u0224': 'Z',
	'\u0126': 'H',
	'\u01ad': 't',
	'\u01b5': 'Z',
	'\u0234': 'l',
	'\u023c': 'c',
	'\u0240': 'z',
	'\u0142': 'l',
	'\u0244': 'U',
	'\u2c60': 'L',
	'\u0248': 'J',
	'\ua74a': 'O',
	'\u024c': 'R',
	'\ua752': 'P',
	'\ua756': 'Q',
	'\ua75a': 'R',
	'\ua75e': 'V',
	'\u0260': 'g',
	'\u01e5': 'g',
	'\u2c64': 'R',
	'\u0166': 'T',
	'\u0268': 'i',
	'\u2c66': 't',
	'\u026c': 'l',
	'\u1d6e': 'f',
	'\u1d87': 'n',
	'\u1d72': 'r',
	'\u2c74': 'v',
	'\u1d76': 'z',
	'\u2c78': 'e',
	'\u027c': 'r',
	'\u1eff': 'y',
	'\ua741': 'k',
	'\u0182': 'B',
	'\u1d86': 'm',
	'\u0288': 't',
	'\u018a': 'D',
	'\u1d8e': 'z',
	'\u0111': 'd',
	'\u0290': 'z',
	'\u0192': 'f',
	'\u1d96': 'i',
	'\u019a': 'l',
	'\u019e': 'n',
	'\u1d88': 'p',
	'\u02a0': 'q',
	'\u01ae': 'T',
	'\u01b2': 'V',
	'\u01b6': 'z',
	'\u023b': 'C',
	'\u023f': 's',
	'\u0141': 'L',
	'\u0243': 'B',
	'\ua745': 'k',
	'\u0247': 'e',
	'\ua749': 'l',
	'\u024b': 'q',
	'\ua74d': 'o',
	'\u024f': 'y',
	'\ua751': 'p',
	'\u0253': 'b',
	'\ua755': 'p',
	'\u0257': 'd',
	'\ua759': 'q',
	'\u00d8': 'O',
	'\u2c63': 'P',
	'\u2c67': 'H',
	'\u026b': 'l',
	'\u1d6d': 'd',
	'\u1d71': 'p',
	'\u0273': 'n',
	'\u1d75': 't',
	'\u1d91': 'd',
	'\u00f8': 'o',
	'\u2c7e': 'S',
	'\u1d7d': 'p',
	'\u2c7f': 'Z',
	'\u0183': 'b',
	'\u0187': 'C',
	'\u1d80': 'b',
	'\u0289': 'u',
	'\u018b': 'D',
	'\u1d8f': 'a',
	'\u0291': 'z',
	'\u0110': 'D',
	'\u0193': 'G',
	'\u1d82': 'f',
	'\u0197': 'I',
	'\u029d': 'j',
	'\u019f': 'O',
	'\u2c6c': 'z',
	'\u01ab': 't',
	'\u01b3': 'Y',
	'\u0236': 't',
	'\u023a': 'A',
	'\u023e': 'T',
	'\ua740': 'K',
	'\u1d8a': 's',
	'\ua744': 'K',
	'\u0246': 'E',
	'\ua748': 'L',
	'\ua74c': 'O',
	'\u024e': 'Y',
	'\ua750': 'P',
	'\ua754': 'P',
	'\u0256': 'd',
	'\ua758': 'Q',
	'\u2c62': 'L',
	'\u0266': 'h',
	'\u2c73': 'w',
	'\u2c6a': 'k',
	'\u1d6c': 'b',
	'\u2c6e': 'M',
	'\u1d70': 'n',
	'\u0272': 'n',
	'\u1d92': 'e',
	'\u1d74': 's',
	'\u2c7a': 'o',
	'\u2c6b': 'Z',
	'\u027e': 'r',
	'\u0180': 'b',
	'\u0282': 's',
	'\u1d84': 'k',
	'\u0188': 'c',
	'\u018c': 'd',
	'\ua742': 'K',
	'\u1d99': 'u',
	'\u0198': 'K',
	'\u1d8c': 'v',
	'\u0221': 'd',
	'\u2c71': 'v',
	'\u0225': 'z',
	'\u01a4': 'P',
	'\u0127': 'h',
	'\u01ac': 'T',
	'\u0235': 'n',
	'\u01b4': 'y',
	'\u2c72': 'W',
	'\u023d': 'L',
	'\ua743': 'k',
	'\u0249': 'j',
	'\ua74b': 'o',
	'\u024d': 'r',
	'\ua753': 'p',
	'\u0255': 'c',
	'\ua757': 'q',
	'\u2c68': 'h',
	'\ua75b': 'r',
	'\ua75f': 'v',
	'\u2c61': 'l',
	'\u2c65': 'a',
	'\u01e4': 'G',
	'\u0167': 't',
	'\u2c69': 'K',
	'\u026d': 'l',
	'\u1d6f': 'm',
	'\u0271': 'm',
	'\u1d73': 'r',
	'\u027d': 'r',
	'\u1efe': 'Y',
}

func mapDecomposeUnavailable(r rune) rune {
	if v, ok := unavailableMapping[r]; ok {
		return v
	}
	return r
}

func UnescapeString(instr string) string {
	if strings.Contains(instr, "&") || strings.Contains(instr, "%") {
		return html.UnescapeString(instr)
	} else {
		return instr
	}
}

//var Transformer transform.Transformer = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
var transformer transform.Transformer = transform.Chain(runes.Map(mapDecomposeUnavailable), norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

//no chinese or cyrilic supported
func StringToSlug(instr string) string {
	instr = strings.Replace(instr, "\u00df", "ss", -1) // ß to ss handling
	if strings.Contains(instr, "&") || strings.Contains(instr, "%") {
		instr = strings.ToLower(html.UnescapeString(instr))
	} else {
		instr = strings.ToLower(instr)
	}
	if strings.Contains(instr, "\\u") {
		instr2, err := strconv.Unquote("\"" + instr + "\"")
		if err != nil {
			instr = instr2
		}
	}
	instr = strings.Replace(instr, "ä", "ae", -1)
	instr = strings.Replace(instr, "ö", "oe", -1)
	instr = strings.Replace(instr, "ü", "ue", -1)
	instr = strings.Replace(instr, "ß", "ss", -1)
	instr = strings.Replace(instr, "&", "and", -1)
	instr = strings.Replace(instr, "@", "at", -1)
	instr = strings.Replace(instr, "½", ",5", -1)
	instr = strings.Replace(instr, "'", "", -1)
	instr = StringReplaceArray(instr, []string{" ", "§", "$", "%", "/", "(", ")", "=", "!", "?", "`", "\\", "}", "]", "[", "{", "|", ",", ".", ";", ":", "_", "+", "#", "<", ">", "*"}, "-")
	instr = strings.Replace(instr, "--", "-", -1)
	instr = strings.Replace(instr, "--", "-", -1)
	instr = strings.Replace(instr, "--", "-", -1)
	instr = strings.Replace(instr+"-", "-i-", "-1-", -1)
	instr = strings.Replace(instr, "-ii-", "-2-", -1)
	instr = strings.Replace(instr, "-iii-", "-3-", -1)
	instr = strings.Replace(instr, "-iv-", "-4-", -1)
	instr = strings.Replace(instr, "-v-", "-5-", -1)
	instr = strings.Replace(instr, "-vi-", "-6-", -1)
	instr = strings.Replace(instr, "-vii-", "-7-", -1)
	instr = strings.Replace(instr, "-viii-", "-8-", -1)
	instr = strings.Replace(instr, "-ix-", "-9-", -1)
	instr = strings.Replace(instr, "-x-", "-10-", -1)
	instr = strings.Replace(instr, "-xi-", "-11-", -1)
	instr = strings.Replace(instr, "-xii-", "-12-", -1)
	instr = strings.Replace(instr, "-xiii-", "-13-", -1)
	instr = strings.Replace(instr, "-xiv-", "-14-", -1)
	instr = strings.Replace(instr, "-xv-", "-15-", -1)
	instr = strings.Replace(instr, "-xvi-", "-16-", -1)
	instr = strings.Replace(instr, "-xvii-", "-17-", -1)
	instr = strings.Replace(instr, "-xviii-", "-18-", -1)
	instr = strings.Replace(instr, "-xix-", "-19-", -1)
	instr = strings.Replace(instr, "-xx-", "-20-", -1)

	defer func() { // recovers panic
		if e := recover(); e != nil {
			fmt.Println("Recovered from panic")
		}
	}()

	//t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, err := transform.String(transformer, instr)
	if err != nil {
		result = instr
	} else {
		result = strings.Trim(result, "-")
	}
	return result
}
