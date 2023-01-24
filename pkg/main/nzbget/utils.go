package nzbget

import (
	"encoding/base64"
	"encoding/xml"
	"io"
	"net/http"
	"os"

	"github.com/Kellerman81/go_media_downloader/logger"
	nzb "github.com/andrewstuart/go-nzb"
	"github.com/pkg/errors"
)

//Source: https://github.com/dashotv/flame

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", errors.Wrap(err, "could not read file")
	}

	return string(b), nil
}

func base64encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func downloadURL(url string) (string, error) {
	// Get the data
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := logger.WebClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "could not http get url")
	}
	defer resp.Body.Close()

	file, err := os.CreateTemp("./temp", "flame-download-*")
	if err != nil {
		return "", errors.Wrap(err, "could not get tmp file")
	}
	defer file.Close()

	// Write the body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "could not copy file")
	}
	return file.Name(), nil
}

func nzbName(data string) (string, error) {
	var n nzb.NZB
	err := xml.Unmarshal([]byte(data), &n)
	if err != nil {
		return "", errors.Wrap(err, "could not unmarshal")
	}

	return n.Meta["name"], nil
}
