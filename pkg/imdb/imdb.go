package main

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"database/sql"
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pelletier/go-toml/v2"
	"github.com/rainycape/unidecode"
)

var version string
var buildstamp string
var githash string

func csvgetint(instr string) int {
	getint, err := strconv.Atoi(instr)
	if err != nil {
		return 0
	}
	return getint
}
func csvgetfloat(instr string) float32 {
	flo, err := strconv.ParseFloat(instr, 32)
	if err != nil {
		return 0
	}
	return float32(flo)
}

func csvgetbool(instr string) bool {
	bo, err := strconv.ParseBool(instr)
	if err != nil {
		return false
	}
	return bo
}

const configfile string = "./config/config.toml"

type MainConfig struct {
	Imdbindexer imdbConfig `koanf:"imdbindexer" toml:"imdbindexer"`
}
type imdbConfig struct {
	Indexedtypes     []string `toml:"indexed_types"`
	Indexedlanguages []string `toml:"indexed_languages"`
	Indexfull        bool     `toml:"index_full"`
}

func LoadCfgDataDB() imdbConfig {
	content, err := os.ReadFile(configfile)
	if err != nil {
		fmt.Println("Error loading config. ", err)
	}
	var outim MainConfig
	errimdb := toml.Unmarshal(content, &outim)

	if errimdb == nil {
		return outim.Imdbindexer
	}
	return imdbConfig{}
}

var dbimdb *sql.DB

func initImdbdb(dbloglevel string, dbfile string) *sql.DB {
	if _, err := os.Stat("./databases/" + dbfile + ".db"); os.IsNotExist(err) {
		_, err := os.Create("./databases/" + dbfile + ".db") // Create SQLite file
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	db, err := sql.Open("sqlite3", "file:./databases/"+dbfile+".db?_fk=1&_mutex=no&_cslike=0")
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxIdleConns(15)
	db.SetMaxOpenConns(5)
	return db
}

func upgradeimdb() {
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
}
func loadakas(lang []string, full bool) {
	allowemptylang := false
	akamap := make(map[string]struct{}, len(lang))
	for idx := range lang {
		if lang[idx] == "" {
			allowemptylang = true
		} else {
			akamap[lang[idx]] = struct{}{}
		}
	}
	lang = nil

	fmt.Println("Opening akas..")
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
		DisableCompression: false,
		DisableKeepAlives:  true,
		IdleConnTimeout:    20 * time.Second}}

	req, _ := http.NewRequest("GET", "https://datasets.imdbws.com/title.akas.tsv.gz", nil)
	// Get the data

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	gzreader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return
	}
	defer gzreader.Close()

	parseraka := csv.NewReader(gzreader)
	parseraka.Comma = '\t'
	parseraka.LazyQuotes = true
	_, _ = parseraka.Read() // skip header

	tx, err := dbimdb.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
		tx = nil
		readWriteMu := &sync.Mutex{}

		readWriteMu.Lock()
		dbimdb.Exec("Update imdb_akas SET region = '' WHERE region = ?", "\\N")
		readWriteMu.Unlock()
		readWriteMu.Lock()
		dbimdb.Exec("Update imdb_akas SET language = '' WHERE language = ?", "\\N")
		readWriteMu.Unlock()
		readWriteMu.Lock()
		dbimdb.Exec("Update imdb_akas SET types = '' WHERE types = ?", "\\N")
		readWriteMu.Unlock()
		readWriteMu.Lock()
		dbimdb.Exec("Update imdb_akas SET attributes = '' WHERE attributes = ?", "\\N")
		readWriteMu.Unlock()
		readWriteMu = nil
	}()

	sqlstmtbyteshort := []byte("insert into imdb_akas (tconst, title, slug, region) VALUES ")
	sqlstmtbytelong := []byte("insert into imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES ")

	sqlparam4byte := []byte("(?, ?, ?, ?)")
	sqlparam9byte := []byte("(?, ?, ?, ?, ?, ?, ?, ?, ?)")

	sqlcommabyte := []byte(",")

	var sqlbuild strings.Builder //bytes.Buffer
	valueArgs := make([]interface{}, 0, 999)
	// addakasqlshort, err := tx.Prepare("insert into imdb_akas (tconst, title, slug, region) VALUES (?, ?, ?, ?)")
	// addakasql, err := tx.Prepare("insert into imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)")
	var ok bool
	for {
		record, err := parseraka.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(fmt.Errorf("an error occurred while parsing aka.. %v", err))
			continue
		}
		if imdbcache.Check(uint32(csvgetint(strings.TrimPrefix(record[0], "t")))) {
			if _, ok = akamap[record[3]]; ok || (len(record[3]) == 0 && allowemptylang) {
				// titlecount, _ := database.ImdbCountRows("imdb_titles", database.Query{Where: "tconst = ?", WhereArgs: []interface{}{record[0]}})

				if len(valueArgs) == 0 {
					if full {
						sqlbuild.Write(sqlstmtbytelong)
					} else {
						sqlbuild.Write(sqlstmtbyteshort)
					}
				} else {
					sqlbuild.Write(sqlcommabyte)
				}
				if full {
					sqlbuild.Write(sqlparam9byte)
					valueArgs = append(valueArgs, record[0])
					valueArgs = append(valueArgs, csvgetint(record[1]))
					valueArgs = append(valueArgs, UnescapeString(record[2]))
					valueArgs = append(valueArgs, StringToSlug(record[2]))
					valueArgs = append(valueArgs, record[3])
					valueArgs = append(valueArgs, record[4])
					valueArgs = append(valueArgs, record[5])
					valueArgs = append(valueArgs, record[6])
					valueArgs = append(valueArgs, csvgetbool(record[7]))
				} else {
					sqlbuild.Write(sqlparam4byte)
					valueArgs = append(valueArgs, record[0])
					valueArgs = append(valueArgs, UnescapeString(record[2]))
					valueArgs = append(valueArgs, StringToSlug(record[2]))
					valueArgs = append(valueArgs, record[3])
				}
				if len(valueArgs) > 900 {
					_, err = tx.Exec(sqlbuild.String(), valueArgs...)
					if err != nil {
						return
					}
					sqlbuild.Reset()
					valueArgs = nil
				}
				// if !full {
				// 	addakasqlshort.Exec(record[0], UnescapeString(record[2]), StringToSlug(record[2]), record[3])
				// } else {
				// 	addakasql.Exec(record[0], csvgetint(record[1]), UnescapeString(record[2]), StringToSlug(record[2]), record[3], record[4], record[5], record[6], csvgetbool(record[7]))
				// }
			}
		}
		record = nil
	}

	if len(valueArgs) > 1 {
		_, err = tx.Exec(sqlbuild.String(), valueArgs...)
		if err != nil {
			return
		}
		sqlbuild.Reset()
		valueArgs = nil
	}
	// addakasqlshort.Close()
	// addakasql.Close()
	gzreader = nil
	parseraka = nil
	req = nil
	client = nil
	akamap = nil
}
func loadratings() {
	fmt.Println("Opening ratings..")

	client := &http.Client{Timeout: 3600 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
			DisableCompression: false,
			DisableKeepAlives:  true,
			IdleConnTimeout:    20 * time.Second}}

	req, _ := http.NewRequest("GET", "https://datasets.imdbws.com/title.ratings.tsv.gz", nil)
	// Get the data

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	gzreader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return
	}
	defer gzreader.Close()

	parserrating := csv.NewReader(gzreader)
	parserrating.Comma = '\t'
	parserrating.LazyQuotes = true
	_, _ = parserrating.Read() // skip header

	tx, err := dbimdb.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()
	// addratingssql, err := tx.Prepare("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (?, ?, ?)")

	sqlstmtbyteshort := []byte("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES ")

	sqlparam4byte := []byte("(?, ?, ?)")

	sqlcommabyte := []byte(",")

	var sqlbuild strings.Builder // bytes.Buffer
	valueArgs := make([]interface{}, 0, 999)
	for {
		record, errcsv := parserrating.Read()
		if errcsv == io.EOF {
			break
		}
		if errcsv != nil {
			fmt.Println(fmt.Errorf("an error occurred while parsing rating.. %v", errcsv))
			continue
		}
		if imdbcache.Check(uint32(csvgetint(strings.TrimPrefix(record[0], "t")))) {
			if len(valueArgs) == 0 {
				sqlbuild.Write(sqlstmtbyteshort)
			} else {
				sqlbuild.Write(sqlcommabyte)
			}
			sqlbuild.Write(sqlparam4byte)
			valueArgs = append(valueArgs, record[0])
			valueArgs = append(valueArgs, csvgetint(record[2]))
			valueArgs = append(valueArgs, csvgetfloat(record[1]))
			if len(valueArgs) > 900 {
				_, err = tx.Exec(sqlbuild.String(), valueArgs...)
				if err != nil {
					return
				}
				sqlbuild.Reset()
				valueArgs = nil
			}
			// addratingssql.Exec(record[0], csvgetint(record[2]), csvgetfloat(record[1]))
		}
		record = nil
	}
	if len(valueArgs) > 1 {
		_, err = tx.Exec(sqlbuild.String(), valueArgs...)
		if err != nil {
			return
		}
		sqlbuild.Reset()
		valueArgs = nil
	}
	// addratingssql.Close()
	gzreader = nil
	parserrating = nil
	req = nil
	client = nil
}
func loadtitles(types []string, full bool) {

	titlemap := make(map[string]struct{}, len(types))
	for idx := range types {
		titlemap[types[idx]] = struct{}{}
	}
	types = nil

	// cacherowlimit := 1999
	fmt.Println("Opening titles..")

	client := &http.Client{Timeout: 3600 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
			DisableCompression: false,
			DisableKeepAlives:  true,
			IdleConnTimeout:    20 * time.Second}}

	req, _ := http.NewRequest("GET", "https://datasets.imdbws.com/title.basics.tsv.gz", nil)
	// Get the data

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	gzreader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return
	}
	defer gzreader.Close()

	parsertitle := csv.NewReader(gzreader)
	parsertitle.Comma = '\t'
	parsertitle.LazyQuotes = true
	_, _ = parsertitle.Read() //skip header

	tx, err := dbimdb.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()

		readWriteMu := &sync.Mutex{}
		readWriteMu.Lock()
		dbimdb.Exec("Update imdb_titles SET genres = '' WHERE genres = ?", "\\N")
		readWriteMu.Unlock()
		readWriteMu.Lock()
		dbimdb.Exec("Update imdb_genres SET genre = '' WHERE genre = ?", "\\N")
		readWriteMu.Unlock()
		readWriteMu = nil
	}()
	// addtitlesqlshort, err := tx.Prepare("insert into imdb_titles (tconst, title_type, primary_title, slug, start_year, runtime_minutes) VALUES (?, ?, ?, ?, ?, ?)")
	// addtitlesql, err := tx.Prepare("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	// addgenresql, err := tx.Prepare("insert into imdb_genres (tconst, genre) VALUES (?, ?)")

	sqlstmtbyteshort := []byte("insert into imdb_titles (tconst, title_type, primary_title, slug, start_year, runtime_minutes) VALUES ")
	sqlstmtbytelong := []byte("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES ")
	sqlstmtbytegenre := []byte("insert into imdb_genres (tconst, genre) VALUES ")

	sqlparam2byte := []byte("(?, ?)")
	sqlparam6byte := []byte("(?, ?, ?, ?, ?, ?)")
	sqlparam10byte := []byte("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

	sqlcommabyte := []byte(",")

	var sqlbuild strings.Builder //bytes.Buffer
	valueArgs := make([]interface{}, 0, 999)
	var sqlbuildgenre strings.Builder //bytes.Buffer
	valueArgsGenre := make([]interface{}, 0, 999)
	var ok bool
	for {
		record, err := parsertitle.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(fmt.Errorf("an error occurred while parsing title.. %v", err))
			continue
		}
		if _, ok = titlemap[record[1]]; ok && record[1] != "" {
			imdbcache.Set(uint32(csvgetint(strings.TrimPrefix(record[0], "t"))))

			if len(valueArgs) == 0 {
				if full {
					sqlbuild.Write(sqlstmtbytelong)
				} else {
					sqlbuild.Write(sqlstmtbyteshort)
				}
			} else {
				sqlbuild.Write(sqlcommabyte)
			}
			if full {
				sqlbuild.Write(sqlparam10byte)
				valueArgs = append(valueArgs, record[0])
				valueArgs = append(valueArgs, record[1])
				valueArgs = append(valueArgs, UnescapeString(record[2]))
				valueArgs = append(valueArgs, StringToSlug(record[2]))
				valueArgs = append(valueArgs, UnescapeString(record[3]))
				valueArgs = append(valueArgs, csvgetbool(record[4]))
				valueArgs = append(valueArgs, csvgetint(record[5]))
				valueArgs = append(valueArgs, csvgetint(record[7]))
				valueArgs = append(valueArgs, csvgetint(record[6]))
				valueArgs = append(valueArgs, record[8])
				if strings.Contains(record[8], ",") {
					for _, genre := range strings.Split(record[8], ",") {
						if genre != "" {
							if len(valueArgsGenre) == 0 {
								sqlbuildgenre.Write(sqlstmtbytegenre)
							} else {
								sqlbuildgenre.Write(sqlcommabyte)
							}
							sqlbuildgenre.Write(sqlparam2byte)
							valueArgsGenre = append(valueArgsGenre, record[0])
							valueArgsGenre = append(valueArgsGenre, genre)
							if len(valueArgsGenre) > 900 {
								_, err = tx.Exec(sqlbuildgenre.String(), valueArgsGenre...)
								if err != nil {
									fmt.Println(err, sqlbuildgenre.String())
									return
								}
								sqlbuildgenre.Reset()
								valueArgsGenre = nil
							}
							// addgenresql.Exec(record[0], genre)
						}
					}
				} else if len(record[8]) >= 1 {
					if len(valueArgsGenre) == 0 {
						sqlbuildgenre.Write(sqlstmtbytegenre)
					} else {
						sqlbuildgenre.Write(sqlcommabyte)
					}
					sqlbuildgenre.Write(sqlparam2byte)
					valueArgsGenre = append(valueArgsGenre, record[0])
					valueArgsGenre = append(valueArgsGenre, record[8])
					if len(valueArgsGenre) > 900 {
						_, err = tx.Exec(sqlbuildgenre.String(), valueArgsGenre...)
						if err != nil {
							fmt.Println(err, sqlbuildgenre.String())
							return
						}
						sqlbuildgenre.Reset()
						valueArgsGenre = nil
					}
					// addgenresql.Exec(record[0], record[8])
				}
			} else {
				sqlbuild.Write(sqlparam6byte)
				valueArgs = append(valueArgs, record[0])
				valueArgs = append(valueArgs, record[1])
				valueArgs = append(valueArgs, UnescapeString(record[2]))
				valueArgs = append(valueArgs, StringToSlug(record[2]))
				valueArgs = append(valueArgs, csvgetint(record[5]))
				valueArgs = append(valueArgs, csvgetint(record[7]))
			}
			if len(valueArgs) > 900 {
				_, err = tx.Exec(sqlbuild.String(), valueArgs...)
				if err != nil {
					fmt.Println(err, sqlbuild.String())
					return
				}
				sqlbuild.Reset()
				valueArgs = nil
			}
			// if !full {
			// 	//addtitlesqlshort.Exec(record[0], record[1], UnescapeString(record[2]), StringToSlug(record[2]), csvgetint(record[5]), csvgetint(record[7]))
			// } else {
			// 	//addtitlesql.Exec(record[0], record[1], UnescapeString(record[2]), StringToSlug(record[2]), UnescapeString(record[3]), csvgetbool(record[4]), csvgetint(record[5]), csvgetint(record[7]), csvgetint(record[6]), record[8])
			// 	if strings.Contains(record[8], ",") {
			// 		for _, genre := range strings.Split(record[8], ",") {
			// 			addgenresql.Exec(record[0], genre)
			// 		}
			// 	} else if len(record[8]) >= 1 {
			// 		addgenresql.Exec(record[0], record[8])
			// 	}
			// }
		}
		record = nil
	}
	if len(valueArgs) > 1 {
		_, err = tx.Exec(sqlbuild.String(), valueArgs...)
		if err != nil {
			return
		}
		sqlbuild.Reset()
		valueArgs = nil
	}
	if len(valueArgsGenre) > 1 {
		_, err = tx.Exec(sqlbuildgenre.String(), valueArgsGenre...)
		if err != nil {
			return
		}
		sqlbuildgenre.Reset()
		valueArgsGenre = nil
	}
	// addgenresql.Close()
	// addtitlesql.Close()
	// addtitlesqlshort.Close()
	gzreader = nil
	parsertitle = nil
	req = nil
	client = nil
	titlemap = nil
}

func main() {
	fmt.Println("Imdb Importer by kellerman81 - version " + version + " " + githash + " from " + buildstamp)
	var cfg_imdb imdbConfig = LoadCfgDataDB()
	fmt.Println("Started Imdb Import")
	os.Remove("./databases/imdbtemp.db")
	dbimdb = initImdbdb("info", "imdbtemp")

	upgradeimdb()

	imdbcache = NewCache(1200000)

	loadtitles(cfg_imdb.Indexedtypes, cfg_imdb.Indexfull)
	loadakas(cfg_imdb.Indexedlanguages, cfg_imdb.Indexfull)
	loadratings()

	imdbcache.Items = nil

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

func UnescapeString(instr string) string {
	if strings.Contains(instr, "&") || strings.Contains(instr, "%") {
		return html.UnescapeString(instr)
	} else {
		return instr
	}
}

var subRune = map[rune]string{
	'&':  "and",
	'@':  "at",
	'"':  "",
	'\'': "",
	'’':  "",
	'_':  "",
	'‒':  "-", // figure dash
	'–':  "-", // en dash
	'—':  "-", // em dash
	'―':  "-", // horizontal bar
	'ä':  "ae",
	'Ä':  "Ae",
	'ö':  "oe",
	'Ö':  "Oe",
	'ü':  "ue",
	'Ü':  "Ue",
	'ß':  "ss",
}

func makeSlug(s string) string {
	s = replaceUnwantedChars(strings.ToLower(unidecode.Unidecode(substituteRune(strings.TrimSpace(s)))))
	if strings.Contains(s, "--") {
		s = strings.Replace(s, "--", "-", -1)
		if strings.Contains(s, "--") {
			s = strings.Replace(s, "--", "-", -1)
			if strings.Contains(s, "--") {
				s = strings.Replace(s, "--", "-", -1)
			}
		}
	}
	return strings.TrimSuffix(s, "-")
}

// SubstituteRune substitutes string chars with provided rune
// substitution map. One pass.
func substituteRune(s string) string {
	var buf bytes.Buffer
	buf.Grow(len(s))
	for _, c := range s {
		if d, ok := subRune[c]; ok {
			buf.WriteString(d)
		} else {
			buf.WriteRune(c)
		}
	}
	return buf.String()
}

var wantedChars = map[rune]bool{
	'a': true,
	'b': true,
	'c': true,
	'd': true,
	'e': true,
	'f': true,
	'g': true,
	'h': true,
	'i': true,
	'j': true,
	'k': true,
	'l': true,
	'm': true,
	'n': true,
	'o': true,
	'p': true,
	'q': true,
	'r': true,
	's': true,
	't': true,
	'u': true,
	'v': true,
	'w': true,
	'x': true,
	'y': true,
	'z': true,
	'A': true,
	'B': true,
	'C': true,
	'D': true,
	'E': true,
	'F': true,
	'G': true,
	'H': true,
	'I': true,
	'J': true,
	'K': true,
	'L': true,
	'M': true,
	'N': true,
	'O': true,
	'P': true,
	'Q': true,
	'R': true,
	'S': true,
	'T': true,
	'U': true,
	'V': true,
	'W': true,
	'X': true,
	'Y': true,
	'Z': true,
	'0': true,
	'1': true,
	'2': true,
	'3': true,
	'4': true,
	'5': true,
	'6': true,
	'7': true,
	'8': true,
	'9': true,
	'-': true,
}

func replaceUnwantedChars(s string) string {
	var buf bytes.Buffer
	buf.Grow(len(s))
	for _, c := range s {
		if _, ok := wantedChars[c]; ok {
			buf.WriteString(string(c))
		} else {
			buf.WriteRune('-')
		}
	}
	return buf.String()
}

// no chinese or cyrilic supported
func StringToSlug(instr string) string {
	instr = strings.Replace(instr, "\u00df", "ss", -1) // ß to ss handling
	instr = UnescapeString(instr)
	if strings.Contains(instr, "\\u") {
		instr, _ = strconv.Unquote("\"" + instr + "\"")
	}
	return makeSlug(instr)
}

func ClearVar(i interface{}) {
	v := reflect.ValueOf(i)
	if !v.IsZero() && v.Kind() == reflect.Pointer {
		v.Elem().Set(reflect.Zero(v.Elem().Type()))
	}
}

var imdbcache *Cache

type Cache struct {
	Items map[uint32]struct{}
	// mu    *sync.Mutex
}

func NewCache(maxsize int) *Cache {
	cache := &Cache{
		Items: make(map[uint32]struct{}, maxsize),
		// mu:    &sync.Mutex{},
	}

	return cache
}
func (cache *Cache) Check(key uint32) bool {
	// cache.mu.Lock()
	// defer cache.mu.Unlock()
	_, exists := cache.Items[key]
	return exists
}
func (cache *Cache) Set(key uint32) {
	// cache.mu.Lock()
	// defer cache.mu.Unlock()
	cache.Items[key] = struct{}{}
}
