package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

/*
HTTPProvider implements a simple HTTP file download utility.
This is NOT a DownloadProvider (like qBittorrent/Deluge).
It's a utility for direct file downloads without a download client.
*/
type HTTPProvider struct {
	*base.BaseClient
}

/*
NewProvider creates a new HTTP download provider instance.

Parameters:
  - config: Base client configuration for the HTTP provider

Returns:
  - *HTTPProvider: Configured HTTP provider instance
  - error: Any error encountered during provider creation

Example:

	config := base.ClientConfig{
	    Name:    "http-downloader",
	    Timeout: 30 * time.Second,
	}
	provider, err := http.NewProvider(config)
*/
func NewProvider(config base.ClientConfig) (*HTTPProvider, error) {
	baseClient := base.NewBaseClient(config)

	return &HTTPProvider{
		BaseClient: baseClient,
	}, nil
}

/*
DownloadFile downloads a file from the specified URL to the target path.

Parameters:
  - ctx: Context for request cancellation and timeout
  - url: Source URL to download from
  - targetPath: Directory path where the file should be saved
  - filename: Name for the downloaded file

Returns:
  - error: Any error encountered during download

Example:

	err := provider.DownloadFile(ctx, "https://example.com/file.nzb", "/downloads", "movie.nzb")
*/
func (p *HTTPProvider) DownloadFile(ctx context.Context, url, targetPath, filename string) error {
	fullPath := filepath.Join(targetPath, filename)

	// Use BaseClient's MakeRequest with a custom response handler for file writing
	return p.MakeRequest(
		ctx,
		http.MethodGet,
		url,
		nil,
		nil,
		func(resp *http.Response) error {
			// Create output file
			outFile, err := os.Create(fullPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer outFile.Close()

			// Copy response body to file
			_, err = io.Copy(outFile, resp.Body)
			if err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}

			return nil
		},
	)
}

/*
GetProviderName returns the provider name for HTTP downloads.
*/
func (p *HTTPProvider) GetProviderName() string {
	return "HTTP"
}

/*
TestConnection tests HTTP connectivity.
Always succeeds as HTTP doesn't require persistent connections.
*/
func (p *HTTPProvider) TestConnection(ctx context.Context) error {
	return nil
}
