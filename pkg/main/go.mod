module github.com/Kellerman81/go_media_downloader/pkg/main

go 1.22

require (
	github.com/DeanThompson/ginpprof v0.0.0-20201112072838-007b1e56b2e1 //extend webserver with pprof tools
	github.com/GoAdminGroup/themes v0.0.43
	github.com/alitto/pond v1.8.3 //Worker Pool
	github.com/andrewstuart/go-nzb v0.0.0-20151130213409-4af25f1cccf1 //access nzbget
	github.com/gdm85/go-libdeluge v0.6.0 //access deluge
	github.com/gin-gonic/gin v1.9.1 //webapi
	github.com/goccy/go-json v0.10.2
	github.com/golang-migrate/migrate/v4 v4.17.0 //initialize db
	github.com/google/uuid v1.6.0 //scheduler
	github.com/gregdel/pushover v1.3.0
	github.com/jmoiron/sqlx v1.3.5 //structscan for db
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // data and imdb db
	github.com/maxence-charriere/go-app/v9 v9.8.0
	github.com/mozillazg/go-unidecode v0.2.0
	github.com/mrobinsn/go-rtorrent v1.8.0
	github.com/odwrtw/transmission v0.0.0-20221028215408-b11d7d55c759
	github.com/pelletier/go-toml/v2 v2.1.1
	github.com/pkg/errors v0.9.1 //used in external apis
	github.com/recoilme/pudge v1.0.3 //config db
	github.com/robfig/cron/v3 v3.0.1 //scheduler
	github.com/rs/zerolog v1.32.0
	golang.org/x/net v0.22.0 //newznab uses that
	golang.org/x/oauth2 v0.18.0 //used for trakt api
	golang.org/x/text v0.14.0 // used for sluggify
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 //Loop Logs
)

require github.com/GoAdminGroup/go-admin v1.2.24

require (
	github.com/360EntSecGroup-Skylar/excelize v1.4.1 // indirect
	github.com/GoAdminGroup/html v0.0.1 // indirect
	github.com/NebulousLabs/fastrand v0.0.0-20181203155948-6fb6489aac4e // indirect
	github.com/bytedance/sonic v1.9.1 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gdm85/go-rencode v0.1.8 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.14.0 // indirect
	github.com/gobuffalo/logger v1.0.6 // indirect
	github.com/gobuffalo/packd v1.0.1 // indirect
	github.com/gobuffalo/packr/v2 v2.8.3 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/klauspost/cpuid/v2 v2.2.4 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/markbates/errx v1.1.0 // indirect
	github.com/markbates/oncer v1.0.0 // indirect
	github.com/markbates/safe v1.0.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/natefinch/lumberjack v2.0.0+incompatible // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/sirupsen/logrus v1.9.2 // indirect
	github.com/smartystreets/goconvey v1.8.1 // indirect
	github.com/syndtr/goleveldb v1.0.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.17.0 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/term v0.18.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/ini.v1 v1.62.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	xorm.io/builder v0.3.7 // indirect
	xorm.io/xorm v1.0.2 // indirect
)
