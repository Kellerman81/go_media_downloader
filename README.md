# Media Downloader
Media Manager similar to Radarr and Sonarr written in go
First version! Bugs included

## Features

- Get Feeds of wanted media (Movies: Imdb Public Lists, Series: Local Wanted file --series.toml--)
- Get Metadata from IMDB, TMDB, OMDB, Trakt, TheTVDB
- Local Imdb Cache
- Missing/Upgradeble monitoring of Episode/Movie
- Search Newznab/Tornab Indexers
- Download nzb to directory, to Nzbget and torrent/magnet to deluge (others might follow)
- Parse files/nzbs
- Structure releases and delete lower quality files/nzbs
- Send Notification after Download start and/or Download finish (currently csv or pushover)
- Scheduler interval based (every x Minutes, Hours, Days)
- Api to start jobs, list/add/edit/delete movies/series/episodes, other actions. For API Call examples look at the Powershell examples in the api example file
- External Api Limiter to reduce the chance of getting blocked
- toml Config for inital config and reconfiguration since there is no webinterface currently

## Missing

- Webinterface - currently included [AdminLTE - Bootstrap 4 Admin Dashboard](https://adminlte.io)
- better api -already done: /debug/pprof if Debug Mode is enabled, added some documentation
- better scheduler -to directly start or run at specific times
- optimize Parser
- DB Config instead of toml -partially done: use pudge as key-value-db for toml config and load config from there in each function - the backup folder will be cleared on each start!

## Get started

- for Docker: Download Repository - and create docker container for build/run using compose files/nzbs (~300-700mb RAM)
- for Windows/Linux/Mac: Download latest [prebuild zip](https://github.com/Kellerman81/go_media_downloader/releases/tag/latest_develop) for your OS
- for all: setup your config and name it config.toml ! - this will initialize the config.db and also update it on change
- for all: setup your series.toml if wanted
- for Docker: Start the build container to build the executable
- for Docker: Start the run container to run the executable
- for Windows/Linux/Mac: Run the go_media_downloader executable
- it will first build the imdb cache and then start importing your feeds, get metadata and scan/match the data - depending on your media repository size it might take several hours
- after the initial scan is finished the API and the scheduler will be started
- the scheduler will the scan for new releases and new/missing data in your folders and refresh your feeds

Find the API Documentation after start at:
(http://localhost:9090/swagger/index.html)
[Donate](https://www.paypal.com/donate?hosted_button_id=JRT8FJ6GG8CXN)