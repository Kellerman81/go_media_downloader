module github.com/Kellerman81/go_media_downloader/pkg/imdb

go 1.26.0

require (
	github.com/golang-migrate/migrate/v4 v4.20.1 //initialize db
	github.com/h2non/filetype v1.1.3
	github.com/mozillazg/go-unidecode v0.2.0
	github.com/pelletier/go-toml/v2 v2.4.3
	modernc.org/sqlite v1.62.0 //imdb db (pure Go, no cgo)
)

require (
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sys v0.49.0 // indirect
	modernc.org/libc v1.80.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)
