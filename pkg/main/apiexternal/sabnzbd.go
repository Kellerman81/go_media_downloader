package apiexternal

import (
	"errors"

	"github.com/Kellerman81/go_media_downloader/pkg/main/sabnzbd"
)

// SendToSabnzbd sends a download URL to a Sabnzbd server.
// It takes the Sabnzbd server address, API key, download URL, category, NZB name,
// and priority as parameters.
// It returns any error from creating the Sabnzbd client, authenticating,
// validating the authentication method, or adding the NZB.
func SendToSabnzbd(server, apikey, urlv, category, nzbname string, priority int) error {
	s, err := sabnzbd.New(sabnzbd.Addr(server), sabnzbd.ApikeyAuth(apikey))
	if err != nil {
		return err
	}

	auth, err := s.Auth()
	if err != nil {
		return err
	}

	if auth != "apikey" {
		return errors.New("sabnzbd instance must be using apikey authentication")
	}
	_, err = s.AddURL(sabnzbd.AddNzbURL(urlv), sabnzbd.AddNzbName(nzbname), sabnzbd.AddNzbCategory(category), sabnzbd.AddNzbPriority(priority))
	if err != nil {
		return err
	}
	return nil
}
