package main

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	_ "net/http/pprof"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mozillazg/go-unidecode/table"
	"github.com/pelletier/go-toml/v2"

	"github.com/h2non/filetype"
)

type mainConfig struct {
	Imdbindexer imdbConfig `koanf:"imdbindexer" toml:"imdbindexer"`
}
type imdbConfig struct {
	Indexedtypes     []string `toml:"indexed_types"`
	Indexedlanguages []string `toml:"indexed_languages"`
	Indexfull        bool     `toml:"index_full"`
	ImdbIDSize       int      `toml:"imdbid_size"`
	LoopSize         int      `toml:"loop_size"`
	UseMemory        bool     `toml:"use_memory"`
	UseCache         bool     `toml:"use_cache"`
}

const configfile = "./config/config.toml"

var (
	loopsize                int
	i                       int
	tx                      *sql.Tx
	indexfull               bool
	usecache                = false
	usememory               = true
	imdbcache               map[uint32]struct{}
	sqlcache                = make(map[string]*sql.Stmt, 1000)
	titlemap                map[string]struct{}
	akamap                  map[string]struct{}
	allowemptylang          bool
	sqlbuild                bytes.Buffer
	valueArgs               = make([]interface{}, 0, 999)
	sqlbuildgenre           bytes.Buffer
	valueArgsGenre          = make([]interface{}, 0, 999)
	version                 string
	buildstamp              string
	githash                 string
	dbimdb                  *sql.DB
	nilstuct                = struct{}{}
	sqlparam2byte           = "(?, ?)"
	sqlparam3byte           = "(?, ?, ?)"
	sqlparam4byte           = "(?, ?, ?, ?)"
	sqlparam6byte           = "(?, ?, ?, ?, ?, ?)"
	sqlparam9byte           = "(?, ?, ?, ?, ?, ?, ?, ?, ?)"
	sqlparam10byte          = "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	sqlcommabyte            = ","
	sqlstmtbyteshorttitles  = "insert into imdb_titles (tconst, title_type, primary_title, slug, start_year, runtime_minutes) VALUES "
	sqlstmtbytelongtitles   = "insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES "
	sqlstmtbytegenre        = "insert into imdb_genres (tconst, genre) VALUES "
	sqlstmtbyteshortakas    = "insert into imdb_akas (tconst, title, slug, region) VALUES "
	sqlstmtbytelongakas     = "insert into imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES "
	sqlstmtbyteshortratings = "insert into imdb_ratings (tconst, num_votes, average_rating) VALUES "
	sqlstmtshorttitles      *sql.Stmt
	sqlstmtlongtitles       *sql.Stmt
	sqlstmtgenre            *sql.Stmt
	sqlstmtshortakas        *sql.Stmt
	sqlstmtlongakas         *sql.Stmt
	sqlstmtshortratings     *sql.Stmt
	PlBuffer                = NewPool(100, 0, func(b *bytes.Buffer) {}, func(b *bytes.Buffer) { b.Reset() })
	substituteRuneSpace     = map[rune]string{
		'&':  "and",
		'@':  "at",
		'"':  "",
		'\'': "",
		'’':  "",
		'_':  "",
		' ':  "-",
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
	subRune = map[rune]bool{
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
)

// csvgetintarr converts the string value in record to an int.
// Returns 0 if the value is empty, "0", "\\N", or cannot be parsed as an int.
func csvgetintarr(record string) int {
	if record == "" || record == "0" || record == "\\N" {
		return 0
	}
	getint, err := strconv.Atoi(record)
	if err != nil {
		return 0
	}
	return int(getint)
}

// csvgetuint32arr converts the string value from the provided CSV
// record to a uint32. It returns 0 if the value is
// empty, "0", or "\\N".
func csvgetuint32arr(record string) uint32 {
	if record == "" || record == "0" || record == "\\N" {
		return 0
	}
	getint, err := strconv.ParseUint(strings.TrimLeft(record, "t"), 10, 0)
	//getint, err := strconv.Atoi(strings.TrimLeft(instr, "t"))
	if err != nil {
		return 0
	}
	return uint32(getint)
}

// csvgetfloatarr converts the string value in record to a float32.
// Returns 0 if the value is empty, "0", "0.0", "\\N", or cannot be parsed as a float32.
func csvgetfloatarr(record string) float32 {
	if record == "" || record == "0" || record == "0.0" || record == "\\N" {
		return 0
	}
	flo, err := strconv.ParseFloat(record, 32)
	if err != nil {
		return 0
	}
	return float32(flo)
}

// csvgetboolarr converts the string value in record to a bool.
// Returns false if the value is "\\N", otherwise returns true if the
// value is "1", "t", "T", "true", "TRUE", or "True", and false otherwise.
func csvgetboolarr(record string) bool {
	if record == "\\N" {
		return false
	}
	switch record {
	case "1", "t", "T", "true", "TRUE", "True":
		return true
	}
	return false
}

// loadCfgDataDB loads the configuration from the config file and unmarshals it into a struct.
// It returns the Imdbindexer config struct on success, or an empty struct on error.
func loadCfgDataDB() imdbConfig {
	content, err := os.ReadFile(configfile)
	if err != nil {
		fmt.Println("Error loading config. ", err)
	}
	var outim mainConfig
	errimdb := toml.Unmarshal(content, &outim)

	if errimdb == nil {
		return outim.Imdbindexer
	}
	return imdbConfig{}
}

// initImdbdb initializes a SQLite database connection for the
// given database file. It creates the database file if it doesn't exist.
// It configures the database connection with some performance tuning
// settings like enabling shared cache and in-memory journaling. It also
// optionally keeps the entire database in memory if the usememory flag
// is set. The database handle is returned.
func initImdbdb(dbloglevel string, dbfile string) *sql.DB {
	if _, err := os.Stat("./databases/" + dbfile + ".db"); os.IsNotExist(err) {
		_, err := os.Create("./databases/" + dbfile + ".db") // Create SQLite file
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	str := "file:./databases/" + dbfile + ".db?_fk=1&_journal=memory&cache=shared"
	if usememory {
		str = "file:./databases/" + dbfile + ".db?_fk=1&_journal=memory&mode=memory&cache=shared"
	}
	db, err := sql.Open("sqlite3", str)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxIdleConns(15)
	db.SetMaxOpenConns(5)
	return db
}

// upgradeimdb upgrades the imdb database schema. It initializes a new
// migrate instance to run the upgrades defined in the imdbdb schema
// files, opens the temporary database, and runs the migration Up
// method to perform the schema changes.
func upgradeimdb() {
	m, err := migrate.New(
		"file://./schema/imdbdb",
		"sqlite3://./databases/imdbtemp.db?_fk=1&_journal=memory&mode=memory&_cslike=0",
	)
	if err != nil {
		fmt.Println(fmt.Errorf("migration failed... %v", err))
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		fmt.Println(fmt.Errorf("an error occurred while syncing the database.. %v", err))
	}
	m = nil
}

// loadakas loads akas (alternative titles) data from a TSV file into the database.
// It opens the TSV file, initializes a CSV reader, skips the header row,
// processes each row by calling processakas, and executes batched INSERT statements.
// It also clears maps used for caching after loading all data.
func loadakas() {
	fmt.Println("Opening akas..")

	filetitle, err := os.Open("./title.akas.tsv")
	if err != nil {
		fmt.Println(fmt.Errorf("an error occurred while opening akas.. %v", err))
		return
	}
	defer filetitle.Close()
	parseraka := csv.NewReader(filetitle)
	parseraka.Comma = '\t'
	parseraka.LazyQuotes = true
	parseraka.ReuseRecord = true
	parseraka.TrimLeadingSpace = true
	_, _ = parseraka.Read() //skip header

	for {
		if processakas(parseraka) == io.EOF {
			break
		}
	}

	if usecache && len(valueArgs) > 1 {
		err = exec(sqlbuild.String(), true, valueArgs)
		if err != nil {
			fmt.Println(fmt.Errorf("an error occurred while exec..%s %v", sqlbuild.String(), err))
			return
		}
	}
	clear(akamap)

	sqlbuild.Reset()
	valueArgs = valueArgs[:0]

	akamap = nil
}

// processakas parses akas (alternative titles) records from the input
// and inserts them into the database. It handles caching optimization
// to batch insert queries.
func processakas(parsertitle *csv.Reader) error {
	record, err := parsertitle.Read()
	if err == io.EOF {
		return err
	}
	if err != nil {
		fmt.Println(fmt.Errorf("an error occurred while parsing title.. %v", err))
		return nil
	}
	if record[1] == "" {
		return nil
	}
	_, exists := imdbcache[csvgetuint32arr(record[0])]
	if !exists {
		return nil
	}
	if !allowemptylang && len(record[3]) == 0 {
		return nil
	}
	if _, ok := akamap[record[3]]; !ok {
		if !allowemptylang {
			return nil
		}
		if record[3] != "" {
			return nil
		}
	}

	if usecache {
		if len(valueArgs) == 0 {
			if indexfull {
				sqlbuild.WriteString(sqlstmtbytelongakas)
			} else {
				sqlbuild.WriteString(sqlstmtbyteshortakas)
			}
		} else {
			sqlbuild.WriteString(sqlcommabyte)
		}
	}
	if indexfull {
		if record[3] == "\\N" {
			record[3] = ""
		}
		if record[4] == "\\N" {
			record[4] = ""
		}
		if record[5] == "\\N" {
			record[5] = ""
		}
		if record[6] == "\\N" {
			record[6] = ""
		}
		if usecache {
			sqlbuild.WriteString(sqlparam9byte)
			i++
			valueArgs = append(valueArgs, record[0], csvgetintarr(record[1]), unescapeString(record[2]), stringToSlug(record[2]), record[3], record[4], record[5], record[6], csvgetboolarr(record[7]))
		} else {
			_, sqlerr := sqlstmtlongakas.Exec(&record[0], csvgetintarr(record[1]), unescapeString(record[2]), stringToSlug(record[2]), &record[3], &record[4], &record[5], &record[6], csvgetboolarr(record[7]))
			if sqlerr != nil {
				fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
			}
		}
	} else {
		if record[3] == "\\N" {
			record[3] = ""
		}
		if usecache {
			sqlbuild.WriteString(sqlparam4byte)
			i++
			valueArgs = append(valueArgs, record[0], unescapeString(record[2]), stringToSlug(record[2]), record[3])
		} else {
			_, sqlerr := sqlstmtshortakas.Exec(&record[0], unescapeString(record[2]), stringToSlug(record[2]), &record[3])
			if sqlerr != nil {
				fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
			}
		}
	}
	if !usecache {
		return nil
	}
	if len(valueArgs) <= 900 {
		return nil
	}
	sqlerr := exec(sqlbuild.String(), false, valueArgs)
	if sqlerr != nil {
		fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
		return nil
	}

	sqlbuild.Reset()
	valueArgs = valueArgs[:0]
	return nil
}

// loadratings loads the ratings data from a TSV file and inserts it into the database.
// It opens the file, initializes a CSV parser for it, skips the header row,
// builds an INSERT statement to batch insert ratings data, executes the statement in batches,
// and closes the file and parser when finished.
func loadratings() {
	fmt.Println("Opening ratings..")

	filetitle, err := os.Open("./title.ratings.tsv")
	if err != nil {
		fmt.Println(fmt.Errorf("an error occurred while opening ratings.. %v", err))
		return
	}
	defer filetitle.Close()
	parserrating := csv.NewReader(filetitle)
	parserrating.Comma = '\t'
	parserrating.LazyQuotes = true
	parserrating.ReuseRecord = true
	parserrating.TrimLeadingSpace = true
	_, _ = parserrating.Read() //skip header

	for {
		if processratings(parserrating) == io.EOF {
			break
		}
	}
	if len(valueArgs) > 1 {
		sqlerr := exec(sqlbuild.String(), true, valueArgs)
		if sqlerr != nil {
			fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
			return
		}
	}

	sqlbuild.Reset()
	valueArgs = valueArgs[:0]
}

// processratings processes each record from the ratings CSV file.
// It checks if the title ID exists in the cache and inserts the rating data into
// the database, either executing the query directly or batching for performance.
func processratings(parsertitle *csv.Reader) error {
	record, err := parsertitle.Read()
	if err == io.EOF {
		return err
	}
	if err != nil {
		fmt.Println(fmt.Errorf("an error occurred while parsing title.. %v", err))
		return nil
	}
	if record[1] == "" {
		return nil
	}
	_, exists := imdbcache[csvgetuint32arr(record[0])]
	if !exists {
		return nil
	}
	if !usecache {
		_, sqlerr := sqlstmtshortratings.Exec(&record[0], csvgetintarr(record[2]), csvgetfloatarr(record[1]))
		if sqlerr != nil {
			fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
		}
		return nil
	}
	if len(valueArgs) == 0 {
		sqlbuild.WriteString(sqlstmtbyteshortratings)
	} else {
		sqlbuild.WriteString(sqlcommabyte)
	}
	sqlbuild.WriteString(sqlparam3byte)
	i++
	valueArgs = append(valueArgs, record[0], csvgetintarr(record[2]), csvgetfloatarr(record[1]))
	if len(valueArgs) <= 900 {
		return nil
	}
	sqlerr := exec(sqlbuild.String(), false, valueArgs)
	if sqlerr != nil {
		fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
		return nil
	}
	sqlbuild.Reset()
	valueArgs = valueArgs[:0]
	return nil
}

// loadtitles loads title data from a TSV file into database tables.
// It parses the TSV rows into structs, handles caching and batch inserts for performance.
func loadtitles() {
	fmt.Println("Opening titles..")

	filetitle, err := os.Open("./title.basics.tsv")
	if err != nil {
		fmt.Println(fmt.Errorf("an error occurred while opening titles.. %v", err))
		return
	}
	defer filetitle.Close()
	parsertitle := csv.NewReader(filetitle)
	parsertitle.Comma = '\t'
	parsertitle.LazyQuotes = true
	parsertitle.ReuseRecord = true
	parsertitle.TrimLeadingSpace = true
	_, _ = parsertitle.Read() //skip header

	for {
		if processtitles(parsertitle) == io.EOF {
			break
		}
	}

	if usecache {
		if len(valueArgs) > 1 {
			err = exec(sqlbuild.String(), true, valueArgs)
			if err != nil {
				fmt.Println(fmt.Errorf("an error occurred while exec..%s %v", sqlbuild.String(), err))
				return
			}
		}
		if len(valueArgsGenre) > 1 {
			err = exec(sqlbuildgenre.String(), true, valueArgsGenre)
			if err != nil {
				fmt.Println(fmt.Errorf("an error occurred while exec..%s %v", sqlbuild.String(), err))
				return
			}
		}
	}
	clear(titlemap)
	titlemap = nil

	sqlbuild.Reset()
	sqlbuildgenre.Reset()
	valueArgs = valueArgs[:0]
	valueArgsGenre = valueArgsGenre[:0]
}

// processtitles processes CSV records from a titles file.
// It inserts/updates records into database tables like imdb_titles and imdb_genres.
// It handles caching for performance by batching INSERTs.
func processtitles(parsertitle *csv.Reader) error {
	record, err := parsertitle.Read()
	if err == io.EOF {
		return err
	}
	if err != nil {
		fmt.Println(fmt.Errorf("an error occurred while parsing title.. %v", err))
		return nil
	}
	if record[1] == "" {
		return nil
	}
	if _, ok := titlemap[record[1]]; !ok {
		return nil
	}
	imdbcache[csvgetuint32arr(record[0])] = nilstuct

	if usecache {
		if len(valueArgs) == 0 {
			if indexfull {
				sqlbuild.WriteString(sqlstmtbytelongtitles)
			} else {
				sqlbuild.WriteString(sqlstmtbyteshorttitles)
			}
		} else {
			sqlbuild.WriteString(sqlcommabyte)
		}
	}
	if indexfull {
		if record[8] == "\\N" {
			record[8] = ""
		}
		if usecache {
			sqlbuild.WriteString(sqlparam10byte)
			i++
			valueArgs = append(valueArgs, record[0], record[1], unescapeString(record[2]), stringToSlug(record[2]), unescapeString(record[3]), csvgetboolarr(record[4]), csvgetintarr(record[5]), csvgetintarr(record[7]), csvgetintarr(record[6]), record[8])
		} else {
			_, sqlerr := sqlstmtlongtitles.Exec(&record[0], &record[1], unescapeString(record[2]), stringToSlug(record[2]), unescapeString(record[3]), csvgetboolarr(record[4]), csvgetintarr(record[5]), csvgetintarr(record[7]), csvgetintarr(record[6]), &record[8])
			if sqlerr != nil {
				fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
			}
		}
		//valueArgs = append(valueArgs, record[0], record[1], record[2], stringToSlug(&record[2]), record[3], csvgetbool(record[4]), csvgetint(record[5]), csvgetint(record[7]), csvgetint(record[6]), record[8])
		if strings.ContainsRune(record[8], ',') {
			genres := strings.Split(record[8], ",")
			var sqlerr error
			for idx := range genres {
				if genres[idx] != "" && genres[idx] != "\\N" {
					if usecache {
						if len(valueArgsGenre) == 0 {
							sqlbuildgenre.WriteString(sqlstmtbytegenre)
						} else {
							sqlbuildgenre.WriteString(sqlcommabyte)
						}
						sqlbuildgenre.WriteString(sqlparam2byte)
						i++
						valueArgsGenre = append(valueArgsGenre, record[0], genres[idx])
						if len(valueArgsGenre) > 900 {
							sqlerr = exec(sqlbuildgenre.String(), false, valueArgsGenre)
							if sqlerr != nil {
								fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
								return nil
							}
							sqlbuildgenre.Reset()
							valueArgsGenre = valueArgsGenre[:0]
						}
					} else {
						_, sqlerr = sqlstmtgenre.Exec(&record[0], &genres[idx])
						if sqlerr != nil {
							fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
						}
					}
				}
			}
		} else if len(record[8]) >= 1 {
			if usecache {
				if len(valueArgsGenre) == 0 {
					sqlbuildgenre.WriteString(sqlstmtbytegenre)
				} else {
					sqlbuildgenre.WriteString(sqlcommabyte)
				}
				sqlbuildgenre.WriteString(sqlparam2byte)
				i++
				valueArgsGenre = append(valueArgsGenre, record[0], record[8])
				if len(valueArgsGenre) > 900 {
					sqlerr := exec(sqlbuildgenre.String(), false, valueArgsGenre)
					if sqlerr != nil {
						fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
						return nil
					}
					sqlbuildgenre.Reset()
					valueArgsGenre = valueArgsGenre[:0]
				}
			} else {
				_, sqlerr := sqlstmtgenre.Exec(&record[0], &record[8])
				if sqlerr != nil {
					fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
				}
			}
		}
	} else {
		if usecache {
			sqlbuild.WriteString(sqlparam6byte)
			i++
			valueArgs = append(valueArgs, record[0], record[1], unescapeString(record[2]), stringToSlug(record[2]), csvgetintarr(record[5]), csvgetintarr(record[7]))
		} else {
			_, sqlerr := sqlstmtshorttitles.Exec(&record[0], &record[1], unescapeString(record[2]), stringToSlug(record[2]), csvgetintarr(record[5]), csvgetintarr(record[7]))
			if sqlerr != nil {
				fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
			}
		}
	}
	if !usecache {
		return nil
	}
	if len(valueArgs) >= 900 {
		sqlerr := exec(sqlbuild.String(), false, valueArgs)
		if sqlerr != nil {
			fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
			return nil
		}
		sqlbuild.Reset()
		valueArgs = valueArgs[:0]
	}
	if len(valueArgsGenre) >= 900 {
		sqlerr := exec(sqlbuildgenre.String(), false, valueArgsGenre)
		if sqlerr != nil {
			fmt.Println(fmt.Errorf("an error occurred while processing sql.. %v", sqlerr))
			return nil
		}
		sqlbuildgenre.Reset()
		valueArgsGenre = valueArgsGenre[:0]
	}
	return nil
}

// exec executes the provided SQL query using the prepared statement
// cached in sqlcache. It starts a new transaction if startnew is true.
// It handles commit/rollback on errors and clearing the cache when
// a new transaction starts.
func exec(query string, last bool, args []interface{}) error {
	stmt, exists := sqlcache[query]
	if !exists {
		sqlcache[query], _ = dbimdb.Prepare(query)
		stmt, exists = sqlcache[query]
	}
	var err error
	if exists && stmt == nil {
		stmt, err = tx.Prepare(query)
		if err != nil {
			fmt.Println(fmt.Errorf("an error occurred while prepping..%s %v", query, err))
			return err
		}
		sqlcache[query] = stmt
	}
	_, err = stmt.Exec(args...)
	if err != nil {
		tx.Rollback()
		tx, _ = dbimdb.Begin()
		fmt.Println(fmt.Errorf("an error occurred while exec..%s %v", query, err))
		return err
	}
	if !last {
		if i < loopsize {
			return nil
		}
	}
	fmt.Println("committing rows: " + strconv.Itoa(i))
	i = 0
	err = tx.Commit()
	if err != nil {
		fmt.Println(fmt.Errorf("an error occurred while commit..%s %v", query, err))
		return err
	}
	tx, _ = dbimdb.Begin()

	return nil
}

func main() {
	// go func() {
	// 	_ = http.ListenAndServe(":8848", nil)
	// }()
	fmt.Println("Imdb Importer by kellerman81 - version " + version + " " + githash + " from " + buildstamp)
	cfgimdb := loadCfgDataDB()
	if cfgimdb.ImdbIDSize == 0 {
		cfgimdb.ImdbIDSize = 1200000
	}
	if cfgimdb.LoopSize == 0 {
		cfgimdb.LoopSize = 400000
	}
	loopsize = cfgimdb.LoopSize
	indexfull = cfgimdb.Indexfull
	usecache = cfgimdb.UseCache
	usememory = cfgimdb.UseMemory
	imdbcache = make(map[uint32]struct{}, cfgimdb.ImdbIDSize)
	fmt.Println("Started Imdb Import")
	os.Remove("./databases/imdbtemp.db")
	dbimdb = initImdbdb("info", "imdbtemp")

	if usememory {
		dbimdb.Exec(`CREATE TABLE [schema_migrations] (
			[version] uint64, 
			[dirty] bool
		);`)
		dbimdb.Exec(`CREATE UNIQUE INDEX [version_unique]
			ON [schema_migrations] ([version]);`)
		dbimdb.Exec(`Insert into [schema_migrations] (version, dirty) VALUES (2, 0)`)

		dbimdb.Exec(`CREATE TABLE "imdb_titles" (
		"tconst"	text NOT NULL,
		"title_type"	text DEFAULT "",
		"primary_title"	text DEFAULT "",
		"slug"	text DEFAULT "",
		"original_title"	text DEFAULT "",
		"is_adult"	numeric,
		"start_year"	integer,
		"end_year"	integer,
		"runtime_minutes"	integer,
		"genres"	text DEFAULT ""
	);`)
		dbimdb.Exec(`CREATE TABLE "imdb_ratings" (
		"id"	integer,
		"created_at"	datetime NOT NULL DEFAULT current_timestamp,
		"updated_at"	datetime NOT NULL DEFAULT current_timestamp,
		"tconst"	text DEFAULT "",
		"num_votes"	integer,
		"average_rating"	real,
		PRIMARY KEY("id")
	);`)
		dbimdb.Exec(`CREATE TABLE "imdb_genres" (
		"id"	integer,
		"created_at"	datetime NOT NULL DEFAULT current_timestamp,
		"updated_at"	datetime NOT NULL DEFAULT current_timestamp,
		"tconst"	text DEFAULT "",
		"genre"	text DEFAULT "",
		PRIMARY KEY("id")
	);`)
		dbimdb.Exec(`CREATE TABLE "imdb_akas" (
		"id"	integer,
		"created_at"	datetime NOT NULL DEFAULT current_timestamp,
		"updated_at"	datetime NOT NULL DEFAULT current_timestamp,
		"tconst"	text DEFAULT "",
		"ordering"	integer,
		"title"	text DEFAULT "",
		"slug"	text DEFAULT "",
		"region"	text DEFAULT "",
		"language"	text DEFAULT "",
		"types"	text DEFAULT "",
		"attributes"	text DEFAULT "",
		"is_original_title"	numeric,
		PRIMARY KEY("id")
	);`)
		dbimdb.Exec(`CREATE INDEX "idx_imdb_titles_slug" ON "imdb_titles" (
		"slug"
	);`)
		dbimdb.Exec(`CREATE INDEX "idx_imdb_titles_primary_title" ON "imdb_titles" (
		"primary_title"
	);`)
		dbimdb.Exec(`CREATE UNIQUE INDEX "idx_imdb_titles_tconst" ON "imdb_titles" (
		"tconst"
	);`)
		dbimdb.Exec(`CREATE INDEX "idx_imdb_akas_slug" ON "imdb_akas" (
		"slug"
	);`)
		dbimdb.Exec(`CREATE INDEX "idx_imdb_akas_title" ON "imdb_akas" (
		"title"
	);`)
		dbimdb.Exec(`CREATE TRIGGER tg_imdb_akas_updated_at
	AFTER UPDATE
	ON imdb_akas FOR EACH ROW
	BEGIN
	  UPDATE imdb_akas SET updated_at = current_timestamp
		where id = old.id;
	END;`)
		dbimdb.Exec(`CREATE TRIGGER tg_imdb_ratings_updated_at
	AFTER UPDATE
	ON imdb_ratings FOR EACH ROW
	BEGIN
	  UPDATE imdb_ratings SET updated_at = current_timestamp
		where id = old.id;
	END;`)
		dbimdb.Exec(`CREATE TRIGGER tg_imdb_genres_updated_at
	AFTER UPDATE
	ON imdb_genres FOR EACH ROW
	BEGIN
	  UPDATE imdb_genres SET updated_at = current_timestamp
		where id = old.id;
	END;`)
		dbimdb.Exec(`CREATE INDEX "idx_imdb_akas_tconst" ON "imdb_akas" (
		"tconst"
	);`)
		dbimdb.Exec(`CREATE INDEX "idx_imdb_titles_start_year" ON "imdb_titles" (
		"start_year"
	);`)
		dbimdb.Exec(`CREATE INDEX "idx_imdb_titles_original_title" ON "imdb_titles" (
		"original_title"
	);`)
		dbimdb.Exec(`CREATE INDEX "idx_imdb_akas_title_slug" ON "imdb_akas" (
		"title",
		"slug"
	);`)
		dbimdb.Exec(`CREATE INDEX "idx_imdb_titles_primary_original_slug" ON "imdb_titles" (
		"primary_title",
		"original_title",
		"slug"
	);`)
	} else {
		upgradeimdb()
	}
	dbimdb.Exec("PRAGMA journal_mode=OFF")
	sqlbuild.Grow(100000)
	sqlbuildgenre.Grow(100000)

	tx, _ = dbimdb.Begin()

	var err error
	sqlstmtshorttitles, _ = dbimdb.Prepare("insert into imdb_titles (tconst, title_type, primary_title, slug, start_year, runtime_minutes) VALUES (?,?,?,?,?,?)")
	sqlstmtlongtitles, err = dbimdb.Prepare("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES (?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}
	sqlstmtgenre, _ = dbimdb.Prepare("insert into imdb_genres (tconst, genre) VALUES (?,?)")
	sqlstmtshortakas, _ = dbimdb.Prepare("insert into imdb_akas (tconst, title, slug, region) VALUES (?,?,?,?)")
	sqlstmtlongakas, _ = dbimdb.Prepare("insert into imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES (?,?,?,?,?,?,?,?,?)")
	sqlstmtshortratings, _ = dbimdb.Prepare("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (?,?,?)")

	allowemptylang = false
	titlemap = make(map[string]struct{}, len(cfgimdb.Indexedtypes))
	for idx := range cfgimdb.Indexedtypes {
		titlemap[cfgimdb.Indexedtypes[idx]] = nilstuct
	}

	downloadimdbfiles()

	loadtitles()
	clear(titlemap)

	akamap = make(map[string]struct{}, len(cfgimdb.Indexedlanguages))
	for idx := range cfgimdb.Indexedlanguages {
		if cfgimdb.Indexedlanguages[idx] == "" {
			allowemptylang = true
		} else {
			akamap[cfgimdb.Indexedlanguages[idx]] = nilstuct
		}
	}
	loadakas()
	clear(akamap)

	loadratings()

	if usememory {
		dbimdb.Exec("VACUUM INTO ?", "./databases/imdbtemp.db")
	}

	clear(imdbcache)
	imdbcache = nil
	var counter int
	if dbimdb.QueryRow("Select count(*) from imdb_titles").Scan(&counter) != nil {
		dbimdb.Close()
		os.Remove("./databases/imdbtemp.db")
		return
	}
	if counter == 0 {
		dbimdb.Close()
		os.Remove("./databases/imdbtemp.db")
		return
	}
	dbimdb.Close()

	fmt.Println("Ended Imdb Import")
}

// unescapeString unescapes HTML entities in the string of the given record. It returns an empty string if the
// record's value is "\\N", otherwise it unescapes &amp; entities if
// present before returning the string.
func unescapeString(record string) string {
	if record == "\\N" {
		return ""
	}
	if strings.ContainsRune(record, '&') {
		return html.UnescapeString(record)
	}
	return record
}

// replaceUnwantedChars replaces unwanted characters in s with '-'.
// It checks if s only contains allowed characters, and returns early if so.
// Otherwise it iterates through s and replaces disallowed characters with '-'.
func replaceUnwantedChars(s *string) {
	if s == nil || *s == "" {
		return
	}
	ok := true
	for idx := range *s {
		if _, ok = subRune[rune((*s)[idx])]; !ok {
			break
		}
	}
	if ok {
		return
	}
	var lastr byte
	out := []byte(*s)[:0]
	for idx := range *s {
		if _, ok = subRune[rune((*s)[idx])]; !ok {
			if idx > 0 && lastr == '-' {
				continue
			}
			out = append(out, '-')
			lastr = '-'
		} else {
			out = append(out, (*s)[idx])
			lastr = (*s)[idx]
		}
	}
	*s = string(out)
}

// stringToSlug converts a string of the record
// to a slug format by removing unwanted characters, collapsing multiple
// hyphens, and trimming leading/trailing hyphens. Returns empty string
// if input string is empty.
func stringToSlug(instr string) string {
	if instr == "" {
		return ""
	}
	inbyte := unidecode2(instr)
	if len(inbyte) == 0 {
		return ""
	}
	inbyte = bytes.TrimRight(inbyte, "- ")
	inbyte = bytes.TrimLeft(inbyte, "- ")
	return string(inbyte)
}

// unidecode2 converts a unicode string to an ASCII transliteration by
// replacing each unicode rune with its best ASCII approximation. It handles
// special cases like converting to lowercase and inserting separators between
// contiguous substitutions. This allows sanitizing unicode strings into
// a more filesystem-friendly ASCII format.
func unidecode2(s string) []byte {
	ret := PlBuffer.Get()
	var laststr string
	var lastrune rune
	//var c byte
	if strings.ContainsRune(s, '&') {
		s = html.UnescapeString(s)
	}
	ret.Grow(len(s) + 10)
	for _, r := range s {
		if val, ok := substituteRuneSpace[r]; ok {
			if laststr != "" && val == laststr {
				continue
			}
			if lastrune == '-' && val == "-" {
				continue
			}
			ret.WriteString(val)
			laststr = val
			if val == "-" {
				lastrune = '-'
			} else {
				lastrune = ' '
			}
			continue
		}
		if laststr != "" {
			laststr = ""
		}

		if r < unicode.MaxASCII {
			if 'A' <= r && r <= 'Z' {
				r += 'a' - 'A'
			}
			if _, ok := subRune[r]; !ok {
				if lastrune == '-' {
					continue
				}
				lastrune = '-'
				ret.WriteRune('-')
			} else {
				if lastrune == '-' && r == '-' {
					continue
				}
				lastrune = r
				ret.WriteRune(r)
			}
			continue
		}
		if r > 0xeffff {
			continue
		}

		section := r >> 8   // Chop off the last two hex digits
		position := r % 256 // Last two hex digits
		if tb, ok := table.Tables[section]; ok {
			if len(tb) > int(position) {
				if len(tb[position]) >= 1 {
					if tb[position][0] > unicode.MaxASCII && lastrune != '-' {
						lastrune = '-'
						ret.WriteRune('-')
						continue
					}
				}
				if lastrune == '-' && tb[position] == "-" {
					continue
				}
				ret.WriteString(tb[position])
			}
		}
	}
	defer PlBuffer.Put(ret)
	return ret.Bytes()
}

// downloadFile downloads the content from the given URL
// and saves it to a file in the given directory with the given filename.
// It returns any error encountered.
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
	out.Sync()
	return err
}

// RemoveFile removes the file at the given path.
// It returns an error if the file could not be removed.
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

// downloadimdbfiles downloads compressed IMDB dataset files,
// uncompresses them, and removes the original compressed files.
// It downloads and uncompresses 3 files:
// - title.basics.tsv.gz
// - title.akas.tsv.gz
// - title.ratings.tsv.gz
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

// gunzip decompresses a gzipped file to a target filename.
// It takes a gzipped source filename and a target filename to decompress to.
// It handles reading the gzipped data, decompressing it, and writing the
// decompressed data to the target file.
func gunzip(source string, target string) {
	data, _ := os.ReadFile(source)
	body := bytes.NewBuffer(data)
	reader, err := gzip.NewReader(body)
	if err != nil {
		fmt.Println("err1. ", err)
		return
	}
	defer reader.Close()

	bodyo, err := match(reader)
	if err != nil {
		fmt.Println("err2. ", err)
		return
	}

	err = copy(target, 0666, bodyo)
	if err != nil {
		fmt.Println("err3. ", err)
	}
}

// copy copies the contents of the source reader to the file at the provided path.
// It creates any necessary parent directories, truncates any existing file, sets the mode,
// copies the data, syncs, and closes the file. Any errors are printed and returned.
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
	file.Sync()
	return err
}

// match reads the first 512 bytes, calls types.Match and returns a reader
// for the whole stream
func match(r *gzip.Reader) (io.Reader, error) {
	buffer := make([]byte, 512)

	n, err := r.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, err
	}
	_, err = filetype.Match(buffer)

	return io.MultiReader(bytes.NewBuffer(buffer[:n]), r), err
}

type Poolobj[T any] struct {
	//objs is a channel of type T
	objs chan *T
	//Function will be run on Get() - include here your logic to create the initial object
	constructor func(*T)
	//Function will be run on Put() - include here your logic to reset the object
	destructor func(*T)
}

// NewPool creates a new Poolobj initialized with the given parameters.
//
// maxsize specifies the maximum number of objects that can be kept in the
// pool.
//
// initcreate specifies the initial number of objects to create in the pool
// on startup.
//
// constructor, if non-nil, is called whenever a new object needs to be
// created.
//
// destructor, if non-nil, is called whenever an object is removed from
// the pool.
func NewPool[T any](maxsize int, initcreate int, constructor func(*T), destructor func(*T)) Poolobj[T] {
	var a Poolobj[T]
	a.constructor = constructor
	a.objs = make(chan *T, maxsize)
	if initcreate > 0 {
		for i := 0; i < initcreate; i++ {
			var bo T
			if a.constructor != nil {
				a.constructor(&bo)
			}
			a.objs <- &bo
		}
	}
	a.destructor = destructor
	return a
}

// Get retrieves an object from the pool or creates a new one if none are
// available. If a constructor was provided, it will be called to initialize
// any newly created objects.
func (p *Poolobj[T]) Get() *T {
	if len(p.objs) >= 1 {
		return <-p.objs
	}
	var bo T
	if p.constructor != nil {
		p.constructor(&bo)
	}
	return &bo
}

// Put returns an object to the pool.
// If the pool is not at capacity, it calls the destructor function if provided,
// then sends the object back on the channel.
func (p *Poolobj[T]) Put(bo *T) {
	if bo == nil {
		return
	}
	if len(p.objs) < cap(p.objs) {
		if p.destructor != nil {
			p.destructor(bo)
		}
		p.objs <- bo
	}
}
