module github.com/Kellerman81/go_media_downloader/imdb

go 1.18

require (
	github.com/golang-migrate/migrate/v4 v4.15.1 //initialize db
	github.com/jmoiron/sqlx v1.3.4
	github.com/knadh/koanf v1.4.0
	github.com/mattn/go-sqlite3 v1.14.12 //data and imdb db
	golang.org/x/text v0.3.7
)

require (
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/sys v0.0.0-20220317061510-51cd9980dadf // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)
