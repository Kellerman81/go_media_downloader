module github.com/Kellerman81/go_media_downloader/pkg/imdb

go 1.25

require (
	github.com/golang-migrate/migrate/v4 v4.18.3 //initialize db
	github.com/h2non/filetype v1.1.3
	github.com/mattn/go-sqlite3 v1.14.32 //data and imdb db
	github.com/mozillazg/go-unidecode v0.2.0
	github.com/pelletier/go-toml/v2 v2.2.4
)

require (
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
)
