module github.com/Kellerman81/go_media_downloader/pkg/main

go 1.25.0

require (
	github.com/DeanThompson/ginpprof v0.0.0-20201112072838-007b1e56b2e1 //extend webserver with pprof tools
	github.com/PuerkitoBio/goquery v1.12.0 //html parsing and manipulation
	github.com/alitto/pond/v2 v2.7.1 //worker pool
	github.com/antchfx/htmlquery v1.3.6 //xpath query for html
	github.com/bogem/id3v2/v2 v2.1.4 //id3v2 tag parser for mp3 files
	github.com/dgraph-io/ristretto/v2 v2.4.0 //memory cache
	github.com/fsnotify/fsnotify v1.9.0 //config watcher
	github.com/gin-gonic/gin v1.12.0 //web framework
	github.com/goccy/go-json v0.10.6 //json parser
	github.com/golang-migrate/migrate/v4 v4.19.1 //initialize db
	github.com/google/uuid v1.6.0 //uuid generation
	github.com/jmoiron/sqlx v1.4.0 //structscan for db
	github.com/mewkiz/flac v1.0.13 //flac audio parser
	github.com/mozillazg/go-unidecode v0.2.0 //unidecode tables
	github.com/pelletier/go-toml/v2 v2.3.0 //toml config parser
	github.com/robfig/cron/v3 v3.0.1 //scheduler
	github.com/rs/zerolog v1.35.1 //logging
	github.com/stretchr/testify v1.11.1 //testing toolkit
	golang.org/x/net v0.53.0 //network utilities
	golang.org/x/oauth2 v0.36.0 //used for trakt api
	golang.org/x/text v0.36.0 //text processing utilities
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 //log file rotation
	maragu.dev/gomponents v1.3.0 //html component library
	maragu.dev/gomponents-htmx v0.6.1 //htmx integration for gomponents
	modernc.org/sqlite v1.49.1 //sqlite db driver
)

require (
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/antchfx/xpath v1.3.6 // indirect
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic v1.15.0 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.12 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.1 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/icza/bitio v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.59.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.1 // indirect
	go.mongodb.org/mongo-driver/v2 v2.5.0 // indirect
	golang.org/x/arch v0.22.0 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	modernc.org/libc v1.72.0 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)
