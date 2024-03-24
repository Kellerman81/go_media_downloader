package sabnzbd

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

//Source: https://github.com/mrobinsn/go-sabnzbd - fixed:add category

const (
	Byte  = 1
	KByte = Byte * 1000
	MByte = KByte * 1000
	GByte = MByte * 1000
	TByte = GByte * 1000
	PByte = TByte * 1000
	EByte = PByte * 1000
)

const (
	KiByte = 1 << ((iota + 1) * 10)
	MiByte = KByte * 1000
	GiByte = MByte * 1000
	TiByte = GByte * 1000
	PiByte = TByte * 1000
	EiByte = PByte * 1000
)

type Sabnzbd struct {
	mu       sync.RWMutex
	https    bool
	insecure bool
	addr     string
	path     string
	auth     authenticator
	httpUser string
	httpPass string
	rt       http.RoundTripper
}

func New(options ...Option) (s *Sabnzbd, err error) {
	s = &Sabnzbd{
		addr: "localhost:8080",
		path: "api",
		auth: &noneAuth{},
	}

	for _, option := range options {
		if err := option(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *Sabnzbd) SetOptions(options ...Option) (err error) {
	for _, option := range options {
		if err := option(s); err != nil {
			return err
		}
	}

	return nil
}

func (s *Sabnzbd) useHTTPS() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.https = true
}

func (s *Sabnzbd) useInsecureHTTP() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.insecure = true
}

func (s *Sabnzbd) useHTTP() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.https = false
}

func (s *Sabnzbd) setAddr(addr string) error {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.addr = addr
	return nil
}

func (s *Sabnzbd) setPath(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.path = path
	return nil
}

func (s *Sabnzbd) setAuth(a authenticator) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auth = a
	return nil
}

func (s *Sabnzbd) setRoundTripper(rt http.RoundTripper) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rt = rt
	return nil
}

type sabnzbdURL struct {
	*url.URL
	v    url.Values
	auth authenticator
	rt   http.RoundTripper
}

var (
	defaultTransport         = &http.Transport{}
	defaultInsecureTransport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
)

func (s *Sabnzbd) url() *sabnzbdURL {
	s.mu.RLock()
	var t http.RoundTripper = defaultTransport
	if s.rt != nil {
		t = s.rt
	}
	defer s.mu.RUnlock()
	su := &sabnzbdURL{
		URL: &url.URL{
			Scheme: "http",
			Host:   s.addr,
			Path:   s.path,
		},
		auth: s.auth,
		rt:   t,
	}
	if s.https {
		su.Scheme = "https"
	}

	if s.httpUser != "" {
		su.User = url.UserPassword(s.httpUser, s.httpPass)
	}

	if s.insecure {
		su.Unsecure()
	}

	su.v = su.URL.Query()
	return su
}

func (su *sabnzbdURL) SetJSONOutput() {
	su.v.Set("output", "json")
}

func (su *sabnzbdURL) SetMode(mode string) {
	su.v.Set("mode", mode)
}

func (su *sabnzbdURL) Authenticate() {
	su.auth.Authenticate(su.v)
}

func (su *sabnzbdURL) String() string {
	su.RawQuery = su.v.Encode()
	return su.URL.String()
}

func (su *sabnzbdURL) Unsecure() {
	su.rt = defaultInsecureTransport
}

func (su *sabnzbdURL) CallJSON(r any) error {
	httpClient := &http.Client{Transport: su.rt}
	//fmt.Printf("GET URL: %s", su.String())
	resp, err := httpClient.Get(su.String())
	if err != nil {
		return fmt.Errorf("sabnzbdURL:CallJSON: failed to get: %s: %v", su.String(), err)
	}
	defer resp.Body.Close()
	//fmt.Printf("Status: %v\n", resp.Status)

	//decoder := json.NewDecoder(resp.Body)
	respStr, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("sabnzbdURL:CallJSON: failed to read response: %v", err)
	}

	if err = json.Unmarshal(respStr, r); err != nil {
		return fmt.Errorf("sabnzbdURL:CallJSON: failed to decode json: %v: %s", err, string(respStr))
	}
	if err, ok := r.(error); ok {
		return apiStringError(err.Error())
	}

	return nil
}

func (su *sabnzbdURL) CallJSONMultipart(reader io.Reader, contentType string, r any) error {
	httpClient := &http.Client{Transport: su.rt}
	resp, err := httpClient.Post(su.String(), contentType, reader)
	if err != nil {
		return fmt.Errorf("sabnzbdURL:CallJSONMultipart: failed to post: %s: %v", su.String(), err)
	}
	defer resp.Body.Close()

	respStr, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("sabnzbdURL:CallJSONMultipart: failed to read response: %v", err)
	}
	if err = json.Unmarshal(respStr, r); err != nil {
		return fmt.Errorf("sabnzbdURL:CallJSONMultipart: failed to decode json: %v: %s", err, string(respStr))
	}
	if err, ok := r.(error); ok {
		return apiStringError(err.Error())
	}

	return nil
}

var (
	ErrApikeyIncorrect = errors.New("API Key Incorrect")
	ErrApikeyRequired  = errors.New("API Key Required")
)

func apiStringError(str string) error {
	switch {
	case str == "":
		return nil
	case strings.Contains(str, ErrApikeyIncorrect.Error()):
		return ErrApikeyIncorrect
	case strings.Contains(str, ErrApikeyRequired.Error()):
		return ErrApikeyRequired
	default:
		return errors.New(str)
	}
}

var ErrInvalidQueueCompleteAction = errors.New("invalid queue complete action")
