module github.com/Kellerman81/go_media_downloader/pkg/main

go 1.24.5

require (
	github.com/DeanThompson/ginpprof v0.0.0-20201112072838-007b1e56b2e1 //extend webserver with pprof tools
	github.com/GoAdminGroup/go-admin v1.2.26
	github.com/GoAdminGroup/themes v0.0.48
	github.com/PuerkitoBio/goquery v1.10.3
	github.com/alitto/pond/v2 v2.5.0 //worker pool
	github.com/andrewstuart/go-nzb v0.0.0-20151130213409-4af25f1cccf1 //access nzbget
	github.com/fsnotify/fsnotify v1.9.0 //config watcher
	github.com/gdm85/go-libdeluge v0.6.0 //access deluge
	github.com/gin-gonic/gin v1.10.1 //webapi
	github.com/goccy/go-json v0.10.5 //json parser
	github.com/golang-migrate/migrate/v4 v4.18.3 //initialize db
	github.com/google/uuid v1.6.0 //scheduler
	github.com/jmoiron/sqlx v1.4.0 //structscan for db
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // needed for database migrate
	github.com/mozillazg/go-unidecode v0.2.0 //unidecode tables
	github.com/mrobinsn/go-rtorrent v1.8.0
	github.com/odwrtw/transmission v0.0.0-20221028215408-b11d7d55c759
	github.com/pelletier/go-toml/v2 v2.2.4 //toml config parser
	github.com/pkg/errors v0.9.1 //used in external apis
	github.com/recoilme/pudge v1.0.3 //config db
	github.com/robfig/cron/v3 v3.0.1 //scheduler
	github.com/rs/zerolog v1.34.0 //logging
	golang.org/x/oauth2 v0.30.0 //used for trakt api
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 //Loop Logs
	maragu.dev/gomponents v1.1.0
	maragu.dev/gomponents-htmx v0.6.1
	modernc.org/sqlite v1.38.2 //sqlite db driver
)

require (
	github.com/360EntSecGroup-Skylar/excelize v1.4.1 // indirect
	github.com/GoAdminGroup/html v0.0.1 // indirect
	github.com/NebulousLabs/fastrand v0.0.0-20181203155948-6fb6489aac4e // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/bytedance/sonic v1.11.8 // indirect
	github.com/bytedance/sonic/loader v0.1.1 // indirect
	github.com/cloudwego/base64x v0.1.4 // indirect
	github.com/cloudwego/iasm v0.2.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.4 // indirect
	github.com/gdm85/go-rencode v0.1.8 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.22.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.8 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/syndtr/goleveldb v1.0.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.19.1 // indirect
	golang.org/x/arch v0.8.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/exp v0.0.0-20250620022241-b7579e27df2b // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	modernc.org/libc v1.66.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	xorm.io/builder v0.3.7 // indirect
	xorm.io/xorm v1.0.2 // indirect
)
