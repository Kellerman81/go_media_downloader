// Package nzbvortex adds NZBs to NZBVortex (macOS) over its HTTPS API.
package nzbvortex

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

var (
	errLoginFailed = errors.New("nzbvortex: login failed - check the API key")
	errEmptyNZB    = errors.New("nzbvortex: the NZB download was empty")
)

// Provider talks to one NZBVortex instance.
type Provider struct {
	*base.BaseClient
	apiKey string

	// fetch downloads the NZB; it has no NZBVortex auth or TLS exception.
	fetch *base.BaseClient

	mu        sync.Mutex
	sessionID string
}

type result struct {
	Result  string `json:"result"`
	AddUUID string `json:"add_uuid"`
}

// is compares an NZBVortex enum value, which comes as "not_logged_in" or
// "notLoggedIn" depending on the version.
func is(value, want string) bool {
	return strings.EqualFold(strings.ReplaceAll(value, "_", ""), want)
}

// NewProvider returns a provider for NZBVortex at host:port. NZBVortex only
// speaks HTTPS with a self-signed certificate, so certificate checks are off
// for this client.
func NewProvider(host string, port int, apiKey, urlBase string) *Provider {
	cfg := base.ServiceConfig("nzbvortex", "https://"+host+":"+strconv.Itoa(port)+strings.TrimRight(urlBase, "/")+"/api")
	cfg.DisableTLSVerify = true

	return &Provider{
		BaseClient: base.NewBaseClient(cfg),
		apiKey:     apiKey,
		fetch:      base.NewBaseClient(base.ServiceConfig("nzbvortex_fetch", "")),
	}
}

func (*Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderNzbVortex
}

func (*Provider) GetProviderName() string { return "nzbvortex" }

// loginHash is base64(sha256("nonce:cnonce:apikey")), the NZBVortex scheme.
func loginHash(nonce, cnonce, apiKey string) string {
	sum := sha256.Sum256([]byte(nonce + ":" + cnonce + ":" + apiKey))

	return base64.StdEncoding.EncodeToString(sum[:])
}

func (p *Provider) session(ctx context.Context, renew bool) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sessionID != "" && !renew {
		return p.sessionID, nil
	}

	var nonce struct {
		AuthNonce string `json:"authNonce"`
	}

	if err := p.MakeRequest(ctx, http.MethodGet, "/auth/nonce", nil, &nonce, nil); err != nil {
		return "", err
	}

	c := make([]byte, 16)
	rand.Read(c)
	cnonce := hex.EncodeToString(c)

	var login struct {
		LoginResult string `json:"loginResult"`
		SessionID   string `json:"sessionID"`
	}

	q := url.Values{"nonce": {nonce.AuthNonce}, "cnonce": {cnonce}, "hash": {loginHash(nonce.AuthNonce, cnonce, p.apiKey)}}
	if err := p.MakeRequest(ctx, http.MethodGet, "/auth/login?"+q.Encode(), nil, &login, nil); err != nil {
		return "", err
	}

	if !is(login.LoginResult, "successful") || login.SessionID == "" {
		return "", errLoginFailed
	}

	p.sessionID = login.SessionID

	return p.sessionID, nil
}

func (p *Provider) upload(ctx context.Context, nzb []byte, name, group string, priority int, renew bool) (result, error) {
	sid, err := p.session(ctx, renew)
	if err != nil {
		return result{}, err
	}

	var body bytes.Buffer

	mw := multipart.NewWriter(&body)

	fw, err := mw.CreateFormFile("name", name)
	if err != nil {
		return result{}, err
	}

	fw.Write(nzb)
	mw.Close()

	q := url.Values{"sessionid": {sid}, "priority": {strconv.Itoa(priority)}}
	if group != "" {
		q.Set("groupname", group)
	}

	var res result

	err = p.MakeRequestWithHeaders(ctx, http.MethodPost, "/nzb/add?"+q.Encode(), &body, &res, nil,
		map[string]string{"Content-Type": mw.FormDataContentType()})

	return res, err
}

// AddNZBURL downloads the NZB at nzbURL and uploads it as name into the
// group (NZBVortex's category). priority is -1 low, 0 normal, 1 high. It
// returns the add_uuid the queue entries carry. NZBVortex cannot add
// paused, so paused is ignored.
func (p *Provider) AddNZBURL(ctx context.Context, nzbURL, group, name string, priority int, _ bool) (string, error) {
	var nzb []byte

	err := p.fetch.MakeRequestForDownload(ctx, http.MethodGet, nzbURL, nil, nil, func(r *http.Response) error {
		var buf bytes.Buffer
		_, err := buf.ReadFrom(r.Body)
		nzb = buf.Bytes()

		return err
	})
	if err != nil {
		return "", err
	}

	if len(nzb) == 0 {
		return "", errEmptyNZB
	}

	if !strings.HasSuffix(strings.ToLower(name), ".nzb") {
		name += ".nzb"
	}

	res, err := p.upload(ctx, nzb, name, group, priority, false)
	if err == nil && is(res.Result, "notloggedin") {
		res, err = p.upload(ctx, nzb, name, group, priority, true)
	}

	if err != nil {
		return "", err
	}

	if !is(res.Result, "ok") {
		return "", errors.New("nzbvortex: add returned " + res.Result)
	}

	return res.AddUUID, nil
}

// TestConnection logs in with the API key.
func (p *Provider) TestConnection(ctx context.Context) error {
	_, err := p.session(ctx, true)

	return err
}
