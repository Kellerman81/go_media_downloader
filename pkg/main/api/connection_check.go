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
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/apprise"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/gotify"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/newznab"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/pushbullet"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/pushover"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/sendmail"
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

// parseServerURL extracts host, port, and SSL from a server URL string, e.g.
// "https://gotify.example.com:8080" -> ("gotify.example.com", 8080, true).
// Mirrors main.go's parseServerURL (package main, unreachable from here) -
// see the doc comment there for the same logic's production usage building
// these same provider types from NotificationConfig.
func parseServerURL(serverURL string) (host string, port int, useSSL bool) {
	port = 80

	if strings.HasPrefix(serverURL, "https://") {
		useSSL = true
		port = 443
		serverURL = strings.TrimPrefix(serverURL, "https://")
	} else if after, ok := strings.CutPrefix(serverURL, "http://"); ok {
		serverURL = after
	}

	if strings.Contains(serverURL, ":") {
		parts := strings.Split(serverURL, ":")

		host = parts[0]
		if portNum, err := strconv.Atoi(parts[1]); err == nil {
			port = portNum
		}
	} else {
		host = serverURL
	}

	return host, port, useSSL
}

// connectionTestClientConfig builds a base.ClientConfig for a one-off
// connection test - generous limits since it's a single call, matching the
// shape main.go's initproviders uses for the same provider types in
// production (see pushoverConfig/gotifyConfig there).
func connectionTestClientConfig(name string) base.ClientConfig {
	return base.ClientConfig{
		Name:                      "test_" + name,
		Timeout:                   wizardTestTimeout,
		AuthType:                  base.AuthNone,
		RateLimitCalls:            300,
		RateLimitSeconds:          3600,
		CircuitBreakerThreshold:   3,
		CircuitBreakerTimeout:     30 * time.Second,
		CircuitBreakerHalfOpenMax: 1,
	}
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

// HandleTestNotificationConnection live-tests a notification provider's
// credentials via each provider's lightweight TestConnection round-trip
// (validate-credentials/version endpoints, never a real notification), or
// for CSV, that the output folder is writable.
func HandleTestNotificationConnection(c *gin.Context) {
	c.Header("Content-Type", "text/html")

	notifType := c.PostForm("notificationtype")
	apikey := strings.TrimSpace(c.PostForm("apikey"))
	serverURL := strings.TrimSpace(c.PostForm("serverurl"))
	appriseURLs := strings.TrimSpace(c.PostForm("appriseurls"))

	ctx, cancel := context.WithTimeout(c.Request.Context(), wizardTestTimeout)
	defer cancel()

	var err error

	switch notifType {
	case "pushover":
		if apikey == "" {
			c.String(http.StatusOK, renderAlert("Enter an API token first.", "warning"))
			return
		}

		p := pushover.NewProviderWithConfig(
			connectionTestClientConfig("pushover"),
			apikey,
			c.PostForm("recipient"),
		)
		if p == nil {
			c.String(http.StatusOK, renderAlert("Invalid Pushover configuration.", "danger"))
			return
		}

		err = p.TestConnection(ctx)

	case "gotify":
		if serverURL == "" || apikey == "" {
			c.String(http.StatusOK, renderAlert("Enter a server URL and token first.", "warning"))
			return
		}

		host, port, useSSL := parseServerURL(serverURL)

		p := gotify.NewProviderWithConfig(
			connectionTestClientConfig("gotify"),
			host,
			port,
			apikey,
			useSSL,
		)
		if p == nil {
			c.String(http.StatusOK, renderAlert("Invalid Gotify configuration.", "danger"))
			return
		}

		err = p.TestConnection(ctx)

	case "pushbullet":
		if apikey == "" {
			c.String(http.StatusOK, renderAlert("Enter an API token first.", "warning"))
			return
		}

		p := pushbullet.NewProvider(apikey)
		if p == nil {
			c.String(http.StatusOK, renderAlert("Invalid Pushbullet configuration.", "danger"))
			return
		}

		err = p.TestConnection(ctx)

	case "apprise":
		if serverURL == "" || appriseURLs == "" {
			c.String(
				http.StatusOK,
				renderAlert("Enter a server URL and at least one Apprise URL first.", "warning"),
			)

			return
		}

		host, port, useSSL := parseServerURL(serverURL)

		p := apprise.NewProvider(host, port, apikey, strings.Split(appriseURLs, ","), useSSL)
		if p == nil {
			c.String(http.StatusOK, renderAlert("Invalid Apprise configuration.", "danger"))
			return
		}

		err = p.TestConnection(ctx)

	case "sendmail":
		smtpServer := strings.TrimSpace(c.PostForm("smtpserver"))
		fromEmail := strings.TrimSpace(c.PostForm("smtpfromemail"))
		toEmail := strings.TrimSpace(c.PostForm("smtptoemail"))

		if smtpServer == "" || fromEmail == "" || toEmail == "" {
			c.String(
				http.StatusOK,
				renderAlert("Enter an SMTP server, from address, and to address first.", "warning"),
			)

			return
		}

		port, _ := strconv.Atoi(strings.TrimSpace(c.PostForm("smtpport")))
		if port == 0 {
			port = 587
		}

		p := sendmail.NewProvider(
			smtpServer,
			port,
			fromEmail,
			[]string{toEmail},
			c.PostForm("smtpusername"),
			c.PostForm("smtppassword"),
		)
		if p == nil {
			c.String(http.StatusOK, renderAlert("Invalid sendmail configuration.", "danger"))
			return
		}

		err = p.TestConnection(ctx)

	case "csv":
		outputto := strings.TrimSpace(c.PostForm("outputto"))
		if outputto == "" {
			c.String(http.StatusOK, renderAlert("Enter an output file path first.", "warning"))
			return
		}

		dir := filepath.Dir(outputto)
		if !checkWritable(dir) {
			c.String(
				http.StatusOK,
				renderAlert(
					"Cannot write to "+dir+" - check the folder exists and is writable.",
					"danger",
				),
			)

			return
		}

		c.String(http.StatusOK, renderAlert("Output folder is writable.", "success"))

		return

	default:
		c.String(http.StatusOK, renderAlert("Choose a notification type first.", "warning"))
		return
	}

	if err != nil {
		c.String(http.StatusOK, renderAlert("Connection failed: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Connection succeeded.", "success"))
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

		limit := "1"
		if _, err := apiexternal.GetTraktUserList(username, listname, c.PostForm("traktlisttype"), &limit); err != nil {
			c.String(http.StatusOK, renderAlert("Connection failed: "+err.Error(), "danger"))
			return
		}

		c.String(http.StatusOK, renderAlert("Connection succeeded.", "success"))

	case strings.HasPrefix(listType, "traktmovie"), strings.HasPrefix(listType, "traktserie"):
		if _, _, err := apiexternal.TestTraktConnectivity(wizardTestTimeout, nil); err != nil {
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

		if _, err := apiexternal.GetPlexWatchlist(serverURL, token, strings.TrimSpace(c.PostForm("plexusername"))); err != nil {
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

		if _, err := apiexternal.GetJellyfinWatchlist(serverURL, token, strings.TrimSpace(c.PostForm("jellyfinusername"))); err != nil {
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
