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
- Scheduler interval based (every x Minutes, Hours, Days) and/or cron based
- Api to start jobs, list/add/edit/delete movies/series/episodes, other actions. For API Call examples look at the Powershell examples in the api example file and at the Swagger Documentation
- External Api Limiter to reduce the chance of getting blocked
- toml Config for inital config and reconfiguration since there is no webinterface currently

### Supported
#### Feed Sources

- IMDB Public List (Movies)
- Trakt Public+Private List (Movies + Shows)
- Trakt Popular (Movies, Shows)
- Trakt Trending (Movies, Shows)
- Trakt Anticipated (Movies, Shows)
- local wanted file (Shows)

#### External Meta Sources

- IMDB (mostly Movies)
- TMDB (Movies)
- OMDB (Movies)
- TRAKT (Movies + Shows)
- TVDB (Shows)
#### Indexers

- Newznab
- Torznab

#### Downloaders

- Drone (File Download NZB,Torrent)
- Nzbget (Nzb)
- Sabnzb (Nzb)
- Deluge (Torrent/Magnet)

#### Notifications

- Pushover
- CSV (Write custom csv row)

#### Media Templates

- Series - You can configure multiple series groups and each group can contain multiple feeds and folders
- Movies - You can configure multiple movie groups and each group can contain multiple feeds and folders

#### General

- Regex Filtering Releases (Custom Filters for required and rejected)
- Schedulers (different intervals for all the actions) Support Interval and Cron based
- Limiters (You can define how much you want to storm an API)
- Configure which Meta Sources to use
- Configure your Qualities and their priorities (use parse/quality api to test this) including wanted and Defaults
- Use FProbe to get Media Information (dimensions, runtime, audio language)
- Currently completly API and Scheduler controlled - NO User Inteface yet - contact me if you want to create one - i most likly won't

### Under Consideration

- Download State Tracking
- Downloading of images from Meta Sites (unlikely)
- Handle multi media releases (ex. -CD1 -CD2) (maybe even joining those) (at the bottom of the list)
- Maybe also add the ability to download non matched episodes (which could't be found on a meta source site - risky since you might get a lot unwanted stuff) (at the bottom of the list) - using the download API you can Download excluded releases
- Unpacking of Downloaded stuff? Currently I let the downloaders do that and don't care about it that much
- Other Media Type Support (i could think of music but don't want to include this currently)
- Switch to db only configuration if I include a full webinterface
- Other Feed Sources (currently i don't need more - start a discussion if you want one specific - also you can add stuff using the API)
- Other Indexer Sources (currently i don't need more - start a discussion if you want one specific)
- Other Downloaders (currently i don't need more - start a discussion if you want one specific - ex qBittorrent)
- Other Notifications (currently i don't need more - start a discussion if you want one specific)
- Always: Add Additional Logging (currently mostly debugging stuff)
- Always: API Changes

## Ram Usage

Currently seen: 
Constanly in use - between 30MB-150MB
Swap Memory: Default ~300-600MB - on a file move action the swap memory will grow to at least the complete file size - so for a 8GB file expect also this much!

## Get started

- for Docker: Download Repository - and create docker container for build/run using compose files/nzbs
- for Windows/Linux/Mac: Download latest [prebuild zip](https://github.com/Kellerman81/go_media_downloader/releases/tag/latest_develop) for your OS
- for all: setup your config and name it config.toml ! - this will initialize the config.db and also update it on change
- for all: setup your series.toml if wanted
- for Docker: Start the build container to build the executable
- for Docker: Start the run container to run the executable
- for Windows/Linux/Mac: Run the go_media_downloader executable
- it will first build the imdb cache and then start importing your feeds, get metadata and scan/match the data - depending on your media repository size it might take several hours
- after the initial scan is finished the API and the scheduler will be started
- the scheduler will the scan for new releases and new/missing data in your folders and refresh your feeds
- You have to configure the templates and Media Groups - also if you want to use trakt, tmdb or omdb you have to get API Keys from them and enter them in the config

Find the API Documentation after start at:
http://{server}:{port}/swagger/index.html


## After first Start - Trakt Authorize

- Create a Application within Trakt: https://trakt.tv/oauth/applications
- Write down ClientID and Secret
- Put ClientID and Secret into the config.toml
- Start app
- Open : http://{server}:{port}/api/trakt/auhorize?apikey={apikey}  to get a url to open
- Open Url in Browser
- Open : http://{server}:{port}/api/trakt/token/{code}?apikey={apikey}  to get the token and save it - the code is in the url from the step above
- Best Practice restart App - You need to do this only once every 3 month if the expiry is reached

## Donate

[Donate](https://www.paypal.com/donate?hosted_button_id=JRT8FJ6GG8CXN)

## Examples

- config: Movies FR and EN as Groups with the same or different feeds - Folders have to be different for the group (don't mix for example english and French Movies) - Use Different Qualities for each group! in the qualities are the Downloader/Indexer Definitions thats why - also please let the downloader place them in different directories otherwise they might mix
- exceptions: if a movie/show is in multiple lists i try to exclude them from the others (can be done in the config) so that i have the movie/show only in the list with the highest quality - if a movie/show is in 2 lists with different qualities the release would be constantly redownloaded