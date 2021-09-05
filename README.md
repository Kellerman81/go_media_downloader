# Media Downloader
Media Manager similar to Radarr and Sonarr written in go
First version! Bugs included

## Features

- Read Imdb csv Exported lists
- Get Metadata from IMDB, TMDB, OMDB, Trakt, TheTVDB
- Local Imdb Cache
- Missing/Upgradeble monitoring of Episode/Movie
- Search Newznab Indexers
- Download nzb to directory
- Parse files/nzbs
- Structure releases and delete lower quality files/nzbs
- Scheduler
- Api
- External Api Limiter
- toml Config

## Missing

- Webinterface - currently included [AdminLTE - Bootstrap 4 Admin Dashboard](https://adminlte.io)
- better api -already done: /debug/pprof if Debug Mode is enabled
- better scheduler -to directly start or run at specific times
- optimize Parser
- DB Config instead of toml -partially done: use pudge as key-value-db for toml config and load config from there in each function - the backup folder will be cleared on each start!

## Get started

- create docker container for build/run using compose files/nzbs (~300-700mb RAM)
- setup your config and name it config.toml ! - this will initialize the config.db and also update it on change
- start the run container - it will first build the imdb cache and then start importing your feeds, get metadata and scan/match the data
- after that the scheduler does the rest

[Donate](https://www.paypal.com/donate?hosted_button_id=JRT8FJ6GG8CXN)