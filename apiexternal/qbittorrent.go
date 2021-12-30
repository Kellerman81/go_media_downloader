package apiexternal

import (
	"bytes"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	wrapper "github.com/pkg/errors"

	"golang.org/x/net/publicsuffix"
)

func SendToQBittorrent(host string, port string, username string, password string, url string, dlpath string, addpaused string) error {
	cl := NewQBittorrentClient("http://" + host + ":" + port + "/")
	var err error
	_, err = cl.Login(username, password)
	if err == nil {
		options := map[string]string{
			"savepath": dlpath,
			"paused":   addpaused,
		}
		// perform connection to Deluge server
		_, err = cl.DownloadFromLink(url, options)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		fmt.Println(err)
	}
	return err
}

//ErrBadResponse means that qbittorrent sent back an unexpected response
var ErrBadResponse = errors.New("received bad response")

//Client creates a connection to qbittorrent and performs requests
type qbtClient struct {
	http          *http.Client
	URL           string
	Authenticated bool
	Jar           http.CookieJar
}

//NewClient creates a new client connection to qbittorrent
func NewQBittorrentClient(url string) *qbtClient {
	client := &qbtClient{}

	// ensure url ends with "/"
	if url[len(url)-1:] != "/" {
		url += "/"
	}

	client.URL = url + "api/v2/"

	// create cookie jar
	client.Jar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	client.http = &http.Client{
		Jar: client.Jar,
	}
	return client
}

//post will perform a POST request with no content-type specified
func (client *qbtClient) post(endpoint string, opts map[string]string) (*http.Response, error) {
	var req *http.Request
	var err error
	// add optional parameters that the user wants
	if opts != nil {
		form := url.Values{}
		for k, v := range opts {
			form.Add(k, v)
		}
		req, err = http.NewRequest("POST", client.URL+endpoint, strings.NewReader(form.Encode()))

	} else {
		req, err = http.NewRequest("POST", client.URL+endpoint, nil)
	}
	if err != nil {
		return nil, wrapper.Wrap(err, "failed to build request")
	}

	// add the content-type so qbittorrent knows what to expect
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// add user-agent header to allow qbittorrent to identify us
	req.Header.Set("User-Agent", "go-qbittorrent v0.1")

	resp, err := client.http.Do(req)
	if err != nil {
		return nil, wrapper.Wrap(err, "failed to perform request")
	}

	return resp, nil

}

//postMultipart will perform a multiple part POST request
func (client *qbtClient) postMultipart(endpoint string, buffer bytes.Buffer, contentType string) (*http.Response, error) {
	req, err := http.NewRequest("POST", client.URL+endpoint, &buffer)
	if err != nil {
		return nil, wrapper.Wrap(err, "error creating request")
	}

	// add the content-type so qbittorrent knows what to expect
	req.Header.Set("Content-Type", contentType)
	// add user-agent header to allow qbittorrent to identify us
	req.Header.Set("User-Agent", "go-qbittorrent v0.1")

	resp, err := client.http.Do(req)
	if err != nil {
		return nil, wrapper.Wrap(err, "failed to perform request")
	}

	return resp, nil
}

//writeOptions will write a map to the buffer through multipart.NewWriter
func writeOptions(writer *multipart.Writer, opts map[string]string) {
	for key, val := range opts {
		writer.WriteField(key, val)
	}
}

//postMultipartData will perform a multiple part POST request without a file
func (client *qbtClient) postMultipartData(endpoint string, opts map[string]string) (*http.Response, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	// write the options to the buffer
	// will contain the link string
	writeOptions(writer, opts)

	// close the writer before doing request to get closing line on multipart request
	if err := writer.Close(); err != nil {
		return nil, wrapper.Wrap(err, "failed to close writer")
	}

	resp, err := client.postMultipart(endpoint, buffer, writer.FormDataContentType())
	if err != nil {
		return nil, err
	}

	return resp, nil
}

//Login logs you in to the qbittorrent client
//returns the current authentication status
func (client *qbtClient) Login(username string, password string) (loggedIn bool, err error) {
	credentials := make(map[string]string)
	credentials["username"] = username
	credentials["password"] = password

	resp, err := client.post("auth/login", credentials)
	if err != nil {
		return false, err
	} else if resp.Status != "200 OK" { // check for correct status code
		return false, wrapper.Wrap(ErrBadResponse, "couldnt log in with "+client.URL+"auth/login")
	}

	// change authentication status so we know were authenticated in later requests
	client.Authenticated = true

	// add the cookie to cookie jar to authenticate later requests
	if cookies := resp.Cookies(); len(cookies) > 0 {
		cookieURL, _ := url.Parse(client.URL)
		client.Jar.SetCookies(cookieURL, cookies)
	}

	// create a new client with the cookie jar and replace the old one
	// so that all our later requests are authenticated
	client.http = &http.Client{
		Jar: client.Jar,
	}

	return client.Authenticated, nil
}

//DownloadFromLink starts downloading a torrent from a link
func (client *qbtClient) DownloadFromLink(link string, options map[string]string) (*http.Response, error) {
	options["urls"] = link
	return client.postMultipartData("torrents/add", options)
}
