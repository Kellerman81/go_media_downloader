package main

import (
	"context"
	"html/template"
	"net"
	"net/http"
	"time"
)

// The setup page asks for the portal address. It is served on 127.0.0.1
// and opened in the app window; once the address works, the same window
// goes on to the portal.

var setupPage = template.Must(template.New("setup").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Go Media Player</title>
<style>
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:#222e3c;color:#e9ecef;
 font-family:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.box{background:#2b3a4c;border-radius:12px;padding:2rem 2.25rem;width:min(30rem,90vw);box-shadow:0 10px 30px rgba(0,0,0,.35)}
h1{font-size:1.4rem;margin:0 0 .25rem}p{color:#adb5bd;margin:0 0 1.25rem;line-height:1.4}
label{display:block;font-size:.9rem;margin-bottom:.35rem}
input{width:100%;box-sizing:border-box;padding:.65rem .75rem;border-radius:8px;border:1px solid #4b5d73;background:#1d2733;color:#fff;font-size:1rem}
button{margin-top:1rem;width:100%;padding:.7rem;border:0;border-radius:8px;background:#3b7ddd;color:#fff;font-size:1rem;cursor:pointer}
.err{background:#5c2b2f;color:#ffd7da;border-radius:8px;padding:.6rem .75rem;margin-bottom:1rem;font-size:.9rem}
.hint{font-size:.8rem;color:#8a96a3;margin-top:1rem}
</style></head><body><form class="box" method="post" action="/save">
<h1>Go Media Player</h1><p>Enter the address of your Go Media Downloader portal.</p>
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}
<label for="url">Portal address</label>
<input id="url" name="url" value="{{.URL}}" placeholder="https://media.example.com" autofocus required>
<button type="submit">Connect</button>
<div class="hint">Change it later by starting the player with -setup, or with -url &lt;address&gt;.</div>
</form></body></html>`))

var redirectPage = template.Must(template.New("go").Parse(`<!doctype html><meta charset="utf-8">
<meta http-equiv="refresh" content="0;url={{.}}"><title>Go Media Player</title><a href="{{.}}">Continue</a>`))

// setup serves the setup page and returns its address; saved receives the
// portal address once it was checked and stored.
func setup(dir string, cfg config, problem string) (string, <-chan string, func(), error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, nil, err
	}

	saved := make(chan string, 1)
	mux := http.NewServeMux()

	show := func(w http.ResponseWriter, url, msg string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = setupPage.Execute(w, map[string]string{"URL": url, "Error": msg})
	}

	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) { show(w, cfg.URL, problem) })
	mux.HandleFunc("POST /save", func(w http.ResponseWriter, r *http.Request) {
		raw := r.FormValue("url")

		portal, err := normalizeURL(raw)
		if err == nil {
			err = reachable(portal)
		}

		if err != nil {
			show(w, raw, err.Error())
			return
		}

		cfg.URL = portal
		if err := saveConfig(dir, cfg); err != nil {
			show(w, raw, "The address works but can't be saved: "+err.Error())
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = redirectPage.Execute(w, appURL(portal))

		select {
		case saved <- portal:
		default:
		}
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()

	stop := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_ = srv.Shutdown(ctx)
	}

	return "http://" + ln.Addr().String() + "/", saved, stop, nil
}
