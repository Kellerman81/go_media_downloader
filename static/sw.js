// Service worker of the installed web app. It caches nothing (every page
// needs the login and live data); it only shows a short notice instead of
// the browser's error page when the server can't be reached.
self.addEventListener('install', function () { self.skipWaiting(); });
self.addEventListener('activate', function (e) { e.waitUntil(self.clients.claim()); });

var offline = '<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">' +
	'<title>Go Media - offline</title><body style="font-family:system-ui,sans-serif;background:#222e3c;color:#fff;' +
	'display:flex;align-items:center;justify-content:center;height:100vh;margin:0;text-align:center">' +
	'<div><h2>The server can\'t be reached</h2><p>Check the connection and try again.</p>' +
	'<button onclick="location.reload()" style="padding:.5rem 1.25rem;border:0;border-radius:.4rem">Retry</button></div>';

self.addEventListener('fetch', function (e) {
	if (e.request.mode !== 'navigate') { return; }
	e.respondWith(fetch(e.request).catch(function () {
		return new Response(offline, { headers: { 'Content-Type': 'text/html; charset=utf-8' } });
	}));
});
