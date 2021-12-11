module github.com/Kellerman81/go_media_downloader/imdb

go 1.17

require (
	github.com/Kellerman81/go_media_downloader v0.0.0-20211209202851-8edb999bf5d8
	github.com/golang-migrate/migrate/v4 v4.15.1 //initialize db
	github.com/h2non/filetype v1.1.3
	github.com/jmoiron/sqlx v1.3.4
	github.com/knadh/koanf v1.3.3
	github.com/mattn/go-sqlite3 v1.14.9 //data and imdb db
	github.com/remeh/sizedwaitgroup v1.0.0 //concurrent wait group
)

require (
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/sys v0.0.0-20211205182925-97ca703d548d // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)
