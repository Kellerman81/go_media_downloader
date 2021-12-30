package nzbget

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

//Source: https://github.com/dashotv/flame

const PriorityVeryLow = -100
const PriorityLow = -50
const PriorityNormal = 0
const PriorityHigh = 50
const PriorityVeryHigh = 100
const PriorityForce = 900

func NewOptions() *appendOptions {
	return &appendOptions{
		Category: "",
		Priority: PriorityNormal,
		DupeMode: "SCORE",
	}
}

func NewClient(endpoint string) *client {
	client := &client{
		URL: endpoint,
		rpc: NewJsonClient(endpoint),
	}
	return client
}

func (c *client) List() (*groupResponse, error) {
	s, err := c.Status()
	if err != nil {
		return nil, err
	}

	r := &groupResponse{response: &response{Timestamp: time.Now(), Status: s}}
	err = c.request("listgroups", nil, r)
	return r, err
}

func (c *client) Groups() ([]group, error) {
	r, err := c.List()
	if err != nil {
		return nil, errors.Wrap(err, "could not get list")
	}
	return r.Result, nil
}

func (c *client) Remove(number int) error {
	// group delete
	return c.EditQueue("GroupDelete", "", []int{number})
}

func (c *client) Delete(number int) error {
	return c.EditQueue("HistoryDelete", "", []int{number})
}

func (c *client) Destroy(number int) error {
	return c.EditQueue("HistoryFinalDelete", "", []int{number})
}

func (c *client) Pause(number int) error {
	return c.EditQueue("GroupPause", "", []int{number})
}

func (c *client) Resume(number int) error {
	return c.EditQueue("GroupResume", "", []int{number})
}

func (c *client) PauseAll() error {
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

func (c *client) ResumeAll() error {
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

func (c *client) EditQueue(command, param string, ids []int) error {
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

func (c *client) Add(URL string, options *appendOptions) (int64, error) {
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

	r, err := c.rpc.Call("append", name, enc, options.Category, options.Priority, options.AddToTop, options.AddPaused, options.DupeKey, options.DupeScore, options.DupeMode, options.Parameters)
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

func (c *client) History(hidden bool) ([]history, error) {
	r := &historyResponse{}
	err := c.request("history", url.Values{"": []string{fmt.Sprintf("%t", hidden)}}, r)
	if err != nil {
		return nil, err
	}
	return r.Result, nil
}

func (c *client) Status() (*status, error) {
	r := &statusResponse{}
	err := c.request("status", url.Values{}, r)
	if err != nil {
		return r.Result, err
	}
	return r.Result, nil
}

func (c *client) Version() (string, error) {
	version := &versionResponse{}
	err := c.request("version", url.Values{}, version)
	if err != nil {
		return "", err
	}
	return version.Version, nil
}

func (c *client) request(path string, params url.Values, target interface{}) (err error) {
	var url string
	var client *http.Client
	var request *http.Request
	var response *http.Response
	var body []byte

	url = fmt.Sprintf("%s/%s", c.URL, path)

	if request, err = http.NewRequest("GET", url, nil); err != nil {
		return errors.Wrap(err, "creating "+url+" request failed")
	}
	request.URL.RawQuery = params.Encode()

	client = &http.Client{Timeout: 5 * time.Second}
	if response, err = client.Do(request); err != nil {
		//log.Fatal(err)
		return errors.Wrap(err, "error making http request")
	}
	defer response.Body.Close()

	if body, err = ioutil.ReadAll(response.Body); err != nil {
		//log.Fatal(err)
		return errors.Wrap(err, "reading request body")
	}

	logrus.Debugf("body: %s", string(body))

	if target == nil {
		return nil
	}

	if err = json.Unmarshal(body, &target); err != nil {
		return errors.Wrap(err, "json unmarshal")
	}

	return nil
}
