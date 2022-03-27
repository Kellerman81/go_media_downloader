module github.com/Kellerman81/go_media_downloader

go 1.18

require (
	github.com/DeanThompson/ginpprof v0.0.0-20201112072838-007b1e56b2e1 //extend webserver with pprof tools
	github.com/GoAdminGroup/go-admin v1.2.23
	github.com/GoAdminGroup/themes v0.0.43
	github.com/andrewstuart/go-nzb v0.0.0-20151130213409-4af25f1cccf1 //access nzbget
	github.com/gdm85/go-libdeluge v0.5.6 //access deluge
	github.com/gin-gonic/gin v1.7.7 //webapi
	github.com/golang-migrate/migrate/v4 v4.15.1 //initialize db
	github.com/google/uuid v1.3.0 //scheduler
	github.com/gregdel/pushover v1.1.0 //notification
	github.com/jmoiron/sqlx v1.3.4 //structscan for db
	github.com/karrick/godirwalk v1.16.1 //faster walk
	github.com/knadh/koanf v1.4.0 //initial config
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // indirect; data and imdb db
	github.com/mrobinsn/go-rtorrent v1.8.0
	github.com/odwrtw/transmission v0.0.0-20211003115842-e3a58d4370e9
	github.com/pkg/errors v0.9.1
	github.com/recoilme/pudge v1.0.3 //config db
	github.com/robfig/cron/v3 v3.0.1 //scheduler
	github.com/sirupsen/logrus v1.8.1 //logger
	github.com/toorop/gin-logrus v0.0.0-20210225092905-2c785434f26f //log gin stuff to logfile also
	golang.org/x/net v0.0.0-20220325170049-de3da57026de
	golang.org/x/oauth2 v0.0.0-20220309155454-6242fa91716a
	golang.org/x/text v0.3.7
	golang.org/x/time v0.0.0-20220224211638-0e9765cccd65
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 //Loop Logs
)

require github.com/swaggo/swag v1.8.0

require (
	github.com/360EntSecGroup-Skylar/excelize v1.4.1 // indirect
	github.com/GoAdminGroup/html v0.0.1 // indirect
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/NebulousLabs/fastrand v0.0.0-20181203155948-6fb6489aac4e // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.1 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/gdm85/go-rencode v0.1.8 // indirect
	github.com/getkin/kin-openapi v0.93.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.19.6 // indirect
	github.com/go-openapi/spec v0.20.4 // indirect
	github.com/go-openapi/swag v0.21.1 // indirect
	github.com/go-playground/locales v0.14.0 // indirect
	github.com/go-playground/universal-translator v0.18.0 // indirect
	github.com/go-playground/validator/v10 v10.10.1 // indirect
	github.com/gobuffalo/logger v1.0.6 // indirect
	github.com/gobuffalo/packd v1.0.1 // indirect
	github.com/gobuffalo/packr/v2 v2.8.3 // indirect
	github.com/goccy/go-json v0.9.6 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/joho/godotenv v1.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/lib/pq v1.10.4 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/markbates/errx v1.1.0 // indirect
	github.com/markbates/oncer v1.0.0 // indirect
	github.com/markbates/safe v1.0.1 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/natefinch/lumberjack v2.0.0+incompatible // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/swaggo/gin-swagger v1.4.1 // indirect
	github.com/syndtr/goleveldb v1.0.0 // indirect
	github.com/ugorji/go/codec v1.2.7 // indirect
	github.com/urfave/cli/v2 v2.4.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.uber.org/zap v1.21.0 // indirect
	golang.org/x/crypto v0.0.0-20220321153916-2c7772ba3064 // indirect
	golang.org/x/sys v0.0.0-20220319134239-a9b59b0215f8 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/tools v0.1.10 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/ini.v1 v1.66.4 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	xorm.io/builder v0.3.9 // indirect
	xorm.io/xorm v1.2.5 // indirect
)
