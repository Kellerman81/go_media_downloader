module github.com/Kellerman81/go_media_downloader

go 1.17

require (
	github.com/DeanThompson/ginpprof v0.0.0-20201112072838-007b1e56b2e1 //extend webserver with pprof tools
	github.com/RussellLuo/slidingwindow v0.0.0-20200528002341-535bb99d338b //Limiter
	github.com/andrewstuart/go-nzb v0.0.0-20151130213409-4af25f1cccf1 //access nzbget
	github.com/foolin/goview v0.3.0 //provide website with gin with templates
	github.com/gdm85/go-libdeluge v0.5.5 //access deluge
	github.com/gin-gonic/gin v1.7.4 //webapi
	github.com/goccy/go-reflect v1.1.0 //alternate reflect
	github.com/golang-migrate/migrate/v4 v4.15.1 //initialize db
	github.com/gregdel/pushover v1.1.0 //notification
	github.com/h2non/filetype v1.1.1
	github.com/jmoiron/sqlx v1.3.4 //structscan for db
	github.com/karrick/godirwalk v1.16.1 //faster walk
	github.com/knadh/koanf v1.3.2 //initial config
	github.com/mattn/go-sqlite3 v1.14.9 //data and imdb db
	github.com/mrobinsn/go-rtorrent v1.6.0
	github.com/mrobinsn/go-sabnzbd v0.0.0-20170707144533-63837cbec46d
	github.com/pkg/errors v0.9.1
	github.com/recoilme/pudge v1.0.3 //config db
	github.com/remeh/sizedwaitgroup v1.0.0 //concurrent wait group
	github.com/robfig/cron/v3 v3.0.1
	github.com/sirupsen/logrus v1.8.1 //logger
	github.com/toorop/gin-logrus v0.0.0-20210225092905-2c785434f26f //log gin stuff to logfile also
	golang.org/x/text v0.3.7
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 //Loop Logs
)

require (
	github.com/swaggo/files v0.0.0-20210815190702-a29dd2bc99b2
	github.com/swaggo/gin-swagger v1.3.3
	github.com/swaggo/swag v1.7.4
)

require (
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/gdm85/go-rencode v0.1.8 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.19.6 // indirect
	github.com/go-openapi/spec v0.20.4 // indirect
	github.com/go-openapi/swag v0.19.15 // indirect
	github.com/go-playground/locales v0.14.0 // indirect
	github.com/go-playground/universal-translator v0.18.0 // indirect
	github.com/go-playground/validator/v10 v10.9.0 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/go-sql-driver/mysql v1.6.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/lib/pq v1.10.4 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/ugorji/go/codec v1.2.6 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/crypto v0.0.0-20211108221036-ceb1ce70b4fa // indirect
	golang.org/x/net v0.0.0-20211118161319-6a13c67c3ce4 // indirect
	golang.org/x/sys v0.0.0-20211117180635-dee7805ff2e1 // indirect
	golang.org/x/tools v0.1.7 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
