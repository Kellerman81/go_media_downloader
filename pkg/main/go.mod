module github.com/Kellerman81/go_media_downloader

go 1.20

require (
	github.com/DeanThompson/ginpprof v0.0.0-20201112072838-007b1e56b2e1 //extend webserver with pprof tools
	github.com/GoAdminGroup/go-admin v1.2.24
	github.com/GoAdminGroup/themes v0.0.43
	github.com/alitto/pond v1.8.3 //Worker Pool
	github.com/andrewstuart/go-nzb v0.0.0-20151130213409-4af25f1cccf1 //access nzbget
	github.com/gdm85/go-libdeluge v0.6.0 //access deluge
	github.com/gin-contrib/logger v0.2.5
	github.com/gin-gonic/gin v1.9.1 //webapi
	github.com/golang-migrate/migrate/v4 v4.16.2 //initialize db
	github.com/google/uuid v1.3.0 //scheduler
	github.com/jmoiron/sqlx v1.3.5 //structscan for db
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // data and imdb db
	github.com/maxence-charriere/go-app/v9 v9.6.7
	github.com/mozillazg/go-unidecode v0.2.0
	github.com/mrobinsn/go-rtorrent v1.8.0
	github.com/odwrtw/transmission v0.0.0-20221028215408-b11d7d55c759
	github.com/pelletier/go-toml/v2 v2.0.8
	github.com/pkg/errors v0.9.1 //used in external apis
	github.com/recoilme/pudge v1.0.3 //config db
	github.com/robfig/cron/v3 v3.0.1 //scheduler
	github.com/rs/zerolog v1.29.1
	golang.org/x/net v0.10.0 //newznab uses that
	golang.org/x/oauth2 v0.8.0 //used for trakt api
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 //Loop Logs
)

require (
	github.com/360EntSecGroup-Skylar/excelize v1.4.1 // indirect
	github.com/GoAdminGroup/html v0.0.1 // indirect
	github.com/NebulousLabs/fastrand v0.0.0-20181203155948-6fb6489aac4e // indirect
	github.com/bytedance/sonic v1.9.1 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/gdm85/go-rencode v0.1.8 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.14.1 // indirect
	github.com/gobuffalo/logger v1.0.7 // indirect
	github.com/gobuffalo/packd v1.0.2 // indirect
	github.com/gobuffalo/packr/v2 v2.8.3 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/karrick/godirwalk v1.17.0 // indirect; faster walk
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/markbates/errx v1.1.0 // indirect
	github.com/markbates/oncer v1.0.0 // indirect
	github.com/markbates/safe v1.0.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/natefinch/lumberjack v2.0.0+incompatible // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect; logger
	github.com/syndtr/goleveldb v1.0.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/crypto v0.9.0 // indirect
	golang.org/x/sync v0.2.0 // indirect
	golang.org/x/sys v0.9.0 // indirect
	golang.org/x/term v0.8.0 // indirect
	golang.org/x/text v0.10.0 // indirect; used for sluggify
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	xorm.io/builder v0.3.12 // indirect
	xorm.io/xorm v1.3.2 // indirect
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/urfave/cli v1.22.14 // indirect
)
