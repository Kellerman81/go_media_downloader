package apiexternal

import (
	"errors"

	"github.com/mrobinsn/go-sabnzbd"
)

func SendToSabnzbd(server string, apikey string, url string, category string, nzbname string, priority int) error {
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

	_, err = s.AddURL(sabnzbd.AddNzbUrl(url), sabnzbd.AddNzbCategory(category), sabnzbd.AddNzbName(nzbname), sabnzbd.AddNzbPriority(priority))
	if err != nil {
		return err
	}
	return nil
}
