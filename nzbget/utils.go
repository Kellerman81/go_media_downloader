package nzbget

import (
	"encoding/base64"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"

	nzb "github.com/andrewstuart/go-nzb"
	"github.com/pkg/errors"
)

func readFile(path string) (string, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return "", errors.Wrap(err, "could not read file")
	}

	return string(b), nil
}

func base64encode(s string) string {
	data := []byte(s)
	str := base64.StdEncoding.EncodeToString(data)
	return str
}

func base64decode(s string) string {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(b)
}

func downloadURL(URL string) (string, error) {
	// Get the data
	resp, err := http.Get(URL)
	if err != nil {
		return "", errors.Wrap(err, "could not http get url")
	}
	defer resp.Body.Close()

	file, err := ioutil.TempFile("/tmp", "flame-download-*")
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
	n := nzb.NZB{}
	err := xml.Unmarshal([]byte(data), &n)
	if err != nil {
		return "", errors.Wrap(err, "could not unmarshal")
	}

	return n.Meta["name"], nil
}
