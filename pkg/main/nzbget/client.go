package nzbget

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/pkg/errors"
)

// Source: https://github.com/dashotv/flame
const (
	PriorityVeryLow  = -100
	PriorityLow      = -50
	PriorityNormal   = 0
	PriorityHigh     = 50
	PriorityVeryHigh = 100
	PriorityForce    = 900
)

func NewOptions() *AppendOptions {
	return &AppendOptions{
		Category: "",
		Priority: PriorityNormal,
		DupeMode: "SCORE",
	}
}

func NewClient(endpoint string) *Client {
	client := &Client{
		URL: endpoint,
		rpc: NewJSONClient(endpoint),
	}
	return client
}

func (c *Client) List() (*GroupResponse, error) {
	s, err := c.Status()
	if err != nil {
		return nil, err
	}

	r := &GroupResponse{response: &response{Timestamp: time.Now(), Status: s}}
	err = c.request("listgroups", nil, r)

	if err == nil {
		return r, nil
	}
	return r, err
}

func (c *Client) Groups() ([]Group, error) {
	r, err := c.List()
	if err != nil {
		return nil, errors.Wrap(err, "could not get list")
	}
	return r.Result, nil
}

func (c *Client) Remove(number int) error {
	// group delete
	return c.EditQueue("GroupDelete", "", []int{number})
}

func (c *Client) Delete(number int) error {
	return c.EditQueue("HistoryDelete", "", []int{number})
}

func (c *Client) Destroy(number int) error {
	return c.EditQueue("HistoryFinalDelete", "", []int{number})
}

func (c *Client) Pause(number int) error {
	return c.EditQueue("GroupPause", "", []int{number})
}

func (c *Client) Resume(number int) error {
	return c.EditQueue("GroupResume", "", []int{number})
}

func (c *Client) PauseAll() error {
	r, err := c.rpc.Call("pausedownload", nil)
	if err != nil {
		return errors.Wrap(err, "could not pause all")
	}
	if r.Error != nil {
		return errors.Wrap(err, "could not pause all")
	}
	if r.Result != true {
		return errors.New("response result is not true")
	}
	return nil
}

func (c *Client) ResumeAll() error {
	r, err := c.rpc.Call("resumedownload", nil)
	if err != nil {
		return errors.Wrap(err, "could not pause all")
	}
	if r.Error != nil {
		return errors.Wrap(err, "could not pause all")
	}
	if r.Result != true {
		return errors.New("response result is not true")
	}
	return nil
}

func (c *Client) EditQueue(command, param string, ids []int) error {
	r, err := c.rpc.Call("editqueue", command, param, ids)
	if err != nil {
		return errors.Wrap(err, "could not pause all")
	}
	if r.Error != nil {
		return errors.Wrap(err, "could not pause all")
	}
	if r.Result != true {
		return errors.New("response result is not true")
	}
	return nil
}

func (c *Client) Add(URL string, options *AppendOptions) (int64, error) {
	path, err := downloadURL(URL)
	if err != nil {
		return 0, errors.Wrap(err, "could not download url")
	}
	defer os.Remove(path)
	str, err := readFile(path)
	if err != nil {
		return 0, errors.Wrap(err, "could not read downloaded file")
	}

	name, err := nzbName(str)
	if err != nil {
		return 0, errors.Wrap(err, "could not get nzb name")
	}
	enc := base64encode(str)

	if options.NiceName != "" {
		name = options.NiceName
	}

	r, err := c.rpc.Call(
		"append",
		name,
		enc,
		options.Category,
		options.Priority,
		options.AddToTop,
		options.AddPaused,
		options.DupeKey,
		options.DupeScore,
		options.DupeMode,
		options.Parameters,
	)
	if err != nil {
		if r != nil && r.Error != nil {
			return 0, errors.Wrap(err, r.Error.Error())
		}
		return 0, err
	}

	n := r.Result.(json.Number)
	i, err := n.Int64()
	if err != nil {
		return 0, err
	}

	return i, nil
}

func (c *Client) History(hidden bool) ([]History, error) {
	r := &historyResponse{}
	err := c.request("history", url.Values{"": []string{fmt.Sprintf("%t", hidden)}}, r)
	if err != nil {
		return nil, err
	}
	return r.Result, nil
}

func (c *Client) Status() (*Status, error) {
	r := &statusResponse{}
	err := c.request("status", url.Values{}, r)
	if err != nil {
		return r.Result, err
	}
	return r.Result, nil
}

func (c *Client) Version() (string, error) {
	version := &versionResponse{}
	err := c.request("version", url.Values{}, version)
	if err != nil {
		return "", err
	}
	return version.Version, nil
}

func (c *Client) request(path string, params url.Values, target any) (err error) {
	var urlv string
	var request *http.Request

	urlv = c.URL + "/" + path

	if request, err = http.NewRequest("GET", urlv, http.NoBody); err != nil {
		return errors.Wrap(err, "creating "+urlv+" request failed")
	}
	request.URL.RawQuery = params.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	var response *http.Response
	if response, err = client.Do(request); err != nil {
		// log.Fatal(err)
		return errors.Wrap(err, "error making http request")
	}
	defer response.Body.Close()

	var body []byte
	if body, err = io.ReadAll(response.Body); err != nil {
		// log.Fatal(err)
		return errors.Wrap(err, "reading request body")
	}

	// logrus.Debugf("body: %s", string(body))

	if target == nil {
		return nil
	}

	if err = json.Unmarshal(body, &target); err != nil {
		return errors.Wrap(err, "json unmarshal")
	}

	return nil
}
