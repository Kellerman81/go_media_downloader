package discogs

import (
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// Discogs Internal Types - Used for JSON unmarshaling
//

// discogsSearchResponse represents the search API response.
type discogsSearchResponse struct {
	Pagination discogsPagination     `json:"pagination"`
	Results    []discogsSearchResult `json:"results"`
}

// discogsPagination represents pagination info.
type discogsPagination struct {
	Page    int `json:"page"`
	Pages   int `json:"pages"`
	PerPage int `json:"per_page"`
	Items   int `json:"items"`
}

// discogsSearchResult represents a search result item.
type discogsSearchResult struct {
	ID          int      `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Thumb       string   `json:"thumb"`
	CoverImage  string   `json:"cover_image"`
	ResourceURL string   `json:"resource_url"`
	URI         string   `json:"uri"`
	MasterID    int      `json:"master_id"`
	MasterURL   string   `json:"master_url"`
	Country     string   `json:"country"`
	Year        string   `json:"year"`
	Format      []string `json:"format"`
	Label       []string `json:"label"`
	Genre       []string `json:"genre"`
	Style       []string `json:"style"`
	Barcode     []string `json:"barcode"`
	CatNo       string   `json:"catno"`
}

// discogsArtistResponse represents an artist.
type discogsArtistResponse struct {
	ID             int            `json:"id"`
	Name           string         `json:"name"`
	RealName       string         `json:"realname"`
	Profile        string         `json:"profile"`
	DataQuality    string         `json:"data_quality"`
	ResourceURL    string         `json:"resource_url"`
	URI            string         `json:"uri"`
	ReleasesURL    string         `json:"releases_url"`
	Images         []discogsImage `json:"images"`
	URLs           []string       `json:"urls"`
	NameVariations []string       `json:"namevariations"`
	Aliases        []discogsRef   `json:"aliases"`
	Members        []discogsRef   `json:"members"`
	Groups         []discogsRef   `json:"groups"`
}

// discogsArtistReleasesResponse represents artist releases.
type discogsArtistReleasesResponse struct {
	Pagination discogsPagination      `json:"pagination"`
	Releases   []discogsArtistRelease `json:"releases"`
}

// discogsArtistRelease represents a release in artist's discography.
type discogsArtistRelease struct {
	ID          int    `json:"id"`
	Type        string `json:"type"` // "master" or "release"
	MainRelease int    `json:"main_release"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Role        string `json:"role"`
	Year        int    `json:"year"`
	Thumb       string `json:"thumb"`
	ResourceURL string `json:"resource_url"`
	Format      string `json:"format"`
	Label       string `json:"label"`
	Status      string `json:"status"`
}

// discogsReleaseResponse represents a release.
type discogsReleaseResponse struct {
	ID                int                   `json:"id"`
	Title             string                `json:"title"`
	Status            string                `json:"status"`
	Year              int                   `json:"year"`
	ResourceURL       string                `json:"resource_url"`
	URI               string                `json:"uri"`
	MasterID          int                   `json:"master_id"`
	MasterURL         string                `json:"master_url"`
	Country           string                `json:"country"`
	Released          string                `json:"released"`
	ReleasedFormatted string                `json:"released_formatted"`
	Notes             string                `json:"notes"`
	DataQuality       string                `json:"data_quality"`
	Artists           []discogsArtistCredit `json:"artists"`
	ExtraArtists      []discogsArtistCredit `json:"extraartists"`
	Labels            []discogsLabelCredit  `json:"labels"`
	Companies         []discogsCompany      `json:"companies"`
	Formats           []discogsFormat       `json:"formats"`
	Genres            []string              `json:"genres"`
	Styles            []string              `json:"styles"`
	Tracklist         []discogsTrack        `json:"tracklist"`
	Identifiers       []discogsIdentifier   `json:"identifiers"`
	Videos            []discogsVideo        `json:"videos"`
	Images            []discogsImage        `json:"images"`
	Community         discogsCommunity      `json:"community"`
}

// discogsMasterResponse represents a master release.
type discogsMasterResponse struct {
	ID             int                   `json:"id"`
	Title          string                `json:"title"`
	MainRelease    int                   `json:"main_release"`
	MainReleaseURL string                `json:"main_release_url"`
	VersionsURL    string                `json:"versions_url"`
	ResourceURL    string                `json:"resource_url"`
	URI            string                `json:"uri"`
	Year           int                   `json:"year"`
	DataQuality    string                `json:"data_quality"`
	Artists        []discogsArtistCredit `json:"artists"`
	Genres         []string              `json:"genres"`
	Styles         []string              `json:"styles"`
	Tracklist      []discogsTrack        `json:"tracklist"`
	Videos         []discogsVideo        `json:"videos"`
	Images         []discogsImage        `json:"images"`
	NumForSale     int                   `json:"num_for_sale"`
	LowestPrice    float64               `json:"lowest_price"`
}

// discogsMasterVersionsResponse represents master release versions.
type discogsMasterVersionsResponse struct {
	Pagination discogsPagination      `json:"pagination"`
	Versions   []discogsMasterVersion `json:"versions"`
}

// discogsMasterVersion represents a version of a master release.
type discogsMasterVersion struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Country     string `json:"country"`
	Year        string `json:"year"`
	Format      string `json:"format"`
	Label       string `json:"label"`
	CatNo       string `json:"catno"`
	Thumb       string `json:"thumb"`
	ResourceURL string `json:"resource_url"`
	Status      string `json:"status"`
	Released    string `json:"released"`
}

// discogsLabelResponse represents a record label.
type discogsLabelResponse struct {
	ID          int            `json:"id"`
	Name        string         `json:"name"`
	Profile     string         `json:"profile"`
	ContactInfo string         `json:"contact_info"`
	ParentLabel discogsRef     `json:"parent_label"`
	Sublabels   []discogsRef   `json:"sublabels"`
	URLs        []string       `json:"urls"`
	Images      []discogsImage `json:"images"`
	DataQuality string         `json:"data_quality"`
	ResourceURL string         `json:"resource_url"`
	URI         string         `json:"uri"`
	ReleasesURL string         `json:"releases_url"`
}

// discogsLabelReleasesResponse represents label releases.
type discogsLabelReleasesResponse struct {
	Pagination discogsPagination     `json:"pagination"`
	Releases   []discogsLabelRelease `json:"releases"`
}

// discogsLabelRelease represents a release on a label.
type discogsLabelRelease struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Year        int    `json:"year"`
	CatNo       string `json:"catno"`
	Format      string `json:"format"`
	Thumb       string `json:"thumb"`
	ResourceURL string `json:"resource_url"`
	Status      string `json:"status"`
}

// Helper types

type discogsRef struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ResourceURL string `json:"resource_url"`
}

type discogsImage struct {
	Type        string `json:"type"`
	URI         string `json:"uri"`
	ResourceURL string `json:"resource_url"`
	URI150      string `json:"uri150"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

type discogsArtistCredit struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ANV         string `json:"anv"`
	Join        string `json:"join"`
	Role        string `json:"role"`
	Tracks      string `json:"tracks"`
	ResourceURL string `json:"resource_url"`
}

type discogsLabelCredit struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	CatNo       string `json:"catno"`
	EntityType  string `json:"entity_type"`
	ResourceURL string `json:"resource_url"`
}

type discogsCompany struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	CatNo          string `json:"catno"`
	EntityType     string `json:"entity_type"`
	EntityTypeName string `json:"entity_type_name"`
	ResourceURL    string `json:"resource_url"`
}

type discogsFormat struct {
	Name         string   `json:"name"`
	Qty          string   `json:"qty"`
	Text         string   `json:"text"`
	Descriptions []string `json:"descriptions"`
}

type discogsTrack struct {
	Position     string                `json:"position"`
	Type         string                `json:"type_"`
	Title        string                `json:"title"`
	Duration     string                `json:"duration"`
	Artists      []discogsArtistCredit `json:"artists"`
	ExtraArtists []discogsArtistCredit `json:"extraartists"`
}

type discogsIdentifier struct {
	Type        string `json:"type"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

type discogsVideo struct {
	URI         string `json:"uri"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Duration    int    `json:"duration"`
	Embed       bool   `json:"embed"`
}

type discogsCommunity struct {
	Have        int              `json:"have"`
	Want        int              `json:"want"`
	Rating      discogsRating    `json:"rating"`
	Submitter   discogsSubmitter `json:"submitter"`
	DataQuality string           `json:"data_quality"`
	Status      string           `json:"status"`
}

type discogsRating struct {
	Count   int     `json:"count"`
	Average float64 `json:"average"`
}

type discogsSubmitter struct {
	Username    string `json:"username"`
	ResourceURL string `json:"resource_url"`
}

//
// Conversion Functions
//

func convertSearchResultsToReleases(
	results []discogsSearchResult,
) []apiexternal_v2.ReleaseSearchResult {
	releases := make([]apiexternal_v2.ReleaseSearchResult, 0, len(results))

	for _, r := range results {
		if r.Type != "release" && r.Type != "master" {
			continue
		}

		// Discogs search returns titles as "Artist - Release Title".
		// Split on the first " - " to populate Artists and clean up the title.
		title := r.Title
		var artists []apiexternal_v2.ArtistRef
		if idx := strings.Index(title, " - "); idx >= 0 {
			artists = []apiexternal_v2.ArtistRef{{Name: title[:idx]}}
			title = title[idx+3:]
		}

		result := apiexternal_v2.ReleaseSearchResult{
			ID:           strconv.Itoa(r.ID),
			Title:        title,
			Artists:      artists,
			Country:      r.Country,
			DiscogsID:    r.ID,
			ProviderType: apiexternal_v2.ProviderDiscogs,
		}

		// Year
		if r.Year != "" {
			result.ReleaseYear, _ = strconv.Atoi(r.Year)
		}

		// Master ID
		if r.MasterID > 0 {
			result.MasterID = r.MasterID
		}

		// Labels
		if len(r.Label) > 0 {
			result.Label = r.Label[0]
		}

		// Catalog number
		if r.CatNo != "" {
			result.CatalogNumber = r.CatNo
		}

		// Format
		if len(r.Format) > 0 {
			result.Format = logger.JoinStringsSep(r.Format, ", ")
		}

		// Genres
		if len(r.Genre) > 0 {
			result.Genres = r.Genre
		}

		// Cover
		if r.CoverImage != "" {
			result.CoverURL = r.CoverImage
		} else if r.Thumb != "" {
			result.CoverURL = r.Thumb
		}

		// Barcode
		if len(r.Barcode) > 0 {
			result.Barcode = r.Barcode[0]
		}

		releases = append(releases, result)
	}

	return releases
}

func convertSearchResultsToArtists(
	results []discogsSearchResult,
) []apiexternal_v2.ArtistSearchResult {
	artists := make([]apiexternal_v2.ArtistSearchResult, 0, len(results))

	for _, r := range results {
		if r.Type != "artist" {
			continue
		}

		result := apiexternal_v2.ArtistSearchResult{
			ID:           strconv.Itoa(r.ID),
			Name:         r.Title,
			DiscogsID:    r.ID,
			ProviderType: apiexternal_v2.ProviderDiscogs,
		}

		// Image
		if r.CoverImage != "" {
			result.ImageURL = r.CoverImage
		} else if r.Thumb != "" {
			result.ImageURL = r.Thumb
		}

		artists = append(artists, result)
	}

	return artists
}

func convertArtistToDetails(artist *discogsArtistResponse) *apiexternal_v2.ArtistDetails {
	details := &apiexternal_v2.ArtistDetails{
		ID:           strconv.Itoa(artist.ID),
		Name:         artist.Name,
		RealName:     artist.RealName,
		Bio:          artist.Profile,
		DiscogsID:    strconv.Itoa(artist.ID),
		ProviderType: apiexternal_v2.ProviderDiscogs,
	}

	// Image
	for _, img := range artist.Images {
		if img.Type == "primary" {
			details.ImageURL = img.URI
			break
		}
	}

	if details.ImageURL == "" && len(artist.Images) > 0 {
		details.ImageURL = artist.Images[0].URI
	}

	// Aliases
	if len(artist.NameVariations) > 0 {
		details.Aliases = artist.NameVariations
	}

	// Website (first URL)
	if len(artist.URLs) > 0 {
		details.Website = artist.URLs[0]
	}

	// Members (for groups)
	if len(artist.Members) > 0 {
		members := make([]string, 0, len(artist.Members))
		for _, m := range artist.Members {
			members = append(members, m.Name)
		}

		details.Members = members
	}

	// Groups (for solo artists)
	if len(artist.Groups) > 0 {
		groups := make([]string, 0, len(artist.Groups))
		for _, g := range artist.Groups {
			groups = append(groups, g.Name)
		}

		details.Groups = groups
	}

	return details
}

func convertArtistReleasesToResults(
	releases []discogsArtistRelease,
) []apiexternal_v2.ReleaseSearchResult {
	results := make([]apiexternal_v2.ReleaseSearchResult, 0, len(releases))

	for _, r := range releases {
		result := apiexternal_v2.ReleaseSearchResult{
			ID:           strconv.Itoa(r.ID),
			Title:        r.Title,
			ReleaseYear:  r.Year,
			Label:        r.Label,
			Format:       r.Format,
			DiscogsID:    r.ID,
			ProviderType: apiexternal_v2.ProviderDiscogs,
		}

		if r.Type == "master" {
			result.MasterID = r.ID
		}

		if r.Thumb != "" {
			result.CoverURL = r.Thumb
		}

		results = append(results, result)
	}

	return results
}

func convertReleaseToDetails(release *discogsReleaseResponse) *apiexternal_v2.ReleaseDetails {
	details := &apiexternal_v2.ReleaseDetails{
		ID:           strconv.Itoa(release.ID),
		Title:        release.Title,
		Status:       release.Status,
		Country:      release.Country,
		ReleaseYear:  release.Year,
		Notes:        release.Notes,
		MasterID:     release.MasterID,
		DiscogsID:    strconv.Itoa(release.ID),
		ProviderType: apiexternal_v2.ProviderDiscogs,
	}

	// Release date
	if release.Released != "" {
		details.ReleaseDate = parseDiscogsDate(release.Released)
	}

	// Artists
	if len(release.Artists) > 0 {
		artists := make([]apiexternal_v2.ArtistRef, 0, len(release.Artists))
		for _, a := range release.Artists {
			artists = append(artists, apiexternal_v2.ArtistRef{
				Name: a.Name,
				ID:   strconv.Itoa(a.ID),
			})
		}

		details.Artists = artists
	}

	// Labels
	if len(release.Labels) > 0 {
		details.Label = release.Labels[0].Name
		details.LabelID = strconv.Itoa(release.Labels[0].ID)
		details.CatalogNumber = release.Labels[0].CatNo
	}

	// Formats
	if len(release.Formats) > 0 {
		formats := make([]string, 0, len(release.Formats))
		for _, f := range release.Formats {
			formats = append(formats, f.Name)
		}

		details.Format = logger.JoinStringsSep(formats, ", ")
		details.DiscCount = countDiscs(release.Formats)
	}

	// Genres and styles
	details.Genres = release.Genres
	if len(release.Styles) > 0 {
		details.Styles = release.Styles
	}

	// Tracklist
	if len(release.Tracklist) > 0 {
		tracks := make([]apiexternal_v2.Track, 0, len(release.Tracklist))
		seq := 0 // 1-based sequential position across all tracks
		for _, t := range release.Tracklist {
			if t.Type != "track" && t.Type != "" {
				continue
			}
			seq++

			disc, trackNum := parseDiscTrack(t.Position)
			if disc == 0 {
				disc = 1
			}
			if trackNum == 0 {
				trackNum = seq
			}

			track := apiexternal_v2.Track{
				Title:       t.Title,
				Position:    seq,
				TrackNumber: trackNum,
				DiscNumber:  disc,
				Duration:    parseDuration(t.Duration),
			}
			// Track artists
			if len(t.Artists) > 0 {
				trackArtists := make([]apiexternal_v2.ArtistRef, 0, len(t.Artists))
				for _, a := range t.Artists {
					trackArtists = append(trackArtists, apiexternal_v2.ArtistRef{
						Name: a.Name,
						ID:   strconv.Itoa(a.ID),
					})
				}

				track.Artists = trackArtists
			}

			tracks = append(tracks, track)
		}

		details.Tracks = tracks
		details.TrackCount = len(tracks)
	}

	// Identifiers (barcode, etc.)
	for _, id := range release.Identifiers {
		switch id.Type {
		case "Barcode":
			if details.Barcode == "" {
				details.Barcode = id.Value
			}
		}
	}

	// Cover image
	for _, img := range release.Images {
		if img.Type == "primary" {
			details.CoverURL = img.URI
			break
		}
	}

	if details.CoverURL == "" && len(release.Images) > 0 {
		details.CoverURL = release.Images[0].URI
	}

	// Community rating
	if release.Community.Rating.Count > 0 {
		details.Rating = release.Community.Rating.Average
		details.RatingCount = release.Community.Rating.Count
	}

	return details
}

func convertMasterToDetails(master *discogsMasterResponse) *apiexternal_v2.ReleaseDetails {
	details := &apiexternal_v2.ReleaseDetails{
		ID:            strconv.Itoa(master.ID),
		Title:         master.Title,
		ReleaseYear:   master.Year,
		MasterID:      master.ID,
		MainReleaseID: master.MainRelease,
		DiscogsID:     strconv.Itoa(master.ID),
		ProviderType:  apiexternal_v2.ProviderDiscogs,
	}

	// Artists
	if len(master.Artists) > 0 {
		artists := make([]apiexternal_v2.ArtistRef, 0, len(master.Artists))
		for _, a := range master.Artists {
			artists = append(artists, apiexternal_v2.ArtistRef{
				Name: a.Name,
				ID:   strconv.Itoa(a.ID),
			})
		}

		details.Artists = artists
	}

	// Genres and styles
	details.Genres = master.Genres
	details.Styles = master.Styles

	// Tracklist
	if len(master.Tracklist) > 0 {
		tracks := make([]apiexternal_v2.Track, 0, len(master.Tracklist))
		for seq, t := range master.Tracklist {
			disc, trackNum := parseDiscTrack(t.Position)
			if disc == 0 {
				disc = 1
			}
			if trackNum == 0 {
				trackNum = seq + 1
			}

			track := apiexternal_v2.Track{
				Title:       t.Title,
				Position:    seq + 1,
				TrackNumber: trackNum,
				DiscNumber:  disc,
				Duration:    parseDuration(t.Duration),
			}

			tracks = append(tracks, track)
		}

		details.Tracks = tracks
		details.TrackCount = len(tracks)
	}

	// Cover image
	for _, img := range master.Images {
		if img.Type == "primary" {
			details.CoverURL = img.URI
			break
		}
	}

	if details.CoverURL == "" && len(master.Images) > 0 {
		details.CoverURL = master.Images[0].URI
	}

	return details
}

func convertVersionsToResults(
	versions []discogsMasterVersion,
) []apiexternal_v2.ReleaseSearchResult {
	results := make([]apiexternal_v2.ReleaseSearchResult, 0, len(versions))

	for _, v := range versions {
		year, _ := strconv.Atoi(v.Year)
		result := apiexternal_v2.ReleaseSearchResult{
			ID:            strconv.Itoa(v.ID),
			Title:         v.Title,
			Country:       v.Country,
			ReleaseYear:   year,
			Format:        v.Format,
			Label:         v.Label,
			CatalogNumber: v.CatNo,
			DiscogsID:     v.ID,
			ProviderType:  apiexternal_v2.ProviderDiscogs,
		}

		if v.Thumb != "" {
			result.CoverURL = v.Thumb
		}

		results = append(results, result)
	}

	return results
}

func convertLabelReleasesToResults(
	releases []discogsLabelRelease,
) []apiexternal_v2.ReleaseSearchResult {
	results := make([]apiexternal_v2.ReleaseSearchResult, 0, len(releases))

	for _, r := range releases {
		result := apiexternal_v2.ReleaseSearchResult{
			ID:            strconv.Itoa(r.ID),
			Title:         r.Title,
			ReleaseYear:   r.Year,
			CatalogNumber: r.CatNo,
			Format:        r.Format,
			DiscogsID:     r.ID,
			ProviderType:  apiexternal_v2.ProviderDiscogs,
		}

		if r.Artist != "" {
			result.Artists = []apiexternal_v2.ArtistRef{{Name: r.Artist}}
		}

		if r.Thumb != "" {
			result.CoverURL = r.Thumb
		}

		results = append(results, result)
	}

	return results
}

//
// Helper Functions
//

// parseDiscogsDate parses Discogs date format (YYYY-MM-DD or variations).
func parseDiscogsDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	layouts := []string{
		"2006-01-02",
		"2006-01",
		"2006",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t
		}
	}

	return time.Time{}
}

// parseDuration parses Discogs duration format (M:SS or MM:SS).
func parseDuration(durStr string) time.Duration {
	if durStr == "" {
		return 0
	}

	parts := strings.Split(durStr, ":")
	if len(parts) != 2 {
		return 0
	}

	minutes, err1 := strconv.Atoi(parts[0])

	seconds, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0
	}

	return time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
}

// parseTrackPosition extracts a flat sequence number from a Discogs position
// string for use as a sort key.  It is intentionally simple — callers that
// need per-disc breakdown should use parseDiscTrack instead.
func parseTrackPosition(pos string, defaultPos int) int {
	disc, track := parseDiscTrack(pos)
	if disc == 0 && track == 0 {
		return defaultPos
	}
	if track == 0 {
		return disc // single-number position
	}
	return track // flat track-on-disc; disc info is in TrackNumber/DiscNumber
}

// parseDiscTrack parses Discogs position strings into (disc, track) pairs.
//
// Supported formats:
//
//	"5"     → (0, 5)   single track number, no disc
//	"1-3"   → (1, 3)   disc 1, track 3
//	"1.3"   → (1, 3)   alternate separator
//	"A1"    → (0, 1)   vinyl side letter + track (disc ignored)
//	"A-1"   → (0, 1)
//
// Returns (0, 0) when no numeric content is found.
func parseDiscTrack(pos string) (disc, track int) {
	if pos == "" {
		return 0, 0
	}

	// Split on the first '-' or '.' that separates disc from track.
	// Only treat the separator as a disc-track boundary when both sides
	// contain at least one digit (avoids mis-parsing "A-1" as disc "A").
	sepIdx := -1
	for i, r := range pos {
		if r == '-' || r == '.' {
			sepIdx = i
			break
		}
	}

	parseDigits := func(s string) int {
		var n int
		for _, r := range s {
			if r >= '0' && r <= '9' {
				n = n*10 + int(r-'0')
			}
		}
		return n
	}

	if sepIdx > 0 && sepIdx < len(pos)-1 {
		left := pos[:sepIdx]
		right := pos[sepIdx+1:]
		leftNum := parseDigits(left)
		rightNum := parseDigits(right)
		if leftNum > 0 && rightNum > 0 {
			return leftNum, rightNum
		}
	}

	// No valid separator — treat the whole thing as a flat track number.
	return 0, parseDigits(pos)
}

// countDiscs counts the number of discs from formats.
func countDiscs(formats []discogsFormat) int {
	total := 0
	for _, f := range formats {
		qty, err := strconv.Atoi(f.Qty)
		if err == nil && qty > 0 {
			total += qty
		} else {
			total++
		}
	}

	if total == 0 {
		return 1
	}

	return total
}
