package main

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// The start page is a file opened in the app window: it asks for the portal
// address (remembered in the window's own browser profile), checks that the
// portal answers and goes on to it. Nothing has to keep running in the
// background, so it works however the browser is started.

const setupFile = "go-media-player-start.html"

const setupPage = `<!doctype html>
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
button:disabled{opacity:.6;cursor:default}
.err{background:#5c2b2f;color:#ffd7da;border-radius:8px;padding:.6rem .75rem;margin-bottom:1rem;font-size:.9rem}
.hint{font-size:.8rem;color:#8a96a3;margin-top:1rem}
[hidden]{display:none}
</style></head><body>
<form class="box" id="f" hidden>
<h1>Go Media Player</h1><p>Enter the address of your Go Media Downloader portal.</p>
<div class="err" id="err" hidden></div>
<label for="url">Portal address</label>
<input id="url" placeholder="https://media.example.com or 192.168.1.10:9090" autocomplete="url" required>
<button type="submit" id="go">Connect</button>
<div class="hint">To change it later, start the player with -setup.</div>
</form>
<div class="box" id="wait" hidden><h1>Go Media Player</h1><p id="msg">Connecting…</p></div>
<script>
var KEY = 'gmd-portal', q = new URLSearchParams(location.search);
var form = document.getElementById('f'), input = document.getElementById('url'), err = document.getElementById('err');

function stored() { try { return localStorage.getItem(KEY) || ''; } catch (e) { return ''; } }
function remember(u) { try { localStorage.setItem(KEY, u); } catch (e) {} }

// normalize cleans an address the way the player's -url does: http:// when
// no scheme is given, without the web interface's own path.
function normalize(raw) {
	raw = (raw || '').trim();
	if (!raw) { return ''; }
	if (raw.indexOf('://') < 0) { raw = 'http://' + raw; }
	try {
		var u = new URL(raw);
		if (u.protocol !== 'http:' && u.protocol !== 'https:') { return ''; }
		var path = u.pathname, i = path.indexOf('/api');
		if (i >= 0) { path = path.slice(0, i); }
		return (u.origin + path).replace(/\/+$/, '');
	} catch (e) { return ''; }
}

function showForm(value, problem) {
	document.getElementById('wait').hidden = true;
	form.hidden = false;
	input.value = value || '';
	err.hidden = !problem;
	err.textContent = problem || '';
	document.getElementById('go').disabled = false;
	input.focus();
}

// connect checks that the portal answers (its app icon loads; works from a
// file page without CORS) and opens it.
function connect(portal) {
	document.getElementById('msg').textContent = 'Connecting to ' + portal + '…';
	form.hidden = true;
	document.getElementById('wait').hidden = false;
	var img = new Image(), done = false;
	var fail = function () {
		if (done) { return; } done = true;
		showForm(portal, 'The portal at ' + portal + " can't be reached - check the address and that the server runs.");
	};
	var timer = setTimeout(fail, 10000);
	img.onload = function () {
		if (done) { return; } done = true; clearTimeout(timer);
		remember(portal);
		location.replace(portal + '/api/admin/library?app=1');
	};
	img.onerror = function () { clearTimeout(timer); fail(); };
	img.src = portal + '/static/img/icons/icon-48x48.png?check=' + Date.now();
}

form.addEventListener('submit', function (e) {
	e.preventDefault();
	var portal = normalize(input.value);
	if (!portal) { showForm(input.value, 'Enter the portal address, e.g. https://media.example.com or http://192.168.1.10:9090'); return; }
	document.getElementById('go').disabled = true;
	connect(portal);
});

// The address given to the player (-url) wins once; else the one used last.
var given = normalize(q.get('url') || ''), portal = q.get('force') === '1' && given ? given : (stored() || given);
if (portal && q.get('setup') !== '1') { connect(portal); } else { showForm(portal, ''); }
</script></body></html>
`

// startPage writes the start page into dir and returns its address, with
// the configured portal (url), whether that one must win over the address
// the page remembers (force) and whether to ask again (setup).
func startPage(dir, portal string, force, setup bool) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	path := filepath.Join(dir, setupFile)
	if err := os.WriteFile(path, []byte(setupPage), 0o644); err != nil {
		return "", err
	}

	q := url.Values{}
	if portal != "" {
		q.Set("url", portal)
	}

	if force {
		q.Set("force", "1")
	}

	if setup {
		q.Set("setup", "1")
	}

	return fileURL(path, q), nil
}

// fileURL is the file:// address of an absolute path (Windows: /C:/...).
func fileURL(path string, q url.Values) string {
	p := filepath.ToSlash(path)
	if runtime.GOOS == "windows" || (len(p) > 1 && p[1] == ':') {
		p = "/" + strings.TrimPrefix(p, "/")
	}

	u := url.URL{Scheme: "file", Path: p, RawQuery: q.Encode()}

	return u.String()
}
