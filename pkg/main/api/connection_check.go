// Package api - connection_test.go adds live "Test Connection" buttons to the
// Downloader, Indexer, Notification, and List Settings screens, so a user can
// check credentials/connectivity for a row without saving first.
//
// Each Settings page can have many rows (one per configured downloader,
// indexer, ...), each with its own dynamically-named fields (array-indexed
// by the row's current Name, e.g. "downloader_MyDownloader_Hostname"), all
// sitting inside the one big page-wide config <form>. Reading the current
// row's values reuses the same scoping applyGmdTemplate already established
// in provider_templates.go: scope to the button's closest .array-item-enhanced
// row card (falling back to the enclosing form for the wizard's flat forms),
// then match each wanted field by name suffix. That sidesteps two htmx
// pitfalls already hit once each while building the path checker: htmx's
// default full-form inclusion means hx-include/hx-params can't cleanly
// isolate "just this row", and htmx's own "previous"/"closest" selector
// extensions only work inside attributes like hx-include, not the plain
// hx-vals "js:" JS the button below uses to read each field itself.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/newznab"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/movielists"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// connTestField maps a fixed lowercase POST key to the struct field name
// suffix (e.g. "Hostname") that should be read from the current row.
type connTestField struct {
	Key    string
	Suffix string
}

// renderConnectionTestControl renders a "Test Connection" button that reads
// the listed fields from the current row (see package doc) and posts them
// under fixed keys to endpoint, plus an empty result span for the response.
func renderConnectionTestControl(
	csrfToken, endpoint string,
	fields []connTestField,
) gomponents.Node {
	var b strings.Builder

	b.WriteString("js:{")

	for i, f := range fields {
		if i > 0 {
			b.WriteString(",")
		}

		suffixJSON, _ := json.Marshal(f.Suffix)
		fmt.Fprintf(
			&b,
			`%s: ((this.closest('.array-item-enhanced')||this.closest('form')||document).querySelector('[name$="_' + %s + '"]')||{}).value || ""`,
			f.Key,
			string(suffixJSON),
		)
	}

	b.WriteString("}")

	return html.Div(
		html.Class("d-flex align-items-center gap-2 mt-1"),
		html.Button(
			html.Class("btn btn-sm btn-outline-primary"),
			html.Type("button"),
			html.I(html.Class("fa-solid fa-plug me-1")),
			gomponents.Text("Test Connection"),
			hx.Target("next span"),
			hx.Swap("innerHTML"),
			hx.Post(endpoint),
			hx.Vals(b.String()),
			hx.Headers(wizardCSRFHeader(csrfToken)),
		),
		html.Span(),
	)
}

// HandleTestIndexerConnection live-tests an indexer's URL/API key by
// fetching its capabilities - works for both Newznab and Torznab since the
// provider auto-detects the protocol from the URL.
func HandleTestIndexerConnection(c *gin.Context) {
	c.Header("Content-Type", "text/html")

	url := strings.TrimSpace(c.PostForm("url"))
	if url == "" {
		c.String(http.StatusOK, renderAlert("Enter a URL first.", "warning"))
		return
	}

	provider := newznab.NewProvider(newznab.ProviderConfig{
		IndexerName: "connection-test",
		BaseURL:     url,
		APIKey:      strings.TrimSpace(c.PostForm("apikey")),
		CustomAPI:   strings.TrimSpace(c.PostForm("customapi")),
		CustomURL:   strings.TrimSpace(c.PostForm("customurl")),
		Enabled:     true,
	})

	ctx, cancel := context.WithTimeout(c.Request.Context(), wizardTestTimeout)
	defer cancel()

	if err := provider.TestConnection(ctx); err != nil {
		c.String(http.StatusOK, renderAlert("Connection failed: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Connection succeeded.", "success"))
}

// HandleTestDownloaderConnection live-tests a download client's
// hostname/port/credentials. Shares its client-construction logic with the
// wizard's equivalent test (wizardTestDownloaderConnection in
// setup_wizard.go) - only where the posted values come from differs.
func HandleTestDownloaderConnection(c *gin.Context) {
	c.Header("Content-Type", "text/html")

	dlType := c.PostForm("dltype")
	hostname := strings.TrimSpace(c.PostForm("hostname"))
	port, _ := strconv.Atoi(c.PostForm("port"))
	username := c.PostForm("username")
	password := c.PostForm("password")

	if dlType == "" || dlType == "drone" {
		c.String(http.StatusOK, renderAlert("This client type has nothing to test.", "warning"))
		return
	}

	if hostname == "" {
		c.String(http.StatusOK, renderAlert("Enter a hostname first.", "warning"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), wizardTestTimeout)
	defer cancel()

	if err := wizardTestDownloaderConnection(ctx, dlType, hostname, port, username, password); err != nil {
		c.String(http.StatusOK, renderAlert("Connection failed: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Connection succeeded.", "success"))
}

// HandleTestNotificationConnection live-tests a notification entry from the
// posted row values with its provider's TestConnection (a credential or
// health check; services without one, such as a generic webhook, get a test
// message), or for CSV, that the output folder is writable.
func HandleTestNotificationConnection(c *gin.Context) {
	c.Header("Content-Type", "text/html")

	field := func(key string) string { return strings.TrimSpace(c.PostForm(key)) }

	cfg := &config.NotificationConfig{
		Name:             "connection-test",
		NotificationType: field("notificationtype"),
		Apikey:           field("apikey"),
		Recipient:        field("recipient"),
		Sender:           field("sender"),
		Command:          field("command"),
		Outputto:         field("outputto"),
		ServerURL:        field("serverurl"),
		AppriseURLs:      field("appriseurls"),
		SMTPServer:       field("smtpserver"),
		SMTPPort:         field("smtpport"),
		SMTPFromEmail:    field("smtpfromemail"),
		SMTPToEmail:      field("smtptoemail"),
		SMTPUsername:     c.PostForm("smtpusername"),
		SMTPPassword:     c.PostForm("smtppassword"),
	}

	if cfg.NotificationType == "" {
		c.String(http.StatusOK, renderAlert("Choose a notification type first.", "warning"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), wizardTestTimeout)
	defer cancel()

	err := testNotificationConfig(ctx, cfg)

	switch {
	case errors.Is(err, providers.ErrNotificationField):
		c.String(http.StatusOK, renderAlert("Fill in the required setting first ("+err.Error()+").", "warning"))
	case err != nil:
		c.String(http.StatusOK, renderAlert("Connection failed: "+err.Error(), "danger"))
	case cfg.NotificationType == "csv":
		c.String(http.StatusOK, renderAlert("Output folder is writable.", "success"))
	default:
		c.String(http.StatusOK, renderAlert("Connection succeeded.", "success"))
	}
}

// testNotificationConfig runs the connection check of cfg's type: the
// provider's TestConnection, or for csv that the output folder is writable.
func testNotificationConfig(ctx context.Context, cfg *config.NotificationConfig) error {
	if cfg.NotificationType == "csv" {
		if cfg.Outputto == "" {
			return fmt.Errorf("output_to: %w", providers.ErrNotificationField)
		}

		if dir := filepath.Dir(cfg.Outputto); !checkWritable(dir) {
			return fmt.Errorf("cannot write to %s - check the folder exists and is writable", dir)
		}

		return nil
	}

	p, err := providers.NewNotificationProvider(cfg)
	if err != nil {
		return err
	}

	return p.TestConnection(ctx)
}

// HandleTestListConnection live-tests a list's credentials/reachability by
// reusing the same production functions Feeds() calls to actually fetch each
// list type, with a minimal request (e.g. limit=1) where possible instead of
// a full fetch. List types with no network component (local files) get an
// existence/access check instead; types with no per-list credentials and no
// lightweight check available report that explicitly rather than guessing.
func HandleTestListConnection(c *gin.Context) {
	c.Header("Content-Type", "text/html")

	listType := c.PostForm("listtype")

	switch {
	case listType == "":
		c.String(http.StatusOK, renderAlert("Choose a list type first.", "warning"))

	case strings.HasPrefix(listType, "traktpublic"):
		username := strings.TrimSpace(c.PostForm("traktusername"))
		listname := strings.TrimSpace(c.PostForm("traktlistname"))

		if username == "" || listname == "" {
			c.String(
				http.StatusOK,
				renderAlert("Enter a Trakt username and list name first.", "warning"),
			)

			return
		}

		if _, err := traktUserList(c.Request.Context(), username, listname, c.PostForm("traktlisttype")); err != nil {
			c.String(http.StatusOK, renderAlert("Connection failed: "+err.Error(), "danger"))
			return
		}

		c.String(http.StatusOK, renderAlert("Connection succeeded.", "success"))

	case strings.HasPrefix(listType, "traktmovie"), strings.HasPrefix(listType, "traktserie"):
		if _, err := traktTrendingMovies(wizardTestTimeout, 1); err != nil {
			c.String(http.StatusOK, renderAlert("Connection failed: "+err.Error(), "danger"))
			return
		}

		c.String(http.StatusOK, renderAlert("Connection succeeded.", "success"))

	case listType == "plexwatchlist":
		serverURL := strings.TrimSpace(c.PostForm("plexserverurl"))
		token := strings.TrimSpace(c.PostForm("plextoken"))

		if serverURL == "" || token == "" {
			c.String(
				http.StatusOK,
				renderAlert("Enter a Plex server URL and token first.", "warning"),
			)

			return
		}

		plexp := providers.GetPlex()
		if plexp == nil {
			c.String(http.StatusOK, renderAlert("Plex client not initialized.", "danger"))
			return
		}

		if _, err := plexp.GetWatchlist(c.Request.Context(), serverURL, token); err != nil {
			c.String(http.StatusOK, renderAlert("Connection failed: "+err.Error(), "danger"))
			return
		}

		c.String(http.StatusOK, renderAlert("Connection succeeded.", "success"))

	case listType == "jellyfinwatchlist":
		serverURL := strings.TrimSpace(c.PostForm("jellyfinserverurl"))
		token := strings.TrimSpace(c.PostForm("jellyfintoken"))

		if serverURL == "" || token == "" {
			c.String(
				http.StatusOK,
				renderAlert("Enter a Jellyfin server URL and API key first.", "warning"),
			)

			return
		}

		jellyfinp := providers.GetJellyfin()
		if jellyfinp == nil {
			c.String(http.StatusOK, renderAlert("Jellyfin client not initialized.", "danger"))
			return
		}

		if _, err := jellyfinp.GetWatchlist(c.Request.Context(), serverURL, token, strings.TrimSpace(c.PostForm("jellyfinusername"))); err != nil {
			c.String(http.StatusOK, renderAlert("Connection failed: "+err.Error(), "danger"))
			return
		}

		c.String(http.StatusOK, renderAlert("Connection succeeded.", "success"))

	case listType == "imdbcsv":
		path := strings.TrimSpace(c.PostForm("imdbcsvfile"))
		if path == "" {
			c.String(http.StatusOK, renderAlert("Enter a CSV file path first.", "warning"))
			return
		}

		if _, err := os.Stat(path); err != nil {
			c.String(http.StatusOK, renderAlert("Cannot access file: "+err.Error(), "danger"))
			return
		}

		c.String(http.StatusOK, renderAlert("File exists and is accessible.", "success"))

	case listType == "rottentomatoes", listType == "letterboxd":
		listURL := strings.TrimSpace(c.PostForm("url"))
		if listURL == "" {
			c.String(http.StatusOK, renderAlert("Enter the list URL first.", "warning"))
			return
		}

		client := movielists.NewClient(config.GetSettingsGeneral().UserAgent)

		var (
			movies []movielists.Movie
			err    error
		)

		if listType == "rottentomatoes" {
			movies, err = client.RottenTomatoes(c.Request.Context(), listURL)
		} else {
			movies, err = client.Letterboxd(c.Request.Context(), listURL, 1)
		}

		if err != nil {
			c.String(http.StatusOK, renderAlert("Reading the list failed: "+err.Error(), "danger"))
			return
		}

		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Found %d movies on the first page, e.g. %s (%d).",
			len(movies), movies[0].Title, movies[0].Year), "success"))

	default:
		c.String(
			http.StatusOK,
			renderAlert(
				"No connectivity check available for this list type - save and check the logs after a manual scan instead.",
				"info",
			),
		)
	}
}
