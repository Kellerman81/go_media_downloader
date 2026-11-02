package api

import (
	"context"
	"errors"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// Music metadata provider keys used in the search UI.
const (
	provMusicBrainz = "musicbrainz"
	provDiscogs     = "discogs"
	provDeezer      = "deezer"
	provLastFM      = "lastfm"
	provITunes      = "itunes"
	provTheAudioDB  = "theaudiodb"
)

// errProviderUnavailable is returned when a requested provider is not configured.
var errProviderUnavailable = errors.New(
	"selected metadata provider is not configured — enable it under the music metadata sources",
)

// musicProviderLabels maps provider keys to human-friendly display names.
var musicProviderLabels = map[string]string{
	provMusicBrainz: "MusicBrainz",
	provDiscogs:     "Discogs",
	provDeezer:      "Deezer",
	provLastFM:      "Last.fm",
	provITunes:      "iTunes",
	provTheAudioDB:  "TheAudioDB",
}

// musicProviderLabel returns a display label for a provider key.
func musicProviderLabel(key string) string {
	if l, ok := musicProviderLabels[key]; ok {
		return l
	}

	return key
}

// musicAlbumSearchProviders returns the configured providers that can search albums,
// in source-priority-friendly order. Only providers registered at startup (i.e.
// enabled in the music metadata source configuration) are listed.
func musicAlbumSearchProviders() []string {
	list := make([]string, 0, 6)

	if providers.GetMusicBrainz() != nil {
		list = append(list, provMusicBrainz)
	}

	if providers.GetDiscogs() != nil {
		list = append(list, provDiscogs)
	}

	if providers.GetDeezer() != nil {
		list = append(list, provDeezer)
	}

	if providers.GetLastFM() != nil {
		list = append(list, provLastFM)
	}

	if providers.GetITunes() != nil {
		list = append(list, provITunes)
	}

	if providers.GetTheAudioDB() != nil {
		list = append(list, provTheAudioDB)
	}

	return list
}

// musicArtistProviders returns the configured providers that support artist discovery
// (search artists + list their releases). Only MusicBrainz and Discogs qualify.
func musicArtistProviders() []string {
	list := make([]string, 0, 2)
	if providers.GetMusicBrainz() != nil {
		list = append(list, provMusicBrainz)
	}

	if providers.GetDiscogs() != nil {
		list = append(list, provDiscogs)
	}

	return list
}

// musicSearchAlbums dispatches an album search to the chosen provider. artist may be
// empty; title carries the album name. Providers that need them separately (iTunes,
// TheAudioDB) receive artist/title directly; others get a combined query string.
// fellBack reports that the chosen provider is not configured and MusicBrainz
// answered instead.
func musicSearchAlbums(
	ctx context.Context,
	provider, artist, title string,
	limit int,
) (results []apiexternal_v2.ReleaseSearchResult, fellBack bool, err error) {
	query := strings.TrimSpace(strings.TrimSpace(artist) + " " + strings.TrimSpace(title))

	switch provider {
	case provDiscogs:
		if p := providers.GetDiscogs(); p != nil {
			results, err = p.SearchReleases(ctx, query, limit)
			return results, false, err
		}

	case provDeezer:
		if p := providers.GetDeezer(); p != nil {
			results, err = p.SearchAlbums(ctx, query, limit)
			return results, false, err
		}

	case provLastFM:
		if p := providers.GetLastFM(); p != nil {
			results, err = p.SearchAlbums(ctx, query, limit)
			return results, false, err
		}

	case provITunes:
		if p := providers.GetITunes(); p != nil {
			results, err = p.SearchAlbums(ctx, artist, title, limit)
			return results, false, err
		}

	case provTheAudioDB:
		if p := providers.GetTheAudioDB(); p != nil {
			results, err = p.SearchAlbums(ctx, artist, title)
			return results, false, err
		}
	}

	// Default / fallback (also used for an empty provider): MusicBrainz. Its
	// query is Lucene syntax, so names like "AC/DC" or "Sunn O)))" must be
	// escaped to be searched as plain text.
	if p := providers.GetMusicBrainz(); p != nil {
		results, _, err = p.SearchReleases(ctx, logger.LuceneEscape(query), limit, 0)
		return results, provider != "" && provider != provMusicBrainz, err
	}

	return nil, false, errProviderUnavailable
}

// musicSearchArtists dispatches an artist search to the chosen provider.
func musicSearchArtists(
	ctx context.Context,
	provider, query string,
	limit int,
) ([]apiexternal_v2.ArtistSearchResult, error) {
	if provider == provDiscogs {
		if p := providers.GetDiscogs(); p != nil {
			return p.SearchArtists(ctx, query, limit)
		}

		return nil, errProviderUnavailable
	}

	if p := providers.GetMusicBrainz(); p != nil {
		return p.SearchArtists(ctx, logger.LuceneEscape(query), limit)
	}

	return nil, errProviderUnavailable
}

// musicArtistReleases lists releases for a previously found artist using the
// provider-native id carried on the ArtistSearchResult.
func musicArtistReleases(
	ctx context.Context,
	provider string,
	artist *apiexternal_v2.ArtistSearchResult,
	limit int,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	if provider == provDiscogs {
		if p := providers.GetDiscogs(); p != nil && artist.DiscogsID > 0 {
			return p.GetArtistReleases(ctx, artist.DiscogsID, limit, 1)
		}

		return nil, errProviderUnavailable
	}

	if p := providers.GetMusicBrainz(); p != nil {
		mbid := artist.MusicBrainzID
		if mbid == "" {
			mbid = artist.ID
		}

		return p.GetArtistReleases(ctx, mbid, limit, 0)
	}

	return nil, errProviderUnavailable
}

// musicReleaseMatchesFilters reports whether a release passes the optional release-type
// and media-format filters. Empty filters accept everything.
func musicReleaseMatchesFilters(
	r *apiexternal_v2.ReleaseSearchResult,
	allowedTypes, formats []string,
) bool {
	if len(allowedTypes) > 0 && r.Type != "" {
		if !logger.SlicesContainsI(allowedTypes, r.Type) {
			return false
		}
	}

	if len(formats) == 0 {
		return true
	}

	if len(r.MediaFormats) == 0 {
		// No disc-format info to verify against — exclude when a filter is set.
		return false
	}

	for i := range r.MediaFormats {
		if !logger.SlicesContainsI(formats, r.MediaFormats[i]) {
			return false
		}
	}

	return true
}
