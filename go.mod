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
	github.com/golang-migrate/migrate/v4 v4.15.0 //initialize db
	github.com/gregdel/pushover v1.1.0 //notification
	github.com/h2non/filetype v1.1.1
	github.com/jmoiron/sqlx v1.3.4 //structscan for db
	github.com/karrick/godirwalk v1.16.1 //faster walk
	github.com/knadh/koanf v1.2.4 //initial config
	github.com/mattn/go-sqlite3 v1.14.8 //data and imdb db
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
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/gdm85/go-rencode v0.1.8 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.0 // indirect
	github.com/go-playground/universal-translator v0.18.0 // indirect
	github.com/go-playground/validator/v10 v10.9.0 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/go-sql-driver/mysql v1.6.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/lib/pq v1.10.3 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/ugorji/go/codec v1.2.6 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519 // indirect
	golang.org/x/sys v0.0.0-20211013075003-97ac67df715c // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
