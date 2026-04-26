package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// MusicMetadataSearchPage renders the music metadata search page.
func MusicMetadataSearchPage(c *gin.Context) {
	musicLists := getMediaListsByType("music")
	csrfToken := getCSRFToken(c)

	content := musicMetadataSearchContent(musicLists, csrfToken)

	pageNode := page(
		"Music Metadata Search",
		false, // activeConfig
		false, // activeDatabase
		true,  // activeManagement
		content,
	)

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	pageNode.Render(&buf)
	c.String(http.StatusOK, buf.String())
}

func musicMetadataSearchContent(mediaConfigs []string, csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Page Header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-music header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Music Metadata Search")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Search MusicBrainz for albums and add them to your library",
						),
					),
				),
			),
		),

		html.Div(
			html.Class("container-fluid"),

			html.Div(
				html.Class("row g-4 mb-4"),

				// Search Form Card
				html.Div(
					html.Class("col-lg-6"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-search me-2")),
								gomponents.Text("Search Albums"),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.Form(
								html.ID("musicSearchForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("music_title"),
										html.Class("form-label"),
										gomponents.Text("Album Title"),
									),
									html.Input(
										html.Type("text"),
										html.Class("form-control"),
										html.ID("music_title"),
										html.Name("music_title"),
										html.Placeholder("Enter album title to search..."),
									),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("music_artist"),
										html.Class("form-label"),
										gomponents.Text("Artist (optional)"),
									),
									html.Input(
										html.Type("text"),
										html.Class("form-control"),
										html.ID("music_artist"),
										html.Name("music_artist"),
										html.Placeholder("Enter artist name..."),
									),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("music_list"),
										html.Class("form-label"),
										gomponents.Text("Add to List"),
									),
									html.Select(
										html.Class("form-select"),
										html.ID("music_list"),
										html.Name("music_list"),
										gomponents.Attr("required", "true"),
										renderSelectOptions(mediaConfigs, ""),
									),
								),

								html.Div(
									html.Class("d-grid"),
									html.Button(
										html.Type("submit"),
										html.Class("btn btn-primary"),
										html.I(html.Class("fas fa-search me-2")),
										gomponents.Text("Search Albums"),
									),
								),
							),
						),
					),
				),

				// Discover Artist Albums Card
				html.Div(
					html.Class("col-lg-6"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class(
								"card-header d-flex justify-content-between align-items-center",
							),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-layer-group me-2")),
								gomponents.Text("Discover Artist Albums"),
							),
							html.Div(
								html.Class("form-check form-switch mb-0"),
								html.Input(
									html.Type("checkbox"),
									html.Class("form-check-input"),
									html.ID("discoverToggle"),
									gomponents.Attr("role", "switch"),
									gomponents.Attr("onchange", "toggleDiscoverSection()"),
								),
								html.Label(
									html.Class("form-check-label"),
									html.For("discoverToggle"),
									gomponents.Text("Enable"),
								),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.P(
								html.Class("text-muted small mb-3"),
								gomponents.Text(
									"Search MusicBrainz for an artist, pick the correct match, "+
										"then queue discovery of all their albums.",
								),
							),
							// Collapsed section — visible only when toggle is on
							html.Div(
								html.ID("discoverSection"),
								gomponents.Attr("style", "display:none"),

								// List selector (shared by all artist result buttons)
								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("discover_list"),
										html.Class("form-label fw-semibold"),
										gomponents.Text("Add to List *"),
									),
									html.Select(
										html.Class("form-select"),
										html.ID("discover_list"),
										gomponents.Attr("required", "true"),
										renderSelectOptions(mediaConfigs, ""),
									),
								),

								// Artist name search input + button
								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("discover_artist_query"),
										html.Class("form-label fw-semibold"),
										gomponents.Text("Artist Name"),
									),
									html.Div(
										html.Class("input-group"),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("discover_artist_query"),
											html.Placeholder("Enter artist name..."),
											gomponents.Attr(
												"onkeydown",
												"if(event.key==='Enter'){event.preventDefault();searchDiscoverArtist();}",
											),
										),
										html.Button(
											html.Type("button"),
											html.Class("btn btn-outline-secondary"),
											gomponents.Attr("onclick", "searchDiscoverArtist()"),
											html.I(html.Class("fas fa-search")),
										),
									),
								),

								// Artist results injected by JS
								html.Div(html.ID("discoverArtistResults")),
							),
						),
					),
				),
			),

			// Series Search Card (full-width row)
			html.Div(
				html.Class("row g-4 mb-2"),
				html.Div(
					html.Class("col-12"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-list-ol me-2")),
								gomponents.Text(
									"Series Search (e.g. Bravo Hits, Now That's What I Call Music)",
								),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.P(
								html.Class("text-muted small mb-3"),
								gomponents.Text(
									"Search for a compilation/series name to preview its releases, "+
										"then import all of them at once.",
								),
							),
							html.Form(
								html.ID("seriesSearchForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("row g-3 align-items-end"),
									html.Div(
										html.Class("col-md-5"),
										html.Label(
											html.For("series_name"),
											html.Class("form-label"),
											gomponents.Text("Series / Compilation Name *"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("series_name"),
											html.Name("series_name"),
											html.Placeholder("e.g. Bravo Hits, Now 100"),
											gomponents.Attr("required", "true"),
										),
									),
									html.Div(
										html.Class("col-md-4"),
										html.Label(
											html.For("series_list"),
											html.Class("form-label"),
											gomponents.Text("Add to List *"),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("series_list"),
											html.Name("series_list"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(mediaConfigs, ""),
										),
									),
									html.Div(
										html.Class("col-md-3 d-flex gap-2"),
										html.Button(
											html.Type("submit"),
											html.Class("btn btn-primary flex-fill"),
											html.I(html.Class("fas fa-search me-1")),
											gomponents.Text("Preview"),
										),
									),
								),
							),
						),
					),
				),
			),

			// Series results area
			html.Div(
				html.ID("seriesSearchResults"),
				html.Class("mt-2"),
			),

			// Album results area
			html.Div(
				html.ID("musicSearchResults"),
				html.Class("mt-2"),
			),

			musicSearchScript(csrfToken),
		),
	)
}

// SearchMusicMetadata handles AJAX music album search requests via MusicBrainz.
func SearchMusicMetadata(c *gin.Context) {
	title := c.PostForm("music_title")
	artist := c.PostForm("music_artist")

	if title == "" && artist == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title or artist is required"})
		return
	}

	provider := musicbrainz.NewProvider()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	query := title
	if artist != "" && title != "" {
		query = artist + " " + title
	} else if artist != "" {
		query = artist
	}

	results, _, err := provider.SearchReleases(ctx, query, 20, 0)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to search MusicBrainz")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "Failed to search MusicBrainz: " + err.Error()},
		)

		return
	}

	if len(results) == 0 {
		c.Header("Content-Type", "text/html; charset=utf-8")

		var buf strings.Builder
		html.Div(
			html.Class("alert alert-warning text-center"),
			gomponents.Text("No albums found for: "+query),
		).Render(&buf)
		c.String(http.StatusOK, buf.String())

		return
	}

	resultNodes := make([]gomponents.Node, 0, len(results)+1)

	resultNodes = append(resultNodes, html.H5(
		html.Class("mb-3"),
		gomponents.Text(fmt.Sprintf("Search Results (%d albums found)", len(results))),
	))

	for i := range results {
		resultNodes = append(resultNodes, createMusicResultCard(&results[i]))
	}

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	html.Div(resultNodes...).Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// AddAlbumToDatabase adds a specific MusicBrainz release to the database and list.
// If discover_series=1 is posted and the release belongs to a MusicBrainz series,
// a background job is queued to add all other albums in that series.
func AddAlbumToDatabase(c *gin.Context) {
	mbid := c.PostForm("mbid")
	releaseGroupID := c.PostForm("release_group_id")
	albumTitle := c.PostForm("album_title")
	listName := c.PostForm("music_list")
	discoverSeries := c.PostForm("discover_series") == "1"

	if mbid == "" || listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MusicBrainz ID and list name are required"})
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

	if err := importfeed.AddAlbumByMusicBrainzID(ctx, mbid, cfgp, listid); err != nil {
		logger.Logtype("error", 0).Err(err).Str("mbid", mbid).Msg("Failed to add album")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "Failed to add album: " + err.Error()},
		)

		return
	}

	if discoverSeries && releaseGroupID != "" {
		titleCopy := albumTitle
		rgIDCopy := releaseGroupID
		cfgpCopy := cfgp
		listidCopy := listid
		worker.Dispatch(
			"discover_series_"+releaseGroupID+"_"+listName,
			func(_ uint32, ctx context.Context) error {
				added := importfeed.DiscoverAndAddSeriesAlbums(
					ctx,
					rgIDCopy,
					titleCopy,
					cfgpCopy,
					listidCopy,
					nil,
					nil,
				)
				logger.Logtype("info", 0).
					Str("release_group_id", rgIDCopy).
					Str("list", listName).
					Int("added", added).
					Msg("DiscoverAndAddSeriesAlbums: completed")

				return nil
			},
			"Data",
		)
	}

	msg := "Album added successfully to " + listName
	if discoverSeries {
		msg += ". Series discovery queued in background."
	}

	c.JSON(http.StatusOK, gin.H{"success": msg})
}

// DiscoverArtistAlbums queues a background job to discover and add all albums by an artist.
func DiscoverArtistAlbums(c *gin.Context) {
	artist := c.PostForm("discover_artist")
	listName := c.PostForm("discover_list")

	if artist == "" || listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Artist name and list name are required"})
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

	artistCopy := artist
	worker.Dispatch(
		"discover_artist_"+artist+"_"+listName,
		func(_ uint32, ctx context.Context) error {
			added := importfeed.DiscoverAndAddArtistAlbums(
				ctx,
				&artistCopy,
				cfgp,
				listid,
				200,
				nil,
				nil,
			)
			logger.Logtype("info", 0).
				Str("artist", artistCopy).
				Str("list", listName).
				Int("added", added).
				Msg("DiscoverArtistAlbums: completed")

			return nil
		},
		"Data",
	)

	c.JSON(
		http.StatusOK,
		gin.H{
			"success": fmt.Sprintf(
				"Discovering albums for '%s' in background, adding to %s",
				artist,
				listName,
			),
		},
	)
}

// SearchArtistOnMusicBrainz searches MusicBrainz for artists matching the query
// and returns an HTML fragment with selectable artist cards. Each card has a
// "Discover All Albums" button that posts to /admin/discover/artist-albums.
func SearchArtistOnMusicBrainz(c *gin.Context) {
	query := c.PostForm("artist_query")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "artist_query is required"})
		return
	}

	provider := musicbrainz.NewProvider()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	results, err := provider.SearchArtists(ctx, query, 10)
	if err != nil {
		logger.Logtype("error", 0).
			Err(err).
			Str("query", query).
			Msg("Failed to search artists on MusicBrainz")
		c.Header("Content-Type", "text/html; charset=utf-8")

		var buf strings.Builder
		html.Div(
			html.Class("alert alert-danger"),
			gomponents.Text("Failed to search MusicBrainz: "+err.Error()),
		).Render(&buf)
		c.String(http.StatusOK, buf.String())

		return
	}

	if len(results) == 0 {
		c.Header("Content-Type", "text/html; charset=utf-8")

		var buf strings.Builder
		html.Div(
			html.Class("alert alert-warning text-center"),
			gomponents.Text("No artists found for: "+query),
		).Render(&buf)
		c.String(http.StatusOK, buf.String())

		return
	}

	nodes := make([]gomponents.Node, 0, len(results))
	for _, a := range results {
		artist := a // capture

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

		metaText := strings.Join(meta, " · ")

		nodes = append(nodes, html.Div(
			html.Class(
				"d-flex justify-content-between align-items-center border rounded px-3 py-2 mb-2",
			),
			html.Div(
				html.Span(html.Class("fw-semibold"), gomponents.Text(artist.Name)),
				func() gomponents.Node {
					if artist.Disambiguation != "" {
						return html.Span(
							html.Class("text-muted ms-2 small"),
							gomponents.Text("("+artist.Disambiguation+")"),
						)
					}

					return gomponents.Text("")
				}(),
				func() gomponents.Node {
					if metaText != "" {
						return html.Div(html.Class("text-muted small"), gomponents.Text(metaText))
					}

					return gomponents.Text("")
				}(),
			),
			html.Button(
				html.Type("button"),
				html.Class("btn btn-sm btn-secondary discover-artist-btn"),
				gomponents.Attr("data-artist-name", artist.Name),
				html.I(html.Class("fas fa-layer-group me-1")),
				gomponents.Text("Discover"),
			),
		))
	}

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	for _, n := range nodes {
		n.Render(&buf)
	}

	c.String(http.StatusOK, buf.String())
}

// SearchMusicSeriesMetadata previews the first releases matching a series name.
// Returns HTML: an "Import All" action bar followed by individual result cards.
func SearchMusicSeriesMetadata(c *gin.Context) {
	seriesName := c.PostForm("series_name")
	listName := c.PostForm("series_list")

	if seriesName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Series name is required"})
		return
	}

	provider := musicbrainz.NewProvider()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, total, err := provider.SearchReleases(ctx, seriesName, 20, 0)
	if err != nil {
		logger.Logtype("error", 0).
			Err(err).
			Str("series", seriesName).
			Msg("Failed to search series on MusicBrainz")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "Failed to search MusicBrainz: " + err.Error()},
		)

		return
	}

	if len(results) == 0 {
		c.Header("Content-Type", "text/html; charset=utf-8")

		var buf strings.Builder
		html.Div(
			html.Class("alert alert-warning text-center"),
			gomponents.Text("No releases found for series: "+seriesName),
		).Render(&buf)
		c.String(http.StatusOK, buf.String())

		return
	}

	shown := len(results)
	headerText := fmt.Sprintf("Preview: %d of %d releases for \"%s\"", shown, total, seriesName)

	nodes := []gomponents.Node{
		// Action bar with Import All button
		html.Div(
			html.Class(
				"d-flex justify-content-between align-items-center mb-3 p-3 bg-light rounded",
			),
			html.Div(
				html.H5(html.Class("mb-0"), gomponents.Text(headerText)),
				func() gomponents.Node {
					if total > shown {
						return html.Small(
							html.Class("text-muted"),
							gomponents.Text(
								fmt.Sprintf(
									"(showing first %d — import will process all %d)",
									shown,
									total,
								),
							),
						)
					}

					return nil
				}(),
			),
			html.Button(
				html.Class("btn btn-warning import-series-btn"),
				gomponents.Attr("data-series-name", seriesName),
				gomponents.Attr("data-series-list", listName),
				html.I(html.Class("fas fa-download me-1")),
				gomponents.Text(fmt.Sprintf("Import All \"%s\"", seriesName)),
			),
		),
	}

	for i := range results {
		nodes = append(nodes, createMusicResultCard(&results[i]))
	}

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	html.Div(nodes...).Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// DiscoverSeriesAlbumsByName queues a background job that imports all MusicBrainz releases
// matching the given series name.
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

	c.JSON(
		http.StatusOK,
		gin.H{
			"success": fmt.Sprintf(
				"Importing all \"%s\" albums in background, adding to %s",
				seriesName,
				listName,
			),
		},
	)
}

func createMusicResultCard(release *apiexternal_v2.ReleaseSearchResult) gomponents.Node {
	artistNames := make([]string, 0, len(release.Artists))
	for _, a := range release.Artists {
		artistNames = append(artistNames, a.Name)
	}

	artistStr := strings.Join(artistNames, ", ")

	year := ""
	if release.ReleaseYear > 0 {
		year = fmt.Sprintf(" (%d)", release.ReleaseYear)
	}

	// unique id for the checkbox within this card
	checkboxID := "discover_series_" + release.MusicBrainzID

	return html.Div(
		html.Class("card mb-3"),
		html.Div(
			html.Class("card-body"),
			html.Div(
				html.Class("d-flex justify-content-between align-items-start"),
				// Left: metadata
				html.Div(
					html.Class("flex-grow-1"),
					html.H5(
						html.Class("card-title mb-1"),
						gomponents.Text(release.Title+year),
					),
					func() gomponents.Node {
						if artistStr != "" {
							return html.Small(
								html.Class("text-muted d-block mb-2"),
								gomponents.Text("by "+artistStr),
							)
						}

						return nil
					}(),
					html.Div(
						html.Class("mb-2"),
						func() gomponents.Node {
							if release.Type != "" {
								return html.Span(
									html.Class("badge bg-primary me-1"),
									gomponents.Text(release.Type),
								)
							}

							return nil
						}(),
						func() gomponents.Node {
							if release.Format != "" {
								return html.Span(
									html.Class("badge bg-secondary me-1"),
									gomponents.Text(release.Format),
								)
							}

							return nil
						}(),
						func() gomponents.Node {
							if release.TrackCount > 0 {
								return html.Span(
									html.Class("badge bg-info me-1"),
									gomponents.Text(fmt.Sprintf("%d tracks", release.TrackCount)),
								)
							}

							return nil
						}(),
						func() gomponents.Node {
							if release.Country != "" {
								return html.Span(
									html.Class("badge bg-light text-dark me-1"),
									gomponents.Text(release.Country),
								)
							}

							return nil
						}(),
					),
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
				// Right: actions
				html.Div(
					html.Class("d-flex flex-column gap-2 ms-3 align-items-end"),
					// Discover series checkbox
					html.Div(
						html.Class("form-check"),
						html.Input(
							html.Type("checkbox"),
							html.Class("form-check-input discover-series-check"),
							html.ID(checkboxID),
							gomponents.Attr("data-mbid", release.MusicBrainzID),
						),
						html.Label(
							html.For(checkboxID),
							html.Class("form-check-label small text-nowrap"),
							gomponents.Text("Discover series"),
						),
					),
					// Add Album button
					html.Button(
						html.Class("btn btn-success add-album-btn"),
						gomponents.Attr("data-mbid", release.MusicBrainzID),
						gomponents.Attr("data-release-group-id", release.ReleaseGroupID),
						gomponents.Attr("data-title", release.Title),
						gomponents.Attr("data-checkbox-id", checkboxID),
						html.I(html.Class("fas fa-plus me-1")),
						gomponents.Text("Add Album"),
					),
				),
			),
		),
	)
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

func musicSearchScript(csrfToken string) gomponents.Node {
	return html.Script(gomponents.Raw(`
document.getElementById('musicSearchForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const title = document.getElementById('music_title').value;
	const artist = document.getElementById('music_artist').value;
	const list = document.getElementById('music_list').value;

	if (!title && !artist) {
		alert('Please enter a title or artist');
		return;
	}

	if (!list) {
		alert('Please select a list');
		return;
	}

	const resultsDiv = document.getElementById('musicSearchResults');
	resultsDiv.innerHTML = '<div class="text-center p-4"><div class="spinner-border text-primary" role="status"><span class="visually-hidden">Loading...</span></div><p class="mt-2">Searching MusicBrainz...</p></div>';

	fetch('/api/admin/search/music', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'music_title=' + encodeURIComponent(title) + '&music_artist=' + encodeURIComponent(artist)
	})
	.then(response => response.text())
	.then(data => {
		resultsDiv.innerHTML = data;
		attachAddAlbumHandlers();
	})
	.catch(error => {
		console.error('Error:', error);
		resultsDiv.innerHTML = '<div class="alert alert-danger">Error searching for albums. Please try again.</div>';
	});
});

function attachAddAlbumHandlers() {
	document.querySelectorAll('.add-album-btn').forEach(btn => {
		btn.addEventListener('click', function() {
			const mbid = this.getAttribute('data-mbid');
			const releaseGroupID = this.getAttribute('data-release-group-id') || '';
			const albumTitle = this.getAttribute('data-title');
			const checkboxID = this.getAttribute('data-checkbox-id');
			const selectedList = document.getElementById('music_list').value;

			if (!selectedList) {
				alert('Please select a list');
				return;
			}

			if (!mbid) {
				alert('No MusicBrainz ID available for this release');
				return;
			}

			const discoverSeries = checkboxID && document.getElementById(checkboxID) && document.getElementById(checkboxID).checked;

			if (confirm('Add "' + albumTitle + '" to list "' + selectedList + '"?' + (discoverSeries ? '\nAlso discover all albums in this series.' : ''))) {
				addAlbumToDatabase(mbid, releaseGroupID, albumTitle, selectedList, discoverSeries, this);
			}
		});
	});
}

function addAlbumToDatabase(mbid, releaseGroupID, albumTitle, listName, discoverSeries, button) {
	const originalText = button.innerHTML;
	button.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>Adding...';
	button.disabled = true;

	let body = 'mbid=' + encodeURIComponent(mbid) +
		'&release_group_id=' + encodeURIComponent(releaseGroupID) +
		'&album_title=' + encodeURIComponent(albumTitle) +
		'&music_list=' + encodeURIComponent(listName);
	if (discoverSeries) {
		body += '&discover_series=1';
	}

	fetch('/api/admin/add/album', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: body
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			button.innerHTML = '<i class="fas fa-check me-1"></i>Added!';
			button.classList.remove('btn-success');
			button.classList.add('btn-outline-success');
			setTimeout(() => {
				button.innerHTML = originalText;
				button.disabled = false;
				button.classList.add('btn-success');
				button.classList.remove('btn-outline-success');
			}, 2000);
		} else {
			alert('Error: ' + (data.error || 'Failed to add album'));
			button.innerHTML = originalText;
			button.disabled = false;
		}
	})
	.catch(error => {
		console.error('Error:', error);
		alert('Error adding album to database');
		button.innerHTML = originalText;
		button.disabled = false;
	});
}

document.getElementById('seriesSearchForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const seriesName = document.getElementById('series_name').value;
	const list = document.getElementById('series_list').value;

	if (!seriesName) {
		alert('Please enter a series name');
		return;
	}

	if (!list) {
		alert('Please select a list');
		return;
	}

	const resultsDiv = document.getElementById('seriesSearchResults');
	resultsDiv.innerHTML = '<div class="text-center p-4"><div class="spinner-border text-warning" role="status"><span class="visually-hidden">Loading...</span></div><p class="mt-2">Searching series on MusicBrainz...</p></div>';

	fetch('/api/admin/search/music/series', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'series_name=' + encodeURIComponent(seriesName) + '&series_list=' + encodeURIComponent(list)
	})
	.then(response => response.text())
	.then(data => {
		resultsDiv.innerHTML = data;
		attachAddAlbumHandlers();
		attachImportSeriesHandlers();
	})
	.catch(error => {
		console.error('Error:', error);
		resultsDiv.innerHTML = '<div class="alert alert-danger">Error searching for series. Please try again.</div>';
	});
});

function attachImportSeriesHandlers() {
	document.querySelectorAll('.import-series-btn').forEach(btn => {
		btn.addEventListener('click', function() {
			const seriesName = this.getAttribute('data-series-name');
			const list = this.getAttribute('data-series-list') || document.getElementById('series_list').value;

			if (!list) {
				alert('Please select a list');
				return;
			}

			if (!confirm('Import all "' + seriesName + '" albums into "' + list + '"?\nThis runs in the background and may take a while.')) {
				return;
			}

			const originalText = this.innerHTML;
			this.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>Queuing...';
			this.disabled = true;

			const btn = this;
			fetch('/api/admin/discover/series-albums', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/x-www-form-urlencoded',
					'X-CSRF-Token': '` + csrfToken + `',
				},
				body: 'series_name=' + encodeURIComponent(seriesName) + '&series_list=' + encodeURIComponent(list)
			})
			.then(response => response.json())
			.then(data => {
				if (data.success) {
					alert(data.success);
					btn.innerHTML = '<i class="fas fa-check me-1"></i>Queued!';
					btn.classList.remove('btn-warning');
					btn.classList.add('btn-outline-success');
				} else {
					alert('Error: ' + (data.error || 'Failed to queue import'));
					btn.innerHTML = originalText;
					btn.disabled = false;
				}
			})
			.catch(error => {
				console.error('Error:', error);
				alert('Error queuing series import');
				btn.innerHTML = originalText;
				btn.disabled = false;
			});
		});
	});
}

function toggleDiscoverSection() {
	const enabled = document.getElementById('discoverToggle').checked;
	document.getElementById('discoverSection').style.display = enabled ? '' : 'none';
	if (!enabled) {
		document.getElementById('discoverArtistResults').innerHTML = '';
	}
}

function searchDiscoverArtist() {
	const query = document.getElementById('discover_artist_query').value.trim();
	if (!query) {
		alert('Please enter an artist name');
		return;
	}

	const resultsDiv = document.getElementById('discoverArtistResults');
	resultsDiv.innerHTML = '<div class="text-center p-3"><div class="spinner-border spinner-border-sm text-secondary" role="status"></div><span class="ms-2 text-muted">Searching MusicBrainz...</span></div>';

	fetch('/api/admin/search/music/artist', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'artist_query=' + encodeURIComponent(query)
	})
	.then(response => response.text())
	.then(data => {
		resultsDiv.innerHTML = data;
		attachDiscoverArtistHandlers();
	})
	.catch(error => {
		console.error('Error:', error);
		resultsDiv.innerHTML = '<div class="alert alert-danger">Error searching for artists. Please try again.</div>';
	});
}

function attachDiscoverArtistHandlers() {
	document.querySelectorAll('.discover-artist-btn').forEach(btn => {
		btn.addEventListener('click', function() {
			const artistName = this.getAttribute('data-artist-name');
			const list = document.getElementById('discover_list').value;

			if (!list) {
				alert('Please select a list first');
				return;
			}

			if (!confirm('Discover all albums by "' + artistName + '" and add to "' + list + '"?\nThis runs in the background.')) {
				return;
			}

			const originalText = this.innerHTML;
			this.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>Queuing...';
			this.disabled = true;
			const btn = this;

			fetch('/api/admin/discover/artist-albums', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/x-www-form-urlencoded',
					'X-CSRF-Token': '` + csrfToken + `',
				},
				body: 'discover_artist=' + encodeURIComponent(artistName) + '&discover_list=' + encodeURIComponent(list)
			})
			.then(response => response.json())
			.then(data => {
				if (data.success) {
					btn.innerHTML = '<i class="fas fa-check me-1"></i>Queued!';
					btn.classList.remove('btn-secondary');
					btn.classList.add('btn-outline-success');
				} else {
					alert('Error: ' + (data.error || 'Failed to queue discovery'));
					btn.innerHTML = originalText;
					btn.disabled = false;
				}
			})
			.catch(error => {
				console.error('Error:', error);
				alert('Error queuing artist discovery');
				btn.innerHTML = originalText;
				btn.disabled = false;
			});
		});
	});
}

// legacy discoverForm handler kept for backward compat — card no longer emits this form
if (document.getElementById('discoverForm')) {
document.getElementById('discoverForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const artist = document.getElementById('discover_artist').value;
	const list = document.getElementById('discover_list').value;

	if (!artist || !list) {
		alert('Please enter an artist name and select a list');
		return;
	}

	if (!confirm('Discover all albums by "' + artist + '" and add to "' + list + '"?\nThis runs in the background.')) {
		return;
	}

	const submitBtn = this.querySelector('button[type="submit"]');
	const originalText = submitBtn.innerHTML;
	submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin me-2"></i>Queuing...';
	submitBtn.disabled = true;

	fetch('/api/admin/discover/artist-albums', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'discover_artist=' + encodeURIComponent(artist) + '&discover_list=' + encodeURIComponent(list)
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			alert(data.success);
		} else {
			alert('Error: ' + (data.error || 'Failed to queue discovery'));
		}
		submitBtn.innerHTML = originalText;
		submitBtn.disabled = false;
	})
	.catch(error => {
		console.error('Error:', error);
		alert('Error queuing artist discovery');
		submitBtn.innerHTML = originalText;
		submitBtn.disabled = false;
	});
});
}
`))
}
