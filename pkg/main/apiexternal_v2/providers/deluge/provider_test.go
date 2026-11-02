package deluge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

type rpcReq struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
}

// fakeDeluge answers the web JSON API. expireOnce makes the first
// non-auth call after login answer "Not authenticated".
type fakeDeluge struct {
	mu         sync.Mutex
	methods    []string
	params     map[string][]any
	logins     int
	expireOnce bool
	handler    func(method string, params []any) (any, map[string]any)
}

func (f *fakeDeluge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req rpcReq
	_ = json.NewDecoder(r.Body).Decode(&req)

	f.mu.Lock()
	f.methods = append(f.methods, req.Method)
	if f.params == nil {
		f.params = map[string][]any{}
	}
	f.params[req.Method] = req.Params

	var (
		result any
		rpcErr map[string]any
	)

	switch req.Method {
	case "auth.login":
		f.logins++
		result = true
	case "web.get_hosts":
		result = []any{}
	default:
		if f.expireOnce {
			f.expireOnce = false
			rpcErr = map[string]any{"message": "Not authenticated", "code": 1}
		} else {
			result, rpcErr = f.handler(req.Method, req.Params)
		}
	}
	f.mu.Unlock()

	_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": rpcErr, "id": 1})
}

func TestSessionExpiryReauthenticates(t *testing.T) {
	f := &fakeDeluge{
		expireOnce: true,
		handler: func(string, []any) (any, map[string]any) {
			return true, nil
		},
	}

	srv := httptest.NewServer(f)
	defer srv.Close()

	p := NewProvider(srv.URL, "", "pw")
	p.storeSessionID("authenticated") // a session that has since expired

	if err := p.PauseTorrent(context.Background(), "h1"); err != nil {
		t.Fatalf("PauseTorrent: %v", err)
	}

	if f.logins != 1 {
		t.Fatalf("logins = %d, want 1 re-login", f.logins)
	}

	if got := f.methods[len(f.methods)-1]; got != "core.pause_torrents" {
		t.Fatalf("last method = %q, want core.pause_torrents", got)
	}

	if ids, _ := f.params["core.pause_torrents"][0].([]any); len(ids) != 1 || ids[0] != "h1" {
		t.Fatalf("pause params = %v, want [[h1]]", f.params["core.pause_torrents"])
	}
}

func TestAddTorrentFileAndDuplicate(t *testing.T) {
	const hash = "0123456789abcdef0123456789abcdef01234567"

	f := &fakeDeluge{
		handler: func(string, []any) (any, map[string]any) {
			return nil, map[string]any{
				"message": "AddTorrentError: Torrent already in session (" + hash + ").",
				"code":    3,
			}
		},
	}

	srv := httptest.NewServer(f)
	defer srv.Close()

	p := NewProvider(srv.URL, "", "pw")

	resp, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{
		TorrentData: []byte("raw"),
		Priority:    1,
	})
	if err != nil {
		t.Fatalf("AddTorrent: %v", err)
	}

	if !resp.Success || resp.Hash != hash {
		t.Fatalf("resp = %+v, want duplicate treated as success", resp)
	}

	params := f.params["core.add_torrent_file"]
	if params[1] != base64.StdEncoding.EncodeToString([]byte("raw")) {
		t.Errorf("filedump = %v, want base64", params[1])
	}

	opts, _ := params[2].(map[string]any)
	if _, ok := opts["download_location"]; ok {
		t.Errorf("download_location sent for empty SavePath")
	}

	if _, ok := opts["priority"]; ok {
		t.Errorf("non-existent priority option sent")
	}
}

func TestRemoveTorrentFalseIsError(t *testing.T) {
	f := &fakeDeluge{
		handler: func(string, []any) (any, map[string]any) {
			return false, nil
		},
	}

	srv := httptest.NewServer(f)
	defer srv.Close()

	p := NewProvider(srv.URL, "", "pw")
	if err := p.RemoveTorrent(context.Background(), "h1", false); err == nil {
		t.Fatal("expected error when remove_torrent returns false")
	}
}

func TestHTTP401Reauthenticates(t *testing.T) {
	var (
		mu     sync.Mutex
		failed bool
		logins int
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcReq
		_ = json.NewDecoder(r.Body).Decode(&req)

		mu.Lock()
		defer mu.Unlock()

		switch {
		case req.Method == "auth.login":
			logins++
			_, _ = w.Write([]byte(`{"result":true,"error":null,"id":1}`))
		case req.Method == "web.get_hosts":
			_, _ = w.Write([]byte(`{"result":[],"error":null,"id":1}`))
		case !failed:
			failed = true
			w.WriteHeader(http.StatusUnauthorized)
		default:
			_, _ = w.Write([]byte(`{"result":true,"error":null,"id":1}`))
		}
	}))
	defer srv.Close()

	p := NewProvider(srv.URL, "", "pw")
	p.storeSessionID("authenticated")

	if err := p.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}

	if logins != 1 {
		t.Fatalf("logins = %d, want 1", logins)
	}
}

const labelTestHash = "0123456789abcdef0123456789abcdef01234567"

func labelFake(plugins, labels []any, setErr bool) *fakeDeluge {
	return &fakeDeluge{
		handler: func(method string, _ []any) (any, map[string]any) {
			switch method {
			case "core.add_torrent_magnet":
				return labelTestHash, nil
			case "core.get_enabled_plugins":
				return plugins, nil
			case "label.get_labels":
				return labels, nil
			case "label.set_torrent":
				if setErr {
					return nil, map[string]any{"message": "Unknown Label", "code": 4}
				}

				return nil, nil
			}

			return nil, nil
		},
	}
}

func addWithLabel(t *testing.T, f *fakeDeluge, label string) {
	t.Helper()

	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	p := NewProvider(srv.URL, "", "pw")

	resp, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{
		URL:   "magnet:?xt=urn:btih:abc",
		Label: label,
	})
	if err != nil || !resp.Success || resp.Hash != labelTestHash {
		t.Fatalf("AddTorrent = %+v, %v; want success", resp, err)
	}
}

func TestAddTorrentLabel(t *testing.T) {
	t.Run("creates and sets label", func(t *testing.T) {
		f := labelFake([]any{"Label"}, []any{"other"}, false)
		addWithLabel(t, f, "Movies")

		if got := f.params["label.add"]; len(got) != 1 || got[0] != "movies" {
			t.Errorf("label.add params = %v, want [movies]", got)
		}

		if got := f.params["label.set_torrent"]; len(got) != 2 || got[0] != labelTestHash || got[1] != "movies" {
			t.Errorf("label.set_torrent params = %v", got)
		}
	})

	t.Run("existing label not re-added", func(t *testing.T) {
		f := labelFake([]any{"Label"}, []any{"movies"}, false)
		addWithLabel(t, f, "movies")

		if slices.Contains(f.methods, "label.add") {
			t.Error("label.add called for an existing label")
		}

		if !slices.Contains(f.methods, "label.set_torrent") {
			t.Error("label.set_torrent not called")
		}
	})

	t.Run("plugin disabled", func(t *testing.T) {
		f := labelFake([]any{"Scheduler"}, nil, false)
		addWithLabel(t, f, "movies")

		if slices.Contains(f.methods, "label.set_torrent") || slices.Contains(f.methods, "label.get_labels") {
			t.Errorf("label calls made without the plugin: %v", f.methods)
		}
	})

	t.Run("label failure does not fail the add", func(t *testing.T) {
		addWithLabel(t, labelFake([]any{"Label"}, []any{"movies"}, true), "movies")
	})

	t.Run("no label configured", func(t *testing.T) {
		f := labelFake([]any{"Label"}, nil, false)
		addWithLabel(t, f, "")

		if slices.Contains(f.methods, "core.get_enabled_plugins") {
			t.Error("plugin lookup without a label")
		}
	})
}
