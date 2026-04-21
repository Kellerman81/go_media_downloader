package musicbrainz

import (
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

//
// MusicBrainz Internal Types - Used for JSON unmarshaling
//

// mbArtistSearchResponse represents the artist search API response.
type mbArtistSearchResponse struct {
	Created string     `json:"created"`
	Count   int        `json:"count"`
	Offset  int        `json:"offset"`
	Artists []mbArtist `json:"artists"`
}

// mbArtist represents an artist.
type mbArtist struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	SortName       string       `json:"sort-name"`
	Type           string       `json:"type"`
	Country        string       `json:"country"`
	Area           mbArea       `json:"area"`
	BeginArea      mbArea       `json:"begin-area"`
	EndArea        mbArea       `json:"end-area"`
	Disambiguation string       `json:"disambiguation"`
	LifeSpan       mbLifeSpan   `json:"life-span"`
	Aliases        []mbAlias    `json:"aliases"`
	Tags           []mbTag      `json:"tags"`
	Relations      []mbRelation `json:"relations"`
	Score          int          `json:"score"`
}

// mbArtistResponse is the full artist response.
type mbArtistResponse = mbArtist

// mbReleaseSearchResponse represents the release search API response.
type mbReleaseSearchResponse struct {
	Created  string      `json:"created"`
	Count    int         `json:"count"`
	Offset   int         `json:"offset"`
	Releases []mbRelease `json:"releases"`
}

// mbRelease represents a release (album edition).
type mbRelease struct {
	ID                 string           `json:"id"`
	Title              string           `json:"title"`
	Status             string           `json:"status"`
	Disambiguation     string           `json:"disambiguation"`
	Date               string           `json:"date"`
	Country            string           `json:"country"`
	Barcode            string           `json:"barcode"`
	ASIN               string           `json:"asin"`
	Quality            string           `json:"quality"`
	ArtistCredit       []mbArtistCredit `json:"artist-credit"`
	ReleaseGroup       mbReleaseGroup   `json:"release-group"`
	LabelInfo          []mbLabelInfo    `json:"label-info"`
	Media              []mbMedium       `json:"media"`
	TextRepresentation mbTextRep        `json:"text-representation"`
	Tags               []mbTag          `json:"tags"`
	Score              int              `json:"score"`
}

// mbReleaseResponse is the full release response.
type mbReleaseResponse = mbRelease

// mbReleaseGroupSearchResponse represents the release group search response.
type mbReleaseGroupSearchResponse struct {
	Created       string           `json:"created"`
	Count         int              `json:"count"`
	Offset        int              `json:"offset"`
	ReleaseGroups []mbReleaseGroup `json:"release-groups"`
}

// mbReleaseGroup represents a release group (album across editions).
type mbReleaseGroup struct {
	ID               string           `json:"id"`
	Title            string           `json:"title"`
	PrimaryType      string           `json:"primary-type"`
	SecondaryTypes   []string         `json:"secondary-types"`
	Disambiguation   string           `json:"disambiguation"`
	FirstReleaseDate string           `json:"first-release-date"`
	ArtistCredit     []mbArtistCredit `json:"artist-credit"`
	Releases         []mbRelease      `json:"releases"`
	Tags             []mbTag          `json:"tags"`
	Score            int              `json:"score"`
}

// mbReleaseGroupResponse is the full release group response.
type mbReleaseGroupResponse = mbReleaseGroup

// mbRecordingSearchResponse represents the recording search response.
type mbRecordingSearchResponse struct {
	Created    string        `json:"created"`
	Count      int           `json:"count"`
	Offset     int           `json:"offset"`
	Recordings []mbRecording `json:"recordings"`
}

// mbRecording represents a recording (track).
type mbRecording struct {
	ID               string           `json:"id"`
	Title            string           `json:"title"`
	Length           int              `json:"length"` // milliseconds
	Disambiguation   string           `json:"disambiguation"`
	Video            bool             `json:"video"`
	FirstReleaseDate string           `json:"first-release-date"`
	ArtistCredit     []mbArtistCredit `json:"artist-credit"`
	Releases         []mbRelease      `json:"releases"`
	ISRCs            []string         `json:"isrcs"`
	Tags             []mbTag          `json:"tags"`
	Score            int              `json:"score"`
}

// mbRecordingResponse is the full recording response.
type mbRecordingResponse = mbRecording

// mbISRCResponse represents the ISRC lookup response.
type mbISRCResponse struct {
	ISRC       string        `json:"isrc"`
	Recordings []mbRecording `json:"recordings"`
}

// mbLabelResponse represents a record label.
type mbLabelResponse struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	SortName       string     `json:"sort-name"`
	Type           string     `json:"type"`
	Country        string     `json:"country"`
	Area           mbArea     `json:"area"`
	LabelCode      string     `json:"label-code"`
	Disambiguation string     `json:"disambiguation"`
	LifeSpan       mbLifeSpan `json:"life-span"`
	Tags           []mbTag    `json:"tags"`
}

// Helper types

type mbArea struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SortName string `json:"sort-name"`
}

type mbLifeSpan struct {
	Begin string `json:"begin"`
	End   string `json:"end"`
	Ended bool   `json:"ended"`
}

type mbAlias struct {
	Name     string `json:"name"`
	SortName string `json:"sort-name"`
	Type     string `json:"type"`
	Locale   string `json:"locale"`
	Primary  bool   `json:"primary"`
}

type mbTag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type mbRelation struct {
	Type      string `json:"type"`
	TypeID    string `json:"type-id"`
	URL       mbURL  `json:"url"`
	Direction string `json:"direction"`
}

type mbURL struct {
	ID       string `json:"id"`
	Resource string `json:"resource"`
}

type mbArtistCredit struct {
	Name       string   `json:"name"`
	Artist     mbArtist `json:"artist"`
	JoinPhrase string   `json:"joinphrase"`
}

type mbLabelInfo struct {
	CatalogNumber string  `json:"catalog-number"`
	Label         mbLabel `json:"label"`
}

type mbLabel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type mbMedium struct {
	Position   int       `json:"position"`
	Format     string    `json:"format"`
	TrackCount int       `json:"track-count"`
	Tracks     []mbTrack `json:"tracks"`
}

type mbTrack struct {
	ID        string      `json:"id"`
	Number    string      `json:"number"`
	Title     string      `json:"title"`
	Length    int         `json:"length"`
	Position  int         `json:"position"`
	Recording mbRecording `json:"recording"`
}

type mbTextRep struct {
	Language string `json:"language"`
	Script   string `json:"script"`
}

//
// Conversion Functions
//

func convertArtistSearchResults(artists []mbArtist) []apiexternal_v2.ArtistSearchResult {
	results := make([]apiexternal_v2.ArtistSearchResult, 0, len(artists))

	for i := range artists {
		result := apiexternal_v2.ArtistSearchResult{
			ID:             artists[i].ID,
			Name:           artists[i].Name,
			SortName:       artists[i].SortName,
			Type:           artists[i].Type,
			Country:        artists[i].Country,
			Disambiguation: artists[i].Disambiguation,
			MusicBrainzID:  artists[i].ID,
			ProviderType:   apiexternal_v2.ProviderMusicBrainz,
		}

		// Area
		if artists[i].Area.Name != "" {
			result.Area = artists[i].Area.Name
		}

		// Active years
		if artists[i].LifeSpan.Begin != "" {
			result.BeginYear = extractYear(artists[i].LifeSpan.Begin)
		}

		if artists[i].LifeSpan.End != "" {
			result.EndYear = extractYear(artists[i].LifeSpan.End)
		}

		results = append(results, result)
	}

	return results
}

func convertArtistToDetails(artist *mbArtistResponse) *apiexternal_v2.ArtistDetails {
	details := &apiexternal_v2.ArtistDetails{
		ID:             artist.ID,
		Name:           artist.Name,
		SortName:       artist.SortName,
		Type:           artist.Type,
		Country:        artist.Country,
		Disambiguation: artist.Disambiguation,
		MusicBrainzID:  artist.ID,
		ProviderType:   apiexternal_v2.ProviderMusicBrainz,
	}

	// Area
	if artist.Area.Name != "" {
		details.Area = artist.Area.Name
	}

	// Life span
	if artist.LifeSpan.Begin != "" {
		details.BeginDate = parseMBDate(artist.LifeSpan.Begin)
	}

	if artist.LifeSpan.End != "" {
		details.EndDate = parseMBDate(artist.LifeSpan.End)
	}

	details.IsEnded = artist.LifeSpan.Ended

	// Aliases
	if len(artist.Aliases) > 0 {
		aliases := make([]string, 0, len(artist.Aliases))
		for i := range artist.Aliases {
			aliases = append(aliases, artist.Aliases[i].Name)
		}

		details.Aliases = aliases
	}

	// Tags/Genres
	if len(artist.Tags) > 0 {
		tags := make([]string, 0, len(artist.Tags))
		for i := range artist.Tags {
			tags = append(tags, artist.Tags[i].Name)
		}

		details.Genres = tags
	}

	// URLs from relations
	for i := range artist.Relations {
		rel := artist.Relations[i]
		if rel.URL.Resource == "" {
			continue
		}

		switch rel.Type {
		case "official homepage":
			details.Website = rel.URL.Resource
		case "wikidata":
			details.WikidataID = extractWikidataID(rel.URL.Resource)
		}
	}

	return details
}

func convertReleaseSearchResults(releases []mbRelease) []apiexternal_v2.ReleaseSearchResult {
	results := make([]apiexternal_v2.ReleaseSearchResult, 0, len(releases))

	for i := range releases {
		result := apiexternal_v2.ReleaseSearchResult{
			ID:             releases[i].ID,
			Title:          releases[i].Title,
			Status:         releases[i].Status,
			Country:        releases[i].Country,
			Barcode:        releases[i].Barcode,
			MusicBrainzID:  releases[i].ID,
			ReleaseGroupID: releases[i].ReleaseGroup.ID,
			ProviderType:   apiexternal_v2.ProviderMusicBrainz,
		}

		// Release date
		if releases[i].Date != "" {
			result.ReleaseYear = extractYear(releases[i].Date)
		}

		// Artists
		if len(releases[i].ArtistCredit) > 0 {
			artists := make([]apiexternal_v2.ArtistRef, len(releases[i].ArtistCredit))
			for j := range releases[i].ArtistCredit {
				artists[j] = apiexternal_v2.ArtistRef{
					Name: releases[i].ArtistCredit[j].Artist.Name,
					ID:   releases[i].ArtistCredit[j].Artist.ID,
				}
			}

			result.Artists = artists
		}

		// Label
		if len(releases[i].LabelInfo) > 0 && releases[i].LabelInfo[0].Label.Name != "" {
			result.Label = releases[i].LabelInfo[0].Label.Name
			result.CatalogNumber = releases[i].LabelInfo[0].CatalogNumber
		}

		// Format from media
		if len(releases[i].Media) > 0 {
			result.Format = releases[i].Media[0].Format
			// Collect per-disc formats and count total tracks
			mediaFormats := make([]string, len(releases[i].Media))

			totalTracks := 0
			for j := range releases[i].Media {
				mediaFormats[j] = releases[i].Media[j].Format
				totalTracks += releases[i].Media[j].TrackCount
			}

			result.MediaFormats = mediaFormats
			result.TrackCount = totalTracks
		}

		// Release group type
		if releases[i].ReleaseGroup.PrimaryType != "" {
			result.Type = releases[i].ReleaseGroup.PrimaryType
		}

		results = append(results, result)
	}

	return results
}

func convertReleaseToDetails(release *mbReleaseResponse) *apiexternal_v2.ReleaseDetails {
	details := &apiexternal_v2.ReleaseDetails{
		ID:            release.ID,
		Title:         release.Title,
		Status:        release.Status,
		Country:       release.Country,
		Barcode:       release.Barcode,
		ASIN:          release.ASIN,
		MusicBrainzID: release.ID,
		ProviderType:  apiexternal_v2.ProviderMusicBrainz,
	}

	// Release date
	if release.Date != "" {
		details.ReleaseDate = parseMBDate(release.Date)
		details.ReleaseYear = extractYear(release.Date)
	}

	// Artists
	if len(release.ArtistCredit) > 0 {
		artists := make([]apiexternal_v2.ArtistRef, 0, len(release.ArtistCredit))
		for i := range release.ArtistCredit {
			artists = append(artists, apiexternal_v2.ArtistRef{
				Name: release.ArtistCredit[i].Artist.Name,
				ID:   release.ArtistCredit[i].Artist.ID,
			})
		}

		details.Artists = artists
	}

	// Label info
	if len(release.LabelInfo) > 0 {
		details.Label = release.LabelInfo[0].Label.Name
		details.LabelID = release.LabelInfo[0].Label.ID
		details.CatalogNumber = release.LabelInfo[0].CatalogNumber
	}

	// Release group
	if release.ReleaseGroup.ID != "" {
		details.ReleaseGroupID = release.ReleaseGroup.ID

		details.Type = release.ReleaseGroup.PrimaryType
		if len(release.ReleaseGroup.SecondaryTypes) > 0 {
			details.SecondaryTypes = release.ReleaseGroup.SecondaryTypes
		}
	}

	// Language
	if release.TextRepresentation.Language != "" {
		details.Language = release.TextRepresentation.Language
	}

	// Media and tracks
	if len(release.Media) > 0 {
		details.Format = release.Media[0].Format
		details.DiscCount = len(release.Media)

		tracks := make([]apiexternal_v2.Track, 0)
		totalTracks := 0

		for discNum := range release.Media {
			for i := range release.Media[discNum].Tracks {
				track := apiexternal_v2.Track{
					ID:         release.Media[discNum].Tracks[i].Recording.ID,
					Title:      release.Media[discNum].Tracks[i].Title,
					Position:   release.Media[discNum].Tracks[i].Position,
					DiscNumber: discNum + 1,
					Duration: time.Duration(
						release.Media[discNum].Tracks[i].Length,
					) * time.Millisecond,
					MusicBrainzID: release.Media[discNum].Tracks[i].Recording.ID,
				}
				if len(release.Media[discNum].Tracks[i].Recording.ISRCs) > 0 {
					track.ISRC = release.Media[discNum].Tracks[i].Recording.ISRCs[0]
				}

				tracks = append(tracks, track)
			}

			totalTracks += release.Media[discNum].TrackCount
		}

		details.Tracks = tracks
		details.TrackCount = totalTracks
	}

	// Tags — prefer release-level tags, fall back to release-group tags
	tags := release.Tags
	if len(tags) == 0 {
		tags = release.ReleaseGroup.Tags
	}

	if len(tags) > 0 {
		genres := make([]string, 0, len(tags))
		for i := range tags {
			genres = append(genres, tags[i].Name)
		}

		details.Genres = genres
	}

	return details
}

func convertReleaseGroupSearchResults(
	groups []mbReleaseGroup,
) []apiexternal_v2.ReleaseSearchResult {
	results := make([]apiexternal_v2.ReleaseSearchResult, 0, len(groups))

	for i := range groups {
		result := apiexternal_v2.ReleaseSearchResult{
			ID:             groups[i].ID,
			Title:          groups[i].Title,
			Type:           groups[i].PrimaryType,
			ReleaseGroupID: groups[i].ID,
			MusicBrainzID:  groups[i].ID,
			ProviderType:   apiexternal_v2.ProviderMusicBrainz,
		}

		// First release date
		if groups[i].FirstReleaseDate != "" {
			result.ReleaseYear = extractYear(groups[i].FirstReleaseDate)
		}

		// Artists
		if len(groups[i].ArtistCredit) > 0 {
			artists := make([]apiexternal_v2.ArtistRef, 0, len(groups[i].ArtistCredit))
			for j := range groups[i].ArtistCredit {
				artists = append(artists, apiexternal_v2.ArtistRef{
					Name: groups[i].ArtistCredit[j].Artist.Name,
					ID:   groups[i].ArtistCredit[j].Artist.ID,
				})
			}

			result.Artists = artists
		}

		results = append(results, result)
	}

	return results
}

func convertReleaseGroupToDetails(group *mbReleaseGroupResponse) *apiexternal_v2.ReleaseDetails {
	details := &apiexternal_v2.ReleaseDetails{
		ID:             group.ID,
		Title:          group.Title,
		Type:           group.PrimaryType,
		SecondaryTypes: group.SecondaryTypes,
		ReleaseGroupID: group.ID,
		MusicBrainzID:  group.ID,
		ProviderType:   apiexternal_v2.ProviderMusicBrainz,
	}

	// First release date
	if group.FirstReleaseDate != "" {
		details.ReleaseDate = parseMBDate(group.FirstReleaseDate)
		details.ReleaseYear = extractYear(group.FirstReleaseDate)
	}

	// Artists
	if len(group.ArtistCredit) > 0 {
		artists := make([]apiexternal_v2.ArtistRef, 0, len(group.ArtistCredit))
		for i := range group.ArtistCredit {
			artists = append(artists, apiexternal_v2.ArtistRef{
				Name: group.ArtistCredit[i].Artist.Name,
				ID:   group.ArtistCredit[i].Artist.ID,
			})
		}

		details.Artists = artists
	}

	// Tags
	if len(group.Tags) > 0 {
		genres := make([]string, 0, len(group.Tags))
		for i := range group.Tags {
			genres = append(genres, group.Tags[i].Name)
		}

		details.Genres = genres
	}

	return details
}

func convertRecordingSearchResults(recordings []mbRecording) []apiexternal_v2.Track {
	results := make([]apiexternal_v2.Track, 0, len(recordings))

	for i := range recordings {
		track := apiexternal_v2.Track{
			ID:            recordings[i].ID,
			Title:         recordings[i].Title,
			Duration:      time.Duration(recordings[i].Length) * time.Millisecond,
			MusicBrainzID: recordings[i].ID,
		}

		// Artists
		if len(recordings[i].ArtistCredit) > 0 {
			artists := make([]apiexternal_v2.ArtistRef, 0, len(recordings[i].ArtistCredit))
			for j := range recordings[i].ArtistCredit {
				artists = append(artists, apiexternal_v2.ArtistRef{
					Name: recordings[i].ArtistCredit[j].Artist.Name,
					ID:   recordings[i].ArtistCredit[j].Artist.ID,
				})
			}

			track.Artists = artists
		}

		// ISRC
		if len(recordings[i].ISRCs) > 0 {
			track.ISRC = recordings[i].ISRCs[0]
		}

		// First release date
		if recordings[i].FirstReleaseDate != "" {
			track.ReleaseYear = extractYear(recordings[i].FirstReleaseDate)
		}

		results = append(results, track)
	}

	return results
}

func convertRecordingToTrack(recording *mbRecordingResponse) *apiexternal_v2.Track {
	track := &apiexternal_v2.Track{
		ID:            recording.ID,
		Title:         recording.Title,
		Duration:      time.Duration(recording.Length) * time.Millisecond,
		MusicBrainzID: recording.ID,
	}

	// Artists
	if len(recording.ArtistCredit) > 0 {
		artists := make([]apiexternal_v2.ArtistRef, 0, len(recording.ArtistCredit))
		for i := range recording.ArtistCredit {
			artists = append(artists, apiexternal_v2.ArtistRef{
				Name: recording.ArtistCredit[i].Artist.Name,
				ID:   recording.ArtistCredit[i].Artist.ID,
			})
		}

		track.Artists = artists
	}

	// ISRCs
	if len(recording.ISRCs) > 0 {
		track.ISRC = recording.ISRCs[0]
	}

	// First release date
	if recording.FirstReleaseDate != "" {
		track.ReleaseYear = extractYear(recording.FirstReleaseDate)
	}

	// Tags
	if len(recording.Tags) > 0 {
		genres := make([]string, 0, len(recording.Tags))
		for i := range recording.Tags {
			genres = append(genres, recording.Tags[i].Name)
		}

		track.Genres = genres
	}

	return track
}

//
// Helper Functions
//

// parseMBDate parses MusicBrainz date formats (YYYY, YYYY-MM, YYYY-MM-DD).
// Layout is selected by string length to avoid trying every format on each call.
func parseMBDate(dateStr string) time.Time {
	var layout string
	switch len(dateStr) {
	case 10:
		layout = "2006-01-02"
	case 7:
		layout = "2006-01"
	case 4:
		layout = "2006"
	default:
		return time.Time{}
	}

	t, err := time.Parse(layout, dateStr)
	if err != nil {
		return time.Time{}
	}

	return t
}

// extractYear extracts the year from a MusicBrainz date string.
func extractYear(dateStr string) int {
	if len(dateStr) >= 4 {
		var year int
		if _, err := time.Parse("2006", dateStr[:4]); err == nil {
			year = parseMBDate(dateStr[:4]).Year()
			return year
		}
	}

	return 0
}

// extractWikidataID extracts the Wikidata ID from a URL.
func extractWikidataID(url string) string {
	// URL format: https://www.wikidata.org/wiki/Q12345
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if strings.HasPrefix(last, "Q") {
			return last
		}
	}

	return ""
}
