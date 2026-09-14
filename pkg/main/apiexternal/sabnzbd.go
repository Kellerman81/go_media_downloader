package apiexternal

import (
	"context"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// SendToSabnzbd sends a download URL to a Sabnzbd server.
// It takes the Sabnzbd server address, API key, download URL, category, NZB name,
// and priority as parameters.
// It returns any error from creating the Sabnzbd client, authenticating,
// validating the authentication method, or adding the NZB.
func SendToSabnzbd(
	downloaderName, server, apikey, urlv, category, nzbname string,
	priority int,
) error {
	// Try v2 provider first
	provider := providers.GetSABnzbd(downloaderName)
	if provider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := provider.AddNZB(ctx, urlv, category, priority)

		// The "legacy client" this used to fall through to on error is long
		// gone (commented out below) - returning nil regardless of err made
		// every add-NZB failure look like a successful download to every
		// caller, silently recording history/notifications for a download
		// that was never actually queued.
		return err
	}
	// s, err := sabnzbd.New(sabnzbd.Addr(server), sabnzbd.ApikeyAuth(apikey))
	// if err != nil {
	// 	return err
	// }

	// auth, err := s.Auth()
	// if err != nil {
	// 	return err
	// }

	// if auth != "apikey" {
	// 	return errors.New("sabnzbd instance must be using apikey authentication")
	// }
	// _, err = s.AddURL(
	// 	sabnzbd.AddNzbURL(urlv),
	// 	sabnzbd.AddNzbName(nzbname),
	// 	sabnzbd.AddNzbCategory(category),
	// 	sabnzbd.AddNzbPriority(priority),
	// )
	// if err != nil {
	// 	return err
	// }
	return errNoClient
}
