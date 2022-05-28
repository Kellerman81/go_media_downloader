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
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rainycape/unidecode"
)

var version string
var buildstamp string
var githash string

func csvsetdefault(instr string, def string) string {
	if instr == `\N` {
		return def
	}
	return instr
}

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
	akamap := NewStringSetMaxSize(len(lang))
	for _, row := range lang {
		akamap.Add(row)
	}
	defer akamap.Clear()

	readWriteMu := &sync.Mutex{}

	fmt.Println("Opening akas..")
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
		DisableCompression: false,
		DisableKeepAlives:  true,
		IdleConnTimeout:    20 * time.Second}}

	req, _ := http.NewRequest("GET", "https://datasets.imdbws.com/title.akas.tsv.gz", nil)
	// Get the data

	resp, errh := client.Do(req)
	if errh != nil {
		return
	}
	defer resp.Body.Close()

	gzreader, errg := gzip.NewReader(resp.Body)
	if errg != nil {
		return
	}
	defer gzreader.Close()

	parseraka := csv.NewReader(gzreader)
	defer ClearVar(&parseraka)
	parseraka.Comma = '\t'
	parseraka.LazyQuotes = true
	_, _ = parseraka.Read() //skip header

	tx, err := dbimdb.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
		tx = nil
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
	}()
	addakasqlshort, err := tx.Prepare("insert into imdb_akas (tconst, title, slug, region) VALUES (?, ?, ?, ?)")
	addakasql, err := tx.Prepare("insert into imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)")

	for {
		record, errcsv := parseraka.Read()
		if errcsv == io.EOF {
			break
		}
		if errcsv != nil {
			fmt.Println(fmt.Errorf("an error occurred while parsing aka.. %v", errcsv))
			continue
		}

		if akamap.Contains(record[3]) || len(record[3]) == 0 {
			//titlecount, _ := database.ImdbCountRows("imdb_titles", database.Query{Where: "tconst = ?", WhereArgs: []interface{}{record[0]}})
			if cachetconst.Contains(csvgetint(strings.TrimPrefix(record[0], "t"))) {

				if !full {
					addakasqlshort.Exec(record[0], UnescapeString(record[2]), StringToSlug(record[2]), record[3])
				} else {
					addakasql.Exec(record[0], csvgetint(record[1]), UnescapeString(record[2]), StringToSlug(record[2]), record[3], record[4], record[5], record[6], csvgetbool(record[7]))
				}
			}
		}
		record = nil
	}
	addakasqlshort.Close()
	addakasql.Close()
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
	defer ClearVar(&parserrating)
	parserrating.Comma = '\t'
	parserrating.LazyQuotes = true
	_, _ = parserrating.Read() //skip header

	tx, err := dbimdb.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()
	addratingssql, err := tx.Prepare("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (?, ?, ?)")

	for {
		record, errcsv := parserrating.Read()
		if errcsv == io.EOF {
			break
		}
		if errcsv != nil {
			fmt.Println(fmt.Errorf("an error occurred while parsing rating.. %v", errcsv))
			continue
		}
		if cachetconst.Contains(csvgetint(strings.TrimPrefix(record[0], "t"))) {

			addratingssql.Exec(record[0], csvgetint(record[2]), csvgetfloat(record[1]))
		}
		record = nil
	}
	addratingssql.Close()
}
func loadtitles(types []string, full bool) {
	titlemap := NewStringSetMaxSize(len(types))
	for _, row := range types {
		titlemap.Add(row)
	}
	defer titlemap.Clear()

	// cacherowlimit := 1999
	readWriteMu := &sync.Mutex{}
	fmt.Println("Opening titles..")

	client := &http.Client{Timeout: 3600 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
			DisableCompression: false,
			DisableKeepAlives:  true,
			IdleConnTimeout:    20 * time.Second}}

	req, _ := http.NewRequest("GET", "https://datasets.imdbws.com/title.basics.tsv.gz", nil)
	// Get the data

	resp, errh := client.Do(req)
	if errh != nil {
		return
	}
	defer resp.Body.Close()

	gzreader, errg := gzip.NewReader(resp.Body)
	if errg != nil {
		return
	}
	defer gzreader.Close()

	parsertitle := csv.NewReader(gzreader)
	defer ClearVar(&parsertitle)
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

		readWriteMu.Lock()
		dbimdb.Exec("Update imdb_titles SET genres = '' WHERE genres = ?", "\\N")
		readWriteMu.Unlock()
		readWriteMu.Lock()
		dbimdb.Exec("Update imdb_genres SET genre = '' WHERE genre = ?", "\\N")
		readWriteMu.Unlock()
	}()
	addtitlesqlshort, err := tx.Prepare("insert into imdb_titles (tconst, title_type, primary_title, slug, start_year, runtime_minutes) VALUES (?, ?, ?, ?, ?, ?)")
	addtitlesql, err := tx.Prepare("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	addgenresql, err := tx.Prepare("insert into imdb_genres (tconst, genre) VALUES (?, ?)")

	for {
		record, errcsv := parsertitle.Read()
		if errcsv == io.EOF {
			break
		}
		if errcsv != nil {
			fmt.Println(fmt.Errorf("an error occurred while parsing title.. %v", errcsv))
			continue
		}
		if titlemap.Contains(record[1]) && record[1] != "" {

			cachetconst.Add(csvgetint(strings.TrimPrefix(record[0], "t")))
			if !full {
				addtitlesqlshort.Exec(record[0], record[1], UnescapeString(record[2]), StringToSlug(record[2]), csvgetint(record[5]), csvgetint(record[7]))
			} else {
				addtitlesql.Exec(record[0], record[1], UnescapeString(record[2]), StringToSlug(record[2]), UnescapeString(record[3]), csvgetbool(record[4]), csvgetint(record[5]), csvgetint(record[7]), csvgetint(record[6]), record[8])
				if strings.Contains(record[8], ",") {
					for _, genre := range strings.Split(record[8], ",") {
						addgenresql.Exec(record[0], genre)
					}
				} else if len(record[8]) >= 1 {
					addgenresql.Exec(record[0], record[8])
				}
			}
		}
		record = nil
	}
	addgenresql.Close()
	addtitlesql.Close()
	addtitlesqlshort.Close()
}

var cachetconst IntSet

func main() {
	fmt.Println("Imdb Importer by kellerman81 - version " + version + " " + githash + " from " + buildstamp)
	var cfg_imdb imdbConfig = LoadCfgDataDB()
	fmt.Println("Started Imdb Import")
	os.Remove("./databases/imdbtemp.db")
	dbimdb = initImdbdb("info", "imdbtemp")

	upgradeimdb()

	cachetconst = NewIntSetMaxSize(1200000)

	loadtitles(cfg_imdb.Indexedtypes, cfg_imdb.Indexfull)
	loadakas(cfg_imdb.Indexedlanguages, cfg_imdb.Indexfull)
	loadratings()

	cachetconst.Clear()

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
	s = strings.TrimSpace(s)
	s = substituteRune(s)
	s = unidecode.Unidecode(s)
	s = strings.ToLower(s)
	s = replaceUnwantedChars(s)
	s = strings.Replace(s, "--", "-", -1)
	s = strings.Replace(s, "--", "-", -1)
	s = strings.Replace(s, "--", "-", -1)
	return s
}

// SubstituteRune substitutes string chars with provided rune
// substitution map. One pass.
func substituteRune(s string) string {
	var buf bytes.Buffer
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
	for _, c := range s {
		if _, ok := wantedChars[c]; ok {
			buf.WriteString(string(c))
		} else {
			buf.WriteRune('-')
		}
	}
	return buf.String()
}

//no chinese or cyrilic supported
func StringToSlug(instr string) string {
	instr = strings.Replace(instr, "\u00df", "ss", -1) // ß to ss handling
	instr = UnescapeString(instr)
	if strings.Contains(instr, "\\u") {
		instr, _ = strconv.Unquote("\"" + instr + "\"")
	}
	return makeSlug(instr)
}

type StringSet struct {
	Values []string
}

func NewStringSet() StringSet {
	return StringSet{}
}

func NewStringSetMaxSize(size int) StringSet {
	return StringSet{Values: make([]string, 0, size)}
}
func NewStringSetExactSize(size int) StringSet {
	return StringSet{Values: make([]string, size)}
}

func (s *StringSet) Add(str string) {
	s.Values = append(s.Values, str)
}

func (s *StringSet) Length() int {
	return len(s.Values)
}

func (s *StringSet) Remove(str string) {
	new := s.Values[:0]
	for _, val := range s.Values {
		if val != str {
			new = append(new, val)
		}
	}
	s.Values = new
	new = nil
}

func (s *StringSet) Contains(str string) bool {
	for _, val := range s.Values {
		if val == str {
			return true
		}
	}
	return false
}

func (s *StringSet) Clear() {
	s.Values = nil
	s = nil
}

func (s *StringSet) Difference(dif StringSet) {
	new := s.Values[:0]
	for _, val := range s.Values {
		insub := false
		for _, difval := range dif.Values {
			if val == difval {
				insub = true
				break
			}
		}
		if !insub {
			new = append(new, val)
		}
	}
	s.Values = new
	new = nil
}

func (s *StringSet) Union(add StringSet) {
	new := s.Values
	for _, val := range add.Values {
		if !s.Contains(val) {
			new = append(new, val)
		}
	}
	s.Values = new
	new = nil
}

func ClearVar(i interface{}) {
	v := reflect.ValueOf(i)
	if !v.IsZero() && v.Kind() == reflect.Pointer {
		v.Elem().Set(reflect.Zero(v.Elem().Type()))
	}
}

type IntSet struct {
	Values []int
}

func NewIntSet() IntSet {
	return IntSet{}
}

func NewIntSetMaxSize(size int) IntSet {
	return IntSet{Values: make([]int, 0, size)}
}
func NewIntSetExactSize(size int) IntSet {
	return IntSet{Values: make([]int, size)}
}

func (s *IntSet) Add(val int) {
	s.Values = append(s.Values, val)
}

func (s *IntSet) Length() int {
	return len(s.Values)
}

func (s *IntSet) Remove(valchk int) {
	new := s.Values[:0]
	for _, val := range s.Values {
		if val != valchk {
			new = append(new, val)
		}
	}
	s.Values = new
	new = nil
}

func (s *IntSet) Contains(valchk int) bool {
	for _, val := range s.Values {
		if val == valchk {
			return true
		}
	}
	return false
}

func (s *IntSet) Clear() {
	s.Values = nil
	s = nil
}
