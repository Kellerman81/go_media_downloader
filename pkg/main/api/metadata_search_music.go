package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// musicReleaseTypeOptions are the selectable MusicBrainz release types (mode 2/3 filter).
var musicReleaseTypeOptions = []string{
	"Album", "EP", "Single", "Compilation", "Soundtrack", "Live",
}

// musicFormatOptions are the selectable physical/digital media formats (mode 2/3 filter).
// "Digital Media" is MusicBrainz' name for digital/"data" releases.
var musicFormatOptions = []string{
	"CD", "Digital Media", "Vinyl", "Cassette", "SACD", "DVD-Audio",
}

// musicFormatDefaults are the formats checked by default.
var musicFormatDefaults = map[string]bool{"CD": true, "Digital Media": true}

// MusicMetadataSearchPage renders the music metadata search page.
func MusicMetadataSearchPage(c *gin.Context) {
	musicLists := getMediaListsByType("music")
	csrfToken := getCSRFToken(c)

	content := musicMetadataSearchContent(musicLists, csrfToken)

	pageNode := page("Music Metadata Search", false, false, true, content)

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	pageNode.Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// ---------------------------------------------------------------------------
// Page layout
// ---------------------------------------------------------------------------

func musicMetadataSearchContent(mediaConfigs []string, csrfToken string) gomponents.Node {
	albumProviders := musicAlbumSearchProviders()
	artistProviders := musicArtistProviders()

	return html.Div(
		html.Class("config-section-enhanced"),

		// Page Header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(
						html.Class("fas fa-music header-icon"),
						gomponents.Attr("aria-hidden", "true"),
					),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Add Music")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Search configured music metadata providers and add albums, artists or compilations to your library",
						),
					),
				),
			),
		),

		html.Div(
			html.Class("container-fluid"),

			html.Input(html.Type("hidden"), html.ID("music_csrf"), html.Value(csrfToken)),

			// Mode tabs
			html.Ul(
				html.Class("nav nav-tabs mb-3"),
				html.Role("tablist"),
				musicTab("m1", "fas fa-compact-disc", "Single Album", true),
				musicTab("m2", "fas fa-user-plus", "Full Artist", false),
				musicTab("m3", "fas fa-list-check", "Selected Albums", false),
				musicTab("m4", "fas fa-layer-group", "Series / Compilation", false),
			),

			html.Div(
				html.Class("tab-content"),

				// --- Mode 1: Single Album ---
				musicTabPane("m1", true,
					musicCardWrap(
						"Search for a single album",
						"Enter an album title (and optionally an artist), pick a provider, then add any match.",
						html.Div(
							html.Class("row g-3 align-items-end"),
							musicProviderSelectCol("m1_provider", albumProviders),
							musicInputCol(
								"m1_title",
								"Album Title",
								"Enter album title...",
								3,
								true,
							),
							musicInputCol(
								"m1_artist",
								"Artist (optional)",
								"Enter artist...",
								3,
								false,
							),
							musicListSelectCol("m1_list", mediaConfigs, 2),
							musicButtonCol("Search", "fas fa-search", "musicSearchAlbum()"),
						),
					),
					html.Div(html.ID("m1_results"), html.Class("mt-3")),
				),

				// --- Mode 2: Full Artist ---
				musicTabPane("m2", false,
					musicCardWrap(
						"Add a complete artist",
						"Search for an artist, confirm the correct match using the album previews, then add the whole catalogue. Only releases matching the selected types and formats are imported.",
						html.Div(
							html.Class("row g-3 align-items-end"),
							musicProviderSelectCol("m2_provider", artistProviders),
							musicInputCol(
								"m2_artist",
								"Artist Name",
								"Enter artist name...",
								4,
								true,
							),
							musicListSelectCol("m2_list", mediaConfigs, 3),
							musicButtonCol("Search", "fas fa-search", "musicSearchArtist('m2')"),
						),
						musicFilterRow("m2"),
					),
					html.Div(html.ID("m2_results"), html.Class("mt-3")),
				),

				// --- Mode 3: Selected Albums ---
				musicTabPane("m3", false,
					musicCardWrap(
						"Add selected albums of an artist",
						"Search for an artist, list their releases, then pick exactly which albums to add.",
						html.Div(
							html.Class("row g-3 align-items-end"),
							musicProviderSelectCol("m3_provider", artistProviders),
							musicInputCol(
								"m3_artist",
								"Artist Name",
								"Enter artist name...",
								4,
								true,
							),
							musicListSelectCol("m3_list", mediaConfigs, 3),
							musicButtonCol("Search", "fas fa-search", "musicSearchArtist('m3')"),
						),
						musicFilterRow("m3"),
					),
					html.Div(html.ID("m3_results"), html.Class("mt-3")),
				),

				// --- Mode 4: Series / Compilation ---
				musicTabPane(
					"m4",
					false,
					musicCardWrap(
						"Series / Compilation (e.g. Bravo Hits, Now That's What I Call Music)",
						"Search for a compilation/series name to preview its releases, then import all of them at once.",
						html.Div(
							html.Class("row g-3 align-items-end"),
							musicInputCol(
								"series_name",
								"Series / Compilation Name",
								"e.g. Bravo Hits, Now 100",
								5,
								true,
							),
							musicListSelectCol("series_list", mediaConfigs, 4),
							musicButtonCol("Preview", "fas fa-search", "musicSearchSeries()"),
						),
					),
					html.Div(html.ID("m4_results"), html.Class("mt-3")),
				),
			),

			musicSearchScript(),
		),
	)
}

// musicTab renders a single nav-tab button.
func musicTab(key, icon, label string, active bool) gomponents.Node {
	cls := "nav-link"
	if active {
		cls = "nav-link active"
	}

	return html.Li(
		html.Class("nav-item"),
		html.Role("presentation"),
		html.Button(
			html.Class(cls),
			html.ID(key+"-tab"),
			html.Type("button"),
			html.Role("tab"),
			html.Data("bs-toggle", "tab"),
			html.Data("bs-target", "#"+key+"-pane"),
			gomponents.Attr("aria-controls", key+"-pane"),
			html.I(html.Class(icon+" me-1"), gomponents.Attr("aria-hidden", "true")),
			gomponents.Text(label),
		),
	)
}

// musicTabPane wraps tab-pane content.
func musicTabPane(key string, active bool, children ...gomponents.Node) gomponents.Node {
	cls := "tab-pane fade"
	if active {
		cls = "tab-pane fade show active"
	}

	return html.Div(
		append([]gomponents.Node{
			html.Class(cls),
			html.ID(key + "-pane"),
			html.Role("tabpanel"),
			gomponents.Attr("aria-labelledby", key+"-tab"),
		}, children...)...,
	)
}

// musicCardWrap wraps a form in a titled card with a description.
func musicCardWrap(title, desc string, children ...gomponents.Node) gomponents.Node {
	body := []gomponents.Node{
		html.P(html.Class("text-muted small mb-3"), gomponents.Text(desc)),
	}

	body = append(body, children...)

	return html.Div(
		html.Class("card shadow-sm"),
		html.Div(
			html.Class("card-header"),
			html.H5(html.Class("mb-0"), gomponents.Text(title)),
		),
		html.Div(append([]gomponents.Node{html.Class("card-body")}, body...)...),
	)
}

// musicProviderSelectCol renders the provider dropdown column.
func musicProviderSelectCol(id string, providerKeys []string) gomponents.Node {
	opts := make([]gomponents.Node, 0, len(providerKeys)+1)
	if len(providerKeys) == 0 {
		opts = append(
			opts,
			html.Option(html.Value(""), gomponents.Text("(no provider configured)")),
		)
	}

	for _, k := range providerKeys {
		opts = append(opts, html.Option(html.Value(k), gomponents.Text(musicProviderLabel(k))))
	}

	return html.Div(
		html.Class("col-md-2"),
		html.Label(html.For(id), html.Class("form-label"), gomponents.Text("Provider")),
		html.Select(append([]gomponents.Node{
			html.Class("form-select"), html.ID(id), html.Name(id),
		}, opts...)...),
	)
}

// musicInputCol renders a labelled text input column.
func musicInputCol(id, label, placeholder string, mdCols int, required bool) gomponents.Node {
	input := []gomponents.Node{
		html.Type("text"), html.Class("form-control"), html.ID(id), html.Name(id),
		html.Placeholder(placeholder),
	}
	if required {
		input = append(input, gomponents.Attr("required", "true"))
	}

	return html.Div(
		html.Class(fmt.Sprintf("col-md-%d", mdCols)),
		html.Label(html.For(id), html.Class("form-label"), gomponents.Text(label)),
		html.Input(input...),
	)
}

// musicListSelectCol renders the "add to list" dropdown column.
func musicListSelectCol(id string, lists []string, mdCols int) gomponents.Node {
	return html.Div(
		html.Class(fmt.Sprintf("col-md-%d", mdCols)),
		html.Label(html.For(id), html.Class("form-label"), gomponents.Text("Add to List")),
		html.Select(
			html.Class("form-select"), html.ID(id), html.Name(id),
			gomponents.Attr("required", "true"),
			renderSelectOptions(lists, ""),
		),
	)
}

// musicButtonCol renders the search/preview button column.
func musicButtonCol(label, icon, onclick string) gomponents.Node {
	return html.Div(
		html.Class("col-md-2 d-grid"),
		html.Button(
			html.Type("button"),
			html.Class("btn btn-primary"),
			gomponents.Attr("onclick", onclick),
			html.I(html.Class(icon+" me-1"), gomponents.Attr("aria-hidden", "true")),
			gomponents.Text(label),
		),
	)
}

// musicFilterRow renders the release-type and media-format filter checkboxes (modes 2/3).
func musicFilterRow(prefix string) gomponents.Node {
	typeChecks := make([]gomponents.Node, 0, len(musicReleaseTypeOptions))
	for _, t := range musicReleaseTypeOptions {
		typeChecks = append(typeChecks, musicCheck(prefix+"_type_"+t, prefix+"_type", t, t, false))
	}

	formatChecks := make([]gomponents.Node, 0, len(musicFormatOptions))
	for _, f := range musicFormatOptions {
		formatChecks = append(
			formatChecks,
			musicCheck(prefix+"_fmt_"+f, prefix+"_format", f, f, musicFormatDefaults[f]),
		)
	}

	return html.Div(
		html.Class("row g-3 mt-1"),
		html.Div(
			html.Class("col-md-6"),
			html.Label(
				html.Class("form-label fw-semibold d-block"),
				gomponents.Text("Release types"),
			),
			html.Div(html.Class("d-flex flex-wrap gap-3"), gomponents.Group(typeChecks)),
			html.Small(
				html.Class("text-muted"),
				gomponents.Text("Leave all unchecked to allow any type."),
			),
		),
		html.Div(
			html.Class("col-md-6"),
			html.Label(
				html.Class("form-label fw-semibold d-block"),
				gomponents.Text("Media formats"),
			),
			html.Div(html.Class("d-flex flex-wrap gap-3"), gomponents.Group(formatChecks)),
			html.Small(
				html.Class("text-muted"),
				gomponents.Text("Leave all unchecked to allow any format."),
			),
		),
	)
}

// musicCheck renders a single checkbox for the filter row.
func musicCheck(id, name, value, label string, checked bool) gomponents.Node {
	attrs := []gomponents.Node{
		html.Type("checkbox"), html.Class("form-check-input"),
		html.ID(id), html.Name(name), html.Value(value),
	}
	if checked {
		attrs = append(attrs, html.Checked())
	}

	return html.Div(
		html.Class("form-check"),
		html.Input(attrs...),
		html.Label(html.For(id), html.Class("form-check-label"), gomponents.Text(label)),
	)
}

// ---------------------------------------------------------------------------
// Result card rendering
// ---------------------------------------------------------------------------

// releaseJSON marshals a release for embedding in a data attribute. gomponents
// escapes the attribute value, so the JSON round-trips safely to the client.
func releaseJSON(r *apiexternal_v2.ReleaseSearchResult) string {
	b, err := json.Marshal(r)
	if err != nil {
		return "{}"
	}

	return string(b)
}

// releaseBadges renders the type/format/track/country badges for a release.
func releaseBadges(r *apiexternal_v2.ReleaseSearchResult) gomponents.Node {
	badges := make([]gomponents.Node, 0, 5)
	if r.Type != "" {
		badges = append(
			badges,
			html.Span(html.Class("badge bg-primary me-1"), gomponents.Text(r.Type)),
		)
	}

	format := r.Format
	if format == "" && len(r.MediaFormats) > 0 {
		format = strings.Join(r.MediaFormats, "+")
	}

	if format != "" {
		badges = append(
			badges,
			html.Span(html.Class("badge bg-secondary me-1"), gomponents.Text(format)),
		)
	}

	if r.TrackCount > 0 {
		badges = append(
			badges,
			html.Span(
				html.Class("badge bg-info me-1"),
				gomponents.Textf("%d tracks", r.TrackCount),
			),
		)
	}

	if r.Country != "" {
		badges = append(
			badges,
			html.Span(html.Class("badge bg-light text-dark me-1"), gomponents.Text(r.Country)),
		)
	}

	return gomponents.Group(badges)
}

// releaseTitleLine builds "Title (Year)".
func releaseTitleLine(r *apiexternal_v2.ReleaseSearchResult) string {
	if r.ReleaseYear > 0 {
		return fmt.Sprintf("%s (%d)", r.Title, r.ReleaseYear)
	}

	return r.Title
}

// releaseArtistLine joins the release artists.
func releaseArtistLine(r *apiexternal_v2.ReleaseSearchResult) string {
	names := make([]string, 0, len(r.Artists))
	for _, a := range r.Artists {
		names = append(names, a.Name)
	}

	return strings.Join(names, ", ")
}

// createMusicResultCard renders a single album result with an Add button that
// carries the full release as JSON (works for any provider).
func createMusicResultCard(release *apiexternal_v2.ReleaseSearchResult) gomponents.Node {
	artistStr := releaseArtistLine(release)

	return html.Div(
		html.Class("card mb-2"),
		html.Div(
			html.Class("card-body py-2"),
			html.Div(
				html.Class("d-flex justify-content-between align-items-start"),
				html.Div(
					html.Class("flex-grow-1"),
					html.H6(
						html.Class("card-title mb-1"),
						gomponents.Text(releaseTitleLine(release)),
					),
					func() gomponents.Node {
						if artistStr != "" {
							return html.Small(
								html.Class("text-muted d-block mb-1"),
								gomponents.Text("by "+artistStr),
							)
						}

						return nil
					}(),
					html.Div(releaseBadges(release)),
					func() gomponents.Node {
						if release.Label != "" {
							return html.Small(
								html.Class("text-muted d-block"),
								gomponents.Text("Label: "+release.Label),
							)
						}

						return nil
					}(),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-success btn-sm add-album-btn ms-3"),
					html.Data("release", releaseJSON(release)),
					html.I(html.Class("fas fa-plus me-1"), gomponents.Attr("aria-hidden", "true")),
					gomponents.Text("Add Album"),
				),
			),
		),
	)
}

// ---------------------------------------------------------------------------
// Mode 1: single album search + add
// ---------------------------------------------------------------------------

// SearchMusicMetadata handles album search for the selected provider.
func SearchMusicMetadata(c *gin.Context) {
	provider := c.PostForm("m1_provider")
	title := c.PostForm("m1_title")
	artist := c.PostForm("m1_artist")

	if title == "" && artist == "" {
		renderMusicAlert(c, "Please enter an album title or artist", "warning")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := musicSearchAlbums(ctx, provider, artist, title, 25)
	if err != nil {
		logger.Logtype("error", 0).
			Err(err).
			Str("provider", provider).
			Msg("music album search failed")
		renderMusicAlert(c, "Search failed: "+err.Error(), "danger")

		return
	}

	if len(results) == 0 {
		renderMusicAlert(c, "No albums found.", "warning")
		return
	}

	nodes := make([]gomponents.Node, 0, len(results)+1)

	nodes = append(
		nodes,
		html.H6(html.Class("mb-2"), gomponents.Textf("%d albums found", len(results))),
	)

	for i := range results {
		nodes = append(nodes, createMusicResultCard(&results[i]))
	}

	renderMusicHTML(c, html.Div(nodes...))
}

// AddAlbumToDatabase adds an album to the database. It accepts either a full release
// JSON (preferred, provider-agnostic) or a legacy MusicBrainz id with optional series
// discovery.
func AddAlbumToDatabase(c *gin.Context) {
	listName := c.PostForm("music_list")
	if listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "List name is required"})
		return
	}

	cfgp, listid := findMusicCfgpAndListID(listName)
	if cfgp == nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{"error": "List '" + listName + "' not found in music configuration"},
		)

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Preferred path: full release JSON from a provider search.
	if releaseStr := c.PostForm("release"); releaseStr != "" {
		var release apiexternal_v2.ReleaseSearchResult
		if err := json.Unmarshal([]byte(releaseStr), &release); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid release data"})
			return
		}

		if err := importfeed.AddAlbumFromSearchResult(ctx, &release, cfgp, listid); err != nil {
			logger.Logtype("error", 0).
				Err(err).
				Str("album", release.Title).
				Msg("Failed to add album")
			c.JSON(
				http.StatusInternalServerError,
				gin.H{"error": "Failed to add album: " + err.Error()},
			)

			return
		}

		c.JSON(http.StatusOK, gin.H{"success": "Added \"" + release.Title + "\" to " + listName})

		return
	}

	// Legacy path: MusicBrainz id (+ optional series discovery).
	mbid := c.PostForm("mbid")
	if mbid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Release data or MusicBrainz ID is required"})
		return
	}

	if err := importfeed.AddAlbumByMusicBrainzID(ctx, mbid, cfgp, listid); err != nil {
		logger.Logtype("error", 0).Err(err).Str("mbid", mbid).Msg("Failed to add album")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "Failed to add album: " + err.Error()},
		)

		return
	}

	c.JSON(http.StatusOK, gin.H{"success": "Album added successfully to " + listName})
}

// ---------------------------------------------------------------------------
// Modes 2 & 3: artist search (with album previews for disambiguation)
// ---------------------------------------------------------------------------

// SearchMusicArtists searches artists on the selected provider and renders disambiguation
// cards. Each card previews up to 5 of the artist's albums (with years) and exposes the
// action appropriate to the requested mode ("m2" = add whole catalogue, "m3" = pick albums).
func SearchMusicArtists(c *gin.Context) {
	mode := c.PostForm("mode")
	if mode != "m2" && mode != "m3" {
		mode = "m2"
	}

	provider := c.PostForm("provider")
	query := c.PostForm("artist")

	if query == "" {
		renderMusicAlert(c, "Please enter an artist name", "warning")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	artists, err := musicSearchArtists(ctx, provider, query, 5)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Str("provider", provider).Msg("artist search failed")
		renderMusicAlert(c, "Search failed: "+err.Error(), "danger")

		return
	}

	if len(artists) == 0 {
		renderMusicAlert(c, "No artists found for: "+query, "warning")
		return
	}

	nodes := make([]gomponents.Node, 0, len(artists)+1)

	nodes = append(
		nodes,
		html.H6(html.Class("mb-2"), gomponents.Text("Select the correct artist:")),
	)

	for i := range artists {
		nodes = append(nodes, createArtistCard(ctx, provider, mode, &artists[i]))
	}

	renderMusicHTML(c, html.Div(nodes...))
}

// createArtistCard renders an artist with album previews and the mode-specific action.
func createArtistCard(
	ctx context.Context,
	provider, mode string,
	artist *apiexternal_v2.ArtistSearchResult,
) gomponents.Node {
	// Preview a few releases to help disambiguate same-named artists.
	previews, _ := musicArtistReleases(ctx, provider, artist, 5)

	previewNodes := make([]gomponents.Node, 0, len(previews))
	for i := range previews {
		if i >= 5 {
			break
		}

		previewNodes = append(
			previewNodes,
			html.Li(gomponents.Text(releaseTitleLine(&previews[i]))),
		)
	}

	var previewBlock gomponents.Node
	if len(previewNodes) > 0 {
		previewBlock = html.Ul(
			append([]gomponents.Node{html.Class("small text-muted mb-0 mt-1")}, previewNodes...)...)
	} else {
		previewBlock = html.Small(
			html.Class("text-muted fst-italic"),
			gomponents.Text("No album preview available"),
		)
	}

	artistJSON, _ := json.Marshal(artist)

	var meta []string
	if artist.Type != "" {
		meta = append(meta, artist.Type)
	}

	if artist.Country != "" {
		meta = append(meta, artist.Country)
	}

	if artist.BeginYear != 0 {
		meta = append(meta, fmt.Sprintf("est. %d", artist.BeginYear))
	}

	var actionBtn gomponents.Node
	if mode == "m3" {
		actionBtn = html.Button(
			html.Type("button"),
			html.Class("btn btn-sm btn-primary list-artist-albums-btn"),
			html.Data("artist", string(artistJSON)),
			html.I(html.Class("fas fa-list-check me-1"), gomponents.Attr("aria-hidden", "true")),
			gomponents.Text("List Albums"),
		)
	} else {
		actionBtn = html.Button(
			html.Type("button"),
			html.Class("btn btn-sm btn-success add-artist-btn"),
			html.Data("artist", string(artistJSON)),
			html.I(html.Class("fas fa-user-plus me-1"), gomponents.Attr("aria-hidden", "true")),
			gomponents.Text("Add Artist"),
		)
	}

	return html.Div(
		html.Class("card mb-2"),
		html.Div(
			html.Class("card-body py-2"),
			html.Div(
				html.Class("d-flex justify-content-between align-items-start"),
				html.Div(
					html.Class("flex-grow-1"),
					html.Div(
						html.Span(html.Class("fw-semibold"), gomponents.Text(artist.Name)),
						func() gomponents.Node {
							if artist.Disambiguation != "" {
								return html.Span(
									html.Class("text-muted ms-2 small"),
									gomponents.Text("("+artist.Disambiguation+")"),
								)
							}

							return nil
						}(),
						func() gomponents.Node {
							if len(meta) > 0 {
								return html.Div(
									html.Class("text-muted small"),
									gomponents.Text(strings.Join(meta, " · ")),
								)
							}

							return nil
						}(),
					),
					previewBlock,
				),
				html.Div(html.Class("ms-3"), actionBtn),
			),
			// Per-artist container where mode-3 album checklists are injected.
			html.Div(html.Class("mt-2"), html.Data("artist-albums", "1")),
		),
	)
}

// parseFilters reads the checkbox filter values for a mode prefix from the POST form.
func parseFilters(c *gin.Context) (types, formats []string) {
	return c.PostFormArray("types"), c.PostFormArray("formats")
}

// AddArtistAlbumsFiltered (mode 2) queues a background job that discovers an artist's
// releases on the selected provider and adds those matching the type/format filters.
func AddArtistAlbumsFiltered(c *gin.Context) {
	provider := c.PostForm("provider")
	listName := c.PostForm("music_list")
	artistStr := c.PostForm("artist")

	if artistStr == "" || listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Artist and list are required"})
		return
	}

	var artist apiexternal_v2.ArtistSearchResult
	if err := json.Unmarshal([]byte(artistStr), &artist); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid artist data"})
		return
	}

	cfgp, listid := findMusicCfgpAndListID(listName)
	if cfgp == nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{"error": "List '" + listName + "' not found in music configuration"},
		)

		return
	}

	types, formats := parseFilters(c)
	artistCopy := artist

	worker.Dispatch(
		"add_artist_"+artist.Name+"_"+listName,
		func(_ uint32, jobctx context.Context) error {
			releases, err := musicArtistReleases(jobctx, provider, &artistCopy, 200)
			if err != nil {
				logger.Logtype("error", 0).
					Err(err).
					Str("artist", artistCopy.Name).
					Msg("AddArtist: failed to list releases")

				return nil
			}

			added := 0
			for i := range releases {
				if jobctx.Err() != nil {
					break
				}

				if !musicReleaseMatchesFilters(&releases[i], types, formats) {
					continue
				}

				if err := importfeed.AddAlbumFromSearchResult(
					jobctx,
					&releases[i],
					cfgp,
					listid,
				); err == nil {
					added++
				}
			}

			logger.Logtype("info", 0).
				Str("artist", artistCopy.Name).
				Str("list", listName).
				Int("added", added).
				Msg("AddArtist: completed")

			return nil
		},
		"Data",
	)

	c.JSON(http.StatusOK, gin.H{
		"success": fmt.Sprintf(
			"Adding albums by \"%s\" to %s in the background.",
			artist.Name,
			listName,
		),
	})
}

// ListArtistReleasesForSelection (mode 3) lists an artist's releases as a selectable
// checklist filtered by the chosen types/formats.
func ListArtistReleasesForSelection(c *gin.Context) {
	provider := c.PostForm("provider")
	artistStr := c.PostForm("artist")

	if artistStr == "" {
		renderMusicAlert(c, "Missing artist data", "danger")
		return
	}

	var artist apiexternal_v2.ArtistSearchResult
	if err := json.Unmarshal([]byte(artistStr), &artist); err != nil {
		renderMusicAlert(c, "Invalid artist data", "danger")
		return
	}

	types, formats := parseFilters(c)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	releases, err := musicArtistReleases(ctx, provider, &artist, 200)
	if err != nil {
		renderMusicAlert(c, "Failed to list releases: "+err.Error(), "danger")
		return
	}

	items := make([]gomponents.Node, 0, len(releases))
	for i := range releases {
		if !musicReleaseMatchesFilters(&releases[i], types, formats) {
			continue
		}

		items = append(items, musicReleaseCheckItem(&releases[i]))
	}

	if len(items) == 0 {
		renderMusicHTML(c, html.Div(html.Class("alert alert-warning mb-0 mt-2"),
			gomponents.Text("No releases match the selected filters.")))

		return
	}

	header := html.Div(
		html.Class("d-flex justify-content-between align-items-center mb-2"),
		html.Span(html.Class("fw-semibold"), gomponents.Textf("%d releases", len(items))),
		html.Div(
			html.Button(
				html.Type("button"),
				html.Class("btn btn-sm btn-outline-secondary me-2 select-all-btn"),
				gomponents.Text("Select all"),
			),
			html.Button(html.Type("button"), html.Class("btn btn-sm btn-success add-selected-btn"),
				html.I(html.Class("fas fa-plus me-1"), gomponents.Attr("aria-hidden", "true")),
				gomponents.Text("Add Selected")),
		),
	)

	renderMusicHTML(c, html.Div(
		html.Class("border rounded p-2 mt-2 artist-album-list"),
		header,
		html.Div(append([]gomponents.Node{html.Class("d-flex flex-column gap-1")}, items...)...),
	))
}

// musicReleaseCheckItem renders one selectable release row.
func musicReleaseCheckItem(r *apiexternal_v2.ReleaseSearchResult) gomponents.Node {
	return html.Label(
		html.Class("form-check d-flex align-items-center gap-2 mb-0"),
		html.Input(
			html.Type("checkbox"),
			html.Class("form-check-input album-select-check mt-0"),
			html.Data("release", releaseJSON(r)),
		),
		html.Span(
			html.Class("form-check-label"),
			html.Span(html.Class("me-2"), gomponents.Text(releaseTitleLine(r))),
			releaseBadges(r),
		),
	)
}

// AddSelectedAlbums (mode 3) adds the releases selected from an artist's release list.
func AddSelectedAlbums(c *gin.Context) {
	listName := c.PostForm("music_list")
	if listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "List name is required"})
		return
	}

	cfgp, listid := findMusicCfgpAndListID(listName)
	if cfgp == nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{"error": "List '" + listName + "' not found in music configuration"},
		)

		return
	}

	releasesStr := c.PostForm("releases")
	if releasesStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No albums selected"})
		return
	}

	var releases []apiexternal_v2.ReleaseSearchResult
	if err := json.Unmarshal([]byte(releasesStr), &releases); err != nil || len(releases) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid selection data"})
		return
	}

	releasesCopy := releases

	worker.Dispatch(
		"add_selected_"+listName,
		func(_ uint32, jobctx context.Context) error {
			added := 0
			for i := range releasesCopy {
				if jobctx.Err() != nil {
					break
				}

				if err := importfeed.AddAlbumFromSearchResult(
					jobctx,
					&releasesCopy[i],
					cfgp,
					listid,
				); err == nil {
					added++
				}
			}

			logger.Logtype("info", 0).
				Str("list", listName).
				Int("added", added).
				Msg("AddSelectedAlbums: completed")

			return nil
		},
		"Data",
	)

	c.JSON(http.StatusOK, gin.H{
		"success": fmt.Sprintf(
			"Adding %d selected album(s) to %s in the background.",
			len(releases),
			listName,
		),
	})
}

// ---------------------------------------------------------------------------
// Mode 4: series / compilation (unchanged behaviour)
// ---------------------------------------------------------------------------

// SearchMusicSeriesMetadata previews the first releases matching a series name.
func SearchMusicSeriesMetadata(c *gin.Context) {
	seriesName := c.PostForm("series_name")
	listName := c.PostForm("series_list")

	if seriesName == "" {
		renderMusicAlert(c, "Series name is required", "warning")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Series/compilation discovery is MusicBrainz-based.
	results, err := musicSearchAlbums(ctx, provMusicBrainz, "", seriesName, 25)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Str("series", seriesName).Msg("series search failed")
		renderMusicAlert(c, "Search failed: "+err.Error(), "danger")

		return
	}

	if len(results) == 0 {
		renderMusicAlert(c, "No releases found for series: "+seriesName, "warning")
		return
	}

	nodes := []gomponents.Node{
		html.Div(
			html.Class(
				"d-flex justify-content-between align-items-center mb-3 p-3 bg-light rounded",
			),
			html.H6(
				html.Class("mb-0"),
				gomponents.Textf("Preview: %d releases for \"%s\"", len(results), seriesName),
			),
			html.Button(
				html.Class("btn btn-warning import-series-btn"),
				html.Data("series-name", seriesName),
				html.Data("series-list", listName),
				html.I(html.Class("fas fa-download me-1"), gomponents.Attr("aria-hidden", "true")),
				gomponents.Textf("Import All \"%s\"", seriesName),
			),
		),
	}

	for i := range results {
		nodes = append(nodes, createMusicResultCard(&results[i]))
	}

	renderMusicHTML(c, html.Div(nodes...))
}

// DiscoverSeriesAlbumsByName queues a background job importing all releases matching a series name.
func DiscoverSeriesAlbumsByName(c *gin.Context) {
	seriesName := c.PostForm("series_name")
	listName := c.PostForm("series_list")

	if seriesName == "" || listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Series name and list name are required"})
		return
	}

	cfgp, listid := findMusicCfgpAndListID(listName)
	if cfgp == nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{"error": "List '" + listName + "' not found in music configuration"},
		)

		return
	}

	nameCopy := seriesName
	worker.Dispatch(
		"import_series_"+seriesName+"_"+listName,
		func(_ uint32, ctx context.Context) error {
			err := importfeed.JobImportDBAlbum(
				ctx,
				&config.ManualConfig{AlbumSeriesName: nameCopy},
				0,
				cfgp,
				listid,
			)
			logger.Logtype("info", 0).
				Str("series", nameCopy).
				Str("list", listName).
				Err(err).
				Msg("DiscoverSeriesAlbumsByName: completed")

			return nil
		},
		"Data",
	)

	c.JSON(http.StatusOK, gin.H{
		"success": fmt.Sprintf(
			"Importing all \"%s\" albums in background, adding to %s",
			seriesName,
			listName,
		),
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// renderMusicAlert writes a Bootstrap alert HTML fragment.
func renderMusicAlert(c *gin.Context, msg, level string) {
	renderMusicHTML(c, html.Div(html.Class("alert alert-"+level+" mb-0"), gomponents.Text(msg)))
}

// renderMusicHTML renders a node as an HTML fragment response.
func renderMusicHTML(c *gin.Context, node gomponents.Node) {
	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	node.Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// musicSearchScript returns the client-side logic for all four music search modes.
func musicSearchScript() gomponents.Node {
	return html.Script(gomponents.Raw(`
function musicCsrf() {
	var el = document.getElementById('music_csrf');
	return el ? el.value : '';
}

function musicPost(url, params) {
	return fetch(url, {
		method: 'POST',
		headers: { 'Content-Type': 'application/x-www-form-urlencoded', 'X-CSRF-Token': musicCsrf() },
		body: params.toString()
	});
}

function musicSpinner(el, label) {
	el.innerHTML = '<div class="text-center p-4"><div class="spinner-border text-primary" role="status"></div><p class="mt-2 mb-0">' + label + '</p></div>';
}

function musicVal(id) { var e = document.getElementById(id); return e ? e.value : ''; }

function musicGetChecked(name) {
	var out = [];
	document.querySelectorAll('input[name="' + name + '"]:checked').forEach(function(c){ out.push(c.value); });
	return out;
}

// --- Mode 1: single album ---
function musicSearchAlbum() {
	var title = musicVal('m1_title'), artist = musicVal('m1_artist');
	if (!title && !artist) { showToaster('warning', 'Enter an album title or artist'); return; }
	var box = document.getElementById('m1_results');
	musicSpinner(box, 'Searching...');
	var p = new URLSearchParams();
	p.append('m1_provider', musicVal('m1_provider'));
	p.append('m1_title', title);
	p.append('m1_artist', artist);
	musicPost('/api/admin/search/music', p).then(function(r){return r.text();})
		.then(function(html){ box.innerHTML = html; })
		.catch(function(){ box.innerHTML = '<div class="alert alert-danger">Search failed.</div>'; });
}

// --- Modes 2 & 3: artist search ---
function musicSearchArtist(prefix) {
	var artist = musicVal(prefix + '_artist');
	if (!artist) { showToaster('warning', 'Enter an artist name'); return; }
	var box = document.getElementById(prefix + '_results');
	musicSpinner(box, 'Searching artists...');
	var p = new URLSearchParams();
	p.append('mode', prefix);
	p.append('provider', musicVal(prefix + '_provider'));
	p.append('artist', artist);
	musicPost('/api/admin/search/music/artist', p).then(function(r){return r.text();})
		.then(function(html){ box.innerHTML = html; })
		.catch(function(){ box.innerHTML = '<div class="alert alert-danger">Search failed.</div>'; });
}

// --- Mode 4: series ---
function musicSearchSeries() {
	var name = musicVal('series_name');
	if (!name) { showToaster('warning', 'Enter a series name'); return; }
	var box = document.getElementById('m4_results');
	musicSpinner(box, 'Searching...');
	var p = new URLSearchParams();
	p.append('series_name', name);
	p.append('series_list', musicVal('series_list'));
	musicPost('/api/admin/search/music/series', p).then(function(r){return r.text();})
		.then(function(html){ box.innerHTML = html; })
		.catch(function(){ box.innerHTML = '<div class="alert alert-danger">Search failed.</div>'; });
}

// Delegated event handling for dynamically-injected result controls.
document.addEventListener('click', function(e) {
	// Add single album (mode 1 + series cards)
	var addBtn = e.target.closest && e.target.closest('.add-album-btn');
	if (addBtn) {
		var pane = addBtn.closest('.tab-pane');
		var listEl = pane ? pane.querySelector('[id$="_list"]') : null;
		var list = listEl ? listEl.value : '';
		if (!list) { showToaster('warning', 'Select a list first'); return; }
		var original = addBtn.innerHTML;
		addBtn.disabled = true;
		addBtn.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>Adding...';
		var p = new URLSearchParams();
		p.append('release', addBtn.getAttribute('data-release'));
		p.append('music_list', list);
		musicPost('/api/admin/add/album', p).then(function(r){return r.json();}).then(function(d){
			if (d.success) {
				showToaster('success', d.success);
				addBtn.innerHTML = '<i class="fas fa-check me-1"></i>Added';
				addBtn.classList.remove('btn-success'); addBtn.classList.add('btn-outline-success');
			} else {
				showToaster('error', d.error || 'Failed to add album');
				addBtn.innerHTML = original; addBtn.disabled = false;
			}
		}).catch(function(){ showToaster('error', 'Failed to add album'); addBtn.innerHTML = original; addBtn.disabled = false; });
		return;
	}

	// Add whole artist (mode 2)
	var artistBtn = e.target.closest && e.target.closest('.add-artist-btn');
	if (artistBtn) {
		var list2 = musicVal('m2_list');
		if (!list2) { showToaster('warning', 'Select a list first'); return; }
		confirmAction('Add artist', 'Add all matching albums by this artist to "' + list2 + '"? This runs in the background.', function() {
			var p = new URLSearchParams();
			p.append('provider', musicVal('m2_provider'));
			p.append('music_list', list2);
			p.append('artist', artistBtn.getAttribute('data-artist'));
			musicGetChecked('m2_type').forEach(function(t){ p.append('types', t); });
			musicGetChecked('m2_format').forEach(function(f){ p.append('formats', f); });
			artistBtn.disabled = true;
			artistBtn.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>Queuing...';
			musicPost('/api/admin/add/artist-albums', p).then(function(r){return r.json();}).then(function(d){
				if (d.success) { showToaster('success', d.success); artistBtn.innerHTML = '<i class="fas fa-check me-1"></i>Queued'; artistBtn.classList.remove('btn-success'); artistBtn.classList.add('btn-outline-success'); }
				else { showToaster('error', d.error || 'Failed'); artistBtn.disabled = false; artistBtn.innerHTML = 'Add Artist'; }
			}).catch(function(){ showToaster('error', 'Failed to queue'); artistBtn.disabled = false; artistBtn.innerHTML = 'Add Artist'; });
		});
		return;
	}

	// List artist albums (mode 3)
	var listBtn = e.target.closest && e.target.closest('.list-artist-albums-btn');
	if (listBtn) {
		var container = listBtn.closest('.card-body').querySelector('[data-artist-albums]');
		musicSpinner(container, 'Loading albums...');
		var p = new URLSearchParams();
		p.append('provider', musicVal('m3_provider'));
		p.append('artist', listBtn.getAttribute('data-artist'));
		musicGetChecked('m3_type').forEach(function(t){ p.append('types', t); });
		musicGetChecked('m3_format').forEach(function(f){ p.append('formats', f); });
		musicPost('/api/admin/search/music/artist-releases', p).then(function(r){return r.text();})
			.then(function(html){ container.innerHTML = html; })
			.catch(function(){ container.innerHTML = '<div class="alert alert-danger">Failed to load albums.</div>'; });
		return;
	}

	// Select all (mode 3)
	var selAll = e.target.closest && e.target.closest('.select-all-btn');
	if (selAll) {
		var list3 = selAll.closest('.artist-album-list');
		var checks = list3.querySelectorAll('.album-select-check');
		var anyUnchecked = Array.prototype.some.call(checks, function(c){ return !c.checked; });
		checks.forEach(function(c){ c.checked = anyUnchecked; });
		return;
	}

	// Add selected albums (mode 3)
	var addSel = e.target.closest && e.target.closest('.add-selected-btn');
	if (addSel) {
		var list4 = musicVal('m3_list');
		if (!list4) { showToaster('warning', 'Select a list first'); return; }
		var wrap = addSel.closest('.artist-album-list');
		var selected = [];
		wrap.querySelectorAll('.album-select-check:checked').forEach(function(c){
			try { selected.push(JSON.parse(c.getAttribute('data-release'))); } catch(err) {}
		});
		if (selected.length === 0) { showToaster('warning', 'Select at least one album'); return; }
		confirmAction('Add selected', 'Add ' + selected.length + ' selected album(s) to "' + list4 + '"? This runs in the background.', function() {
			var p = new URLSearchParams();
			p.append('music_list', list4);
			p.append('releases', JSON.stringify(selected));
			addSel.disabled = true;
			addSel.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>Queuing...';
			musicPost('/api/admin/add/albums-selected', p).then(function(r){return r.json();}).then(function(d){
				if (d.success) { showToaster('success', d.success); addSel.innerHTML = '<i class="fas fa-check me-1"></i>Queued'; }
				else { showToaster('error', d.error || 'Failed'); addSel.disabled = false; addSel.innerHTML = 'Add Selected'; }
			}).catch(function(){ showToaster('error', 'Failed to queue'); addSel.disabled = false; addSel.innerHTML = 'Add Selected'; });
		});
		return;
	}

	// Import series (mode 4)
	var impBtn = e.target.closest && e.target.closest('.import-series-btn');
	if (impBtn) {
		var sName = impBtn.getAttribute('data-series-name');
		var sList = impBtn.getAttribute('data-series-list') || musicVal('series_list');
		if (!sList) { showToaster('warning', 'Select a list first'); return; }
		confirmAction('Import series', 'Import all "' + sName + '" albums into "' + sList + '"? This runs in the background.', function() {
			var p = new URLSearchParams();
			p.append('series_name', sName);
			p.append('series_list', sList);
			impBtn.disabled = true;
			impBtn.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>Queuing...';
			musicPost('/api/admin/discover/series-albums', p).then(function(r){return r.json();}).then(function(d){
				if (d.success) { showToaster('success', d.success); impBtn.innerHTML = '<i class="fas fa-check me-1"></i>Queued'; impBtn.classList.remove('btn-warning'); impBtn.classList.add('btn-outline-success'); }
				else { showToaster('error', d.error || 'Failed'); impBtn.disabled = false; }
			}).catch(function(){ showToaster('error', 'Failed to queue'); impBtn.disabled = false; });
		});
		return;
	}
});
`))
}

// findMusicCfgpAndListID resolves a list name to its MediaTypeConfig and list index.
func findMusicCfgpAndListID(listName string) (*config.MediaTypeConfig, int) {
	allMedia := config.GetSettingsMediaAll()
	if allMedia == nil {
		return nil, -1
	}

	for i := range allMedia.Music {
		cfgp := config.GetSettingsMedia("music_" + allMedia.Music[i].Name)
		if cfgp == nil {
			continue
		}

		if listid, ok := cfgp.ListsMapIdx[listName]; ok {
			return cfgp, listid
		}
	}

	return nil, -1
}
