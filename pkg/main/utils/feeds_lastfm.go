package utils

import (
	"context"
	"errors"
	"strconv"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/lastfm"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// getlastfmprovider returns the registered Last.fm provider or creates a
// temporary one from config if the registry is empty. Returns nil (and an
// error) when no API key is configured.
func getlastfmprovider() (*lastfm.Provider, error) {
	if p := providers.GetLastFM(); p != nil {
		return p, nil
	}

	apiKey := config.GetSettingsGeneral().LastFMAPIKey
	if apiKey == "" {
		return nil, errors.New("lastfm_apikey is not configured")
	}

	return lastfm.NewProvider(), nil
}

// getlastfmtopartists fetches the Last.fm global top-artists chart and, for
// each artist that is not yet in dbartists, calls JobImportArtist to create the
// artist entry with full MusicBrainz metadata.  Artists (new or existing) are
// then added to d.Albums as ManualConfig entries so the normal album-import
// pipeline can fetch their releases.
//
// Config fields used from list.CfgList:
//   - Limit: how many top artists to fetch (default 100, max 1000 per Last.fm).
//     Set to 0 to use the default.
func (d *feedResults) getlastfmtopartists(
	ctx context.Context,
	list *config.MediaListsConfig,
	cfgp *config.MediaTypeConfig,
) error {
	// Resolve listid from cfgp.Lists so callers don't need to pass it.
	listid := -1
	for i := range cfgp.Lists {
		if cfgp.Lists[i].Name == list.Name {
			listid = i
			break
		}
	}

	if listid == -1 {
		return errors.New("lastfmtopartists: list not found in media config: " + list.Name)
	}
	p, err := getlastfmprovider()
	if err != nil {
		return err
	}

	limit, _ := strconv.Atoi(list.CfgList.Limit)
	if limit <= 0 {
		limit = 100
	}

	entries, err := p.GetTopArtists(ctx, 1, limit)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		logger.Logtype("warn", 1).
			Str("list", list.Name).
			Msg("lastfmtopartists: no entries returned from Last.fm")
		return nil
	}

	logger.Logtype("info", 2).
		Int("count", len(entries)).
		Str("list", list.Name).
		Msg("lastfmtopartists: fetched chart entries")

	for i := range entries {
		if err := logger.CheckContextEnded(ctx); err != nil {
			return err
		}

		name := entries[i].Name
		mbid := entries[i].MBID

		if name == "" {
			continue
		}

		// Check if artist already exists in dbartists (by MBID first, then name).
		var existingID uint

		if mbid != "" {
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbartists WHERE musicbrainz_id = ?",
				&existingID,
				&mbid,
			)
		}

		if existingID == 0 {
			artistSlug := logger.StringToSlug(name)
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbartists WHERE name = ? COLLATE NOCASE OR slug = ?",
				&existingID,
				&name,
				&artistSlug,
			)
		}

		if existingID == 0 {
			// Artist is new — import metadata from MusicBrainz and create the
			// dbartist row + artists tracking entry.
			_, importErr := importfeed.JobImportArtist(
				ctx,
				&importfeed.ArtistConfig{
					Name:          name,
					MusicBrainzID: mbid,
					TrackMode:     "albums",
				},
				cfgp,
				listid,
				true,
			)
			if importErr != nil {
				logger.Logtype("warn", 1).
					Str("artist", name).
					Str("mbid", mbid).
					Err(importErr).
					Msg("lastfmtopartists: JobImportArtist failed; skipping album import")
				continue
			}

			logger.Logtype("info", 1).
				Str("artist", name).
				Int("rank", entries[i].Rank).
				Msg("lastfmtopartists: new artist imported")
		} else {
			logger.Logtype("debug", 2).
				Str("artist", name).
				Uint("id", existingID).
				Msg("lastfmtopartists: artist already exists, will refresh albums")
		}

		// Queue for album import — ManualConfig with ArtistName (and ArtistID as MBID)
		// triggers JobImportDBAlbum in artist mode, which fetches all releases from
		// MusicBrainz and adds them to dbalbums.
		d.Albums = append(d.Albums, config.ManualConfig{
			ArtistName: name,
			ArtistID:   mbid,
		})
	}

	logger.Logtype("info", 1).
		Int("queued", len(d.Albums)).
		Str("list", list.Name).
		Msg("lastfmtopartists: artists queued for album import")

	return nil
}
