package apiexternal

import (
	"errors"

	"github.com/Kellerman81/go_media_downloader/sabnzbd"
)

func SendToSabnzbd(server string, apikey string, urlv string, category string, nzbname string, priority int) error {
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
