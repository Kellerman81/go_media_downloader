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
- better api
- better scheduler
- optimize Parser
- DB Config instead of toml

## Get started

- create docker container for build/run using compose files/nzbs (~300-700mb RAM)
- setup your config and name it config.toml !
- start the run container - it will first build the imdb cache and then start importing your feeds, get metadata and scan/match the data
- after that the scheduler does the rest
