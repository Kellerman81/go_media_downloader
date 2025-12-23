module github.com/Kellerman81/go_media_downloader/pkg/main

go 1.25

require (
	github.com/DeanThompson/ginpprof v0.0.0-20201112072838-007b1e56b2e1 //extend webserver with pprof tools
	github.com/PuerkitoBio/goquery v1.11.0
	github.com/alitto/pond/v2 v2.6.0 //worker pool
	github.com/antchfx/htmlquery v1.3.5
	github.com/dgraph-io/ristretto v0.2.0
	github.com/fsnotify/fsnotify v1.9.0 //config watcher
	github.com/gin-gonic/gin v1.11.0 //webapi
	github.com/goccy/go-json v0.10.5 //json parser
	github.com/golang-migrate/migrate/v4 v4.19.1 //initialize db
	github.com/google/uuid v1.6.0 //scheduler
	github.com/jmoiron/sqlx v1.4.0 //structscan for db
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // needed for database migrate
	github.com/mozillazg/go-unidecode v0.2.0 //unidecode tables
	github.com/pelletier/go-toml/v2 v2.2.4 //toml config parser
	github.com/robfig/cron/v3 v3.0.1 //scheduler
	github.com/rs/zerolog v1.34.0 //logging
	github.com/stretchr/testify v1.11.1
	golang.org/x/net v0.48.0
	golang.org/x/oauth2 v0.34.0 //used for trakt api
	golang.org/x/text v0.32.0
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 //Loop Logs
	maragu.dev/gomponents v1.2.0
	maragu.dev/gomponents-htmx v0.6.1
	modernc.org/sqlite v1.41.0 //sqlite db driver
)

require (
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/antchfx/xpath v1.3.5 // indirect
	github.com/bytedance/sonic v1.14.0 // indirect
	github.com/bytedance/sonic/loader v0.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.27.0 // indirect
	github.com/goccy/go-yaml v1.18.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/pkg/errors v0.9.1 // indirect; used in external apis
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/quic-go/qpack v0.5.1 // indirect
	github.com/quic-go/quic-go v0.54.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.0 // indirect
	go.uber.org/mock v0.5.0 // indirect
	golang.org/x/arch v0.20.0 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/exp v0.0.0-20250620022241-b7579e27df2b // indirect
	golang.org/x/mod v0.30.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/tools v0.39.0 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	modernc.org/libc v1.66.10 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)
