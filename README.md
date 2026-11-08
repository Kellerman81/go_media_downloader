# Media Downloader
Media Manager similar to Radarr and Sonarr written in go

Main goal of project: Support multiple configurations for Series/Movies in one. Also do some additional checks and reduce memory footprint.

Inspirations: Radarr/Sonarr and Flexget and beets

## Table of Contents
* [Features](#features)
  * [Supported](#supported)
    * [Feed Sources](#feed-sources)
    * [External Meta Sources](#external-meta-sources)
    * [Indexers](#indexers)
    * [Downloaders](#downloaders)
    * [Notifications](#notifications)
    * [Media Templates](#media-templates)
    * [General](#general)
  * [Under Consideration](#under-consideration)
* [Ram Usage](#ram-usage)
* [Get started](#get-started)
* [After first Start - Trakt Authorize](#after-first-start---trakt-authorize)
* [Donate](#donate)
* [Examples](#examples)
* [History](#history)
* [References](#references)

## Features

- Get Feeds of wanted media (Movies: Imdb Public Lists, Series: Local Wanted file --series.toml--)
- Get Metadata from IMDB, TMDB, OMDB, Trakt, TheTVDB...
- Local Imdb Cache
- Missing/Upgradeble monitoring of Episode/Movie/Music/Book/Audiobook
- Search Newznab/Tornab Indexers
- Download nzb/torrent to a directory or send them to one of 14 Usenet/torrent clients (see [Downloaders](#downloaders))
- Track sent downloads (added, downloading with progress, completed, failed, removed) and what the structure job did with them (processed, rejected); a failed or rejected download makes the item searchable again, and failed downloads can be removed from the client (`remove_failed`)
- Parse files/nzbs
- Structure releases and delete lower quality files/nzbs
- Send Notifications through 19 services and local programs (see [Notifications](#notifications)) when a download starts, a file is organized or upgraded, an entry is added to a list, or a new title is added to the database
- Scheduler interval based (every x Minutes, Hours, Days) and/or cron based
- Api to start jobs, list/add/edit/delete movies/series/episodes, other actions. For API Call examples look at the Powershell examples in the api example file and at the Swagger Documentation
- External Api Limiter to reduce the chance of getting blocked
- toml Config for inital config and reconfiguration

### Supported
#### Feed Sources

- IMDB Public List (Movies) - CSV via URL (`imdbcsv`) or local file (`imdbfile`)
- Trakt Public+Private List (Movies + Shows)
- Trakt Popular (Movies, Shows)
- Trakt Trending (Movies, Shows)
- Trakt Anticipated (Movies, Shows)
- TMDB Lists (Movies + Shows)
- TMDB Discover (Movies + Shows)
- TMDB Popular (Movies + Shows)
- TMDB Trending (Movies + Shows)
- TMDB Upcoming (Movies)
- Newznab RSS feed (Movies + Shows)
- Plex Watchlist (Movies + Shows)
- Jellyfin Watchlist (Movies + Shows)
- Movie Web Scraper - HTML/XPath or CSRF API (`moviescraper`)
- Rotten Tomatoes browse pages and editorial guides (`rottentomatoes`) and Letterboxd watchlists and lists (`letterboxd`) - matched to IMDb by title and year
- local TOML config file (Shows, Audiobooks, Books, Music)
- Music Charts scraper - offiziellecharts.de, officialcharts.com, or any HTML chart page (`musiccharts`)
- Last.fm Top Artists (`lastfmtopartists`)
- Book Bestsellers scraper - Thalia, Spiegel, or any HTML bestseller page (`bookbestsellers`)

#### External Meta Sources

- IMDB (mostly Movies)
- TMDB (Movies + Shows)
- OMDB (Movies)
- TRAKT (Movies + Shows)
- TVDB (Shows)
- TVmaze (Shows)
- Audnex (Audiobooks)
- Audible (Audiobooks, per region)
- OpenLibrary (Books)
- Goodreads (Books)
- MusicBrainz (Music)
- AcoustID (Music fingerprinting)
- Last.fm (Music)
- Discogs (Music)
- Deezer (Music)
- TheAudioDB (Music)
- iTunes (Music)

#### Indexers

- Newznab
- Torznab

#### Downloaders

- Drone (File Download NZB,Torrent) - `drone`
- Nzbget (Nzb) - `nzbget`
- Sabnzb (Nzb) - `sabnzbd`
- NZBVortex (Nzb)* - `nzbvortex`
- qBittorrent (Torrent/Magnet) - `qbittorrent`
- rTorrent (Torrent/Magnet untested) - `rtorrent`
- Deluge (Torrent/Magnet) - `deluge`
- Transmission (Torrent/Magnet) - `transmission`
- Vuze with the Web Remote plugin (Torrent/Magnet)* - `vuze`
- aria2 (Torrent/Magnet) - `aria2`
- Flood (Torrent/Magnet) - `flood`
- uTorrent (Torrent/Magnet) - `utorrent`
- rqbit (Torrent/Magnet) - `rqbit`
- Tribler (Torrent/Magnet) - `tribler`
- Synology Download Station (Torrent/Magnet)* - `downloadstation`

\* implemented from the client's API and Sonarr's client code but not yet tested against a real installation (NZBVortex is macOS-only, Download Station needs DSM; Vuze uses the Transmission RPC). aria2, Flood, uTorrent, rqbit and Tribler were tested against their Docker images.

#### Notifications

- CSV (Write custom csv row) - `csv`
- Pushover - `pushover`
- Gotify - `gotify`
- Apprise - `apprise`
- Pushbullet - `pushbullet`
- Email via SMTP - `sendmail`
- Discord (webhook)* - `discord`
- Slack (incoming webhook)* - `slack`
- Telegram (bot)* - `telegram`
- ntfy - `ntfy`
- Webhook (JSON POST to any URL)* - `webhook`
- Signal (via signal-cli-rest-api)* - `signal`
- Join* - `join`
- Prowl* - `prowl`
- Pushcut* - `pushcut`
- Pushsafer* - `pushsafer`
- Simplepush* - `simplepush`
- Mailgun (email)* - `mailgun`
- SendGrid (email)* - `sendgrid`
- Run a program - `exec` (gets title and message as arguments and as GMD_TITLE / GMD_MESSAGE; started without a shell)

\* implemented from the service's API but not yet tested against the real service. ntfy was tested against its Docker image; for Signal only the connection check and the server's "not registered" answer were tested, without a registered number.

Notification events (per media group):
- `added_download` - a release was sent to a download client
- `added_data` - a file was organized into the library
- `upgraded_data` - an organized file replaced older ones
- `added_to_list` - an entry was added to one of the group's lists
- `added_to_db` - a new movie/series/book/audiobook/album was added to the database

#### Media Templates

- Series - You can configure multiple series groups and each group can contain multiple feeds and folders
- Movies - You can configure multiple movie groups and each group can contain multiple feeds and folders
- Music - You can configure multiple music groups and each group can contain multiple feeds and folders
- Book - You can configure multiple book groups and each group can contain multiple feeds and folders
- Audiobook - You can configure multiple audiobook groups and each group can contain multiple feeds and folders

#### General

- Regex Filtering Releases (Custom Filters for required and rejected)
- Schedulers (different intervals for all the actions) Support Interval and Cron based
- Limiters (You can define how much you want to storm an API)
- Configure which Meta Sources to use
- Configure your Qualities and their priorities (use parse/quality api to test this) including wanted and Defaults
- Use FProbe or mediainfo to get Media Information (dimensions, runtime, audio language)
- Currently completly API and Scheduler controlled
- Webinterface V2 can be accessed by / - Username admin - Password is the WebApiKey   -  BUT: It is in Development Backup Config and Database before using - it could break

### Under Consideration

- Downloading of images from Meta Sites (unlikely)
- Handle multi media releases (ex. -CD1 -CD2) (maybe even joining those) (at the bottom of the list)
- Maybe also add the ability to download non matched episodes (which could't be found on a meta source site - risky since you might get a lot unwanted stuff) (at the bottom of the list) - using the download API you can Download excluded releases
- Other Media Type Support 
- Switch to db only configuration if I include a full webinterface
- Other Feed Sources (currently i don't need more - start a discussion if you want one specific - also you can add stuff using the API)
- Other Indexer Sources (currently i don't need more - start a discussion if you want one specific)
- Other Downloaders (start a discussion if you want one specific - not planned: Freebox, Hadouken, Pneumatic)
- Other Notifications (start a discussion if you want one specific - not planned: Custom Script, Notifiarr, Twitter)
- Always: Add Additional Logging (currently mostly debugging stuff)
- Always: API Changes

## Ram Usage

Currently seen: 
Constanly in use - between 100MB-500MB-peek:1-1.5GB
Swap Memory: Default ~300-600MB - on a file move action the swap memory will grow to at least the complete file size - so for a 8GB file expect also this much!

## Get started

- Guide for Docker:
  - create a compose file with the following content:
  ```
  services:
    media_downloader:
      image: kellerman81/go_media_downloader:latest
      volumes:
        - /mnt/user/go_media_downloader/databases:/app/databases
        - /mnt/user/go_media_downloader/logs:/app/logs
        - /mnt/user/go_media_downloader/config:/app/config
      ports:
        - 9094:9090
  ```
	- Start the container -> the config and the databases will be created
  - Stop the container
	- setup your config "config.toml"
	- setup your series.toml if wanted
	- Start the container again -> the metadata will be downloaded and the scheduler will be started (if enabled)
  
- Guide for Windows/Linux:
	- Download latest [release](https://github.com/Kellerman81/go_media_downloader/releases) for your OS
	- setup your config and name it config.toml -> it will create an example if no config could be found
	- setup your series.toml if wanted
	- Run the go_media_downloader executable

- it will first build the imdb cache and then start importing your feeds, get metadata and scan/match the data - depending on your media repository size it might take several hours
- after the initial scan is finished the API and the scheduler will be started
- the scheduler will then scan for new releases and new/missing data in your folders and refresh your feeds
- You have to configure the templates and Media Groups - also if you want to use trakt, tmdb or omdb you have to get API Keys from them and enter them in the config

Find the API Documentation after start at:
http://{server}:{port}/swagger/index.html
or with more features but maybe not as current:
https://go-media-downloader.readme.io/


## After first Start - Trakt Authorize

- (This part is optional and only needed if you want to get private trakt lists if you have a premium account)
- Create a Application within Trakt: https://trakt.tv/oauth/applications - redirect url needs to be: http://localhost:9090
- Write down ClientID and Secret
- Put ClientID and Secret into the config.toml
- Start app
- Open : http://{server}:{port}/api/trakt/authorize?apikey={apikey}  to get a url to open
- Open Url in Browser
- Open : http://{server}:{port}/api/trakt/token/{code}?apikey={apikey}  to get the token and save it - the code is in the url from the step above
- Best Practice restart App - You need to do this only once every 3 month if the expiry is reached

- The Auth can now also be done from the new Webinterface which is now the preferred way

## Donate

[Donate](https://www.paypal.com/donate/?hosted_button_id=QHYKHZXFNGQ2N)

## Examples

- Look into [Wiki](https://github.com/Kellerman81/go_media_downloader/wiki)
- config: Movies FR and EN as Groups with the same or different feeds - Folders have to be different for the group (don't mix for example english and French Movies) - Use Different Qualities for each group! in the qualities are the Downloader/Indexer Definitions thats why - also please let the downloader place them in different directories otherwise they might mix
- exceptions: if a movie/show is in multiple lists i try to exclude them from the others (can be done in the config) so that i have the movie/show only in the list with the highest quality - if a movie/show is in 2 lists with different qualities the release would be constantly redownloaded
- if you want more help start a discussion - the system is running stable for some time now
- TmDBDiscover Hallmark Movies: tmdb_discover = ['include_video=true&sort_by=primary_release_date.desc&with_companies=4056&with_genres=10749|10751|35'] 

## History

- V0.1: initial Release
- V0.1.1: Performance and Ram Usage Changes - rewrite a lot of functions
- v0.2.3: Added Tmdb List retrieve + tmdbdiscover
- v0.2.7: Added new Webinterface /api/admin - username is admin and password is the WebApiKey
- v0.3.0: Added Music and Audiobook support

## References

This Program uses the following API's (if wanted and if you register your own API key)

Movies / Shows
- [IMDB](https://www.imdb.com/)
- [OMDB](https://www.omdbapi.com/)
- [TheMovieDB](https://www.themoviedb.org/)
- [Trakt.tv](https://www.trakt.tv/)
- [TheTVDB](https://thetvdb.com/)
- [TVmaze](https://www.tvmaze.com/)
- [Plex](https://www.plex.tv/)
- [Jellyfin](https://jellyfin.org/)

Music
- [MusicBrainz](https://musicbrainz.org/doc/MusicBrainz_API)
- [AcoustID](https://acoustid.org/webservice)
- [Last.fm](https://www.last.fm/api)
- [Discogs](https://www.discogs.com/developers)
- [Deezer](https://developers.deezer.com/api)
- [TheAudioDB](https://www.theaudiodb.com/api.php)
- [iTunes](https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI)
- [Spotify](https://developer.spotify.com/documentation/web-api)
- [Lyrics.ovh](https://lyrics.ovh/)
- [LRCLib](https://lrclib.net/)

Audiobooks
- [Audnex](https://api.audnex.us/)
- [Audible](https://www.audible.com/)

Books
- [OpenLibrary](https://openlibrary.org/)
- [Goodreads](https://www.goodreads.com/)

Downloaders
- [NZBGet](https://nzbget.com/)
- [SABnzbd](https://sabnzbd.org/)
- [NZBVortex](https://www.nzbvortex.com/)
- [qBittorrent](https://www.qbittorrent.org/)
- [rTorrent](https://github.com/rakshasa/rtorrent)
- [Deluge](https://deluge-torrent.org/)
- [Transmission](https://transmissionbt.com/)
- [Vuze](https://www.vuze.com/)
- [aria2](https://aria2.github.io/)
- [Flood](https://flood.js.org/)
- [uTorrent](https://www.utorrent.com/)
- [rqbit](https://github.com/ikatson/rqbit)
- [Tribler](https://www.tribler.org/)
- [Synology Download Station](https://www.synology.com/dsm/packages/DownloadStation)

Notifications
- [Pushover](https://www.pushover.net/)
- [Pushbullet](https://www.pushbullet.com/)
- [Gotify](https://gotify.net/)
- [Apprise](https://github.com/caronc/apprise)
- [Discord Webhooks](https://support.discord.com/hc/en-us/articles/228383668-Intro-to-Webhooks)
- [Slack Incoming Webhooks](https://api.slack.com/messaging/webhooks)
- [Telegram Bots](https://core.telegram.org/bots)
- [ntfy](https://ntfy.sh/)
- [signal-cli-rest-api](https://github.com/bbernhard/signal-cli-rest-api)
- [Join](https://joaoapps.com/join/)
- [Prowl](https://www.prowlapp.com/)
- [Pushcut](https://www.pushcut.io/)
- [Pushsafer](https://www.pushsafer.com/)
- [Simplepush](https://simplepush.io/)
- [Mailgun](https://www.mailgun.com/)
- [SendGrid](https://sendgrid.com/)

For a list of the used Go Modules please look into the go.mod File in pkg/main and pkg/imdb

## Bugs

- Currently it may happen that new series won't be searched - use /api/series/search/id/123 endpoint in that case once  after that it will update
- currently movies/series in different languages need to be in diffent root folders - ex. /share/Movies_EN and ..Movies_FR
- If a movie or episode isn't imported after download you might need to disable the runtime check
- it can happen that an updated episode file doesnt remove the old file