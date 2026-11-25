package api

import (
	"net/http"
	"net/mail"
	"slices"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// The accounts' messages (requests approved or declined, requested media
// downloaded, reports handled): a bell in the top bar, toasts and desktop
// notifications for new ones, each dismissable; and the address for mails.

// HandleAccountNotifications lists the account's open messages
// (/admin/account/notifications).
func HandleAccountNotifications(c *gin.Context) {
	items := database.OpenUserNotifications(accName(accessOf(c)))
	if items == nil {
		items = []database.UserNotification{}
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// HandleAccountNotificationDismiss closes a message, id 0 all of them
// (/admin/account/notifications/dismiss: id).
func HandleAccountNotificationDismiss(c *gin.Context) {
	id64, _ := strconv.ParseUint(c.PostForm("id"), 10, 64)
	id := uint(id64)

	if err := database.DismissUserNotification(accName(accessOf(c)), &id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// HandleAccountEmail stores the address the account gets update mails at
// (/admin/account/email: email; empty: no mails).
func HandleAccountEmail(c *gin.Context) {
	email := strings.TrimSpace(c.PostForm("email"))
	reply := func(msg, level string) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, renderComponentToString(renderEmailForm(getCSRFToken(c), email, renderAlert(msg, level))))
	}

	if email != "" {
		if a, err := mail.ParseAddress(email); err != nil || a.Address != email {
			reply(t("That doesn't look like an e-mail address."), "danger")
			return
		}
	}

	if err := database.SetUserEmail(accName(accessOf(c)), email); err != nil {
		reply(t("Not saved: ")+err.Error(), "danger")
		return
	}

	reply(map[bool]string{true: t("No more mails."), false: tf("Saved: messages also go to %s.", email)}[email == ""], "success")
}

// renderEmailForm is My Account's address for update mails.
func renderEmailForm(csrf, email, alert string) gomponents.Node {
	return html.Div(html.ID("accountEmail"), gomponents.Raw(alert),
		html.Div(html.Class("card mt-3"), html.Style("max-width: 32rem"), html.Div(html.Class("card-body"),
			html.H5(html.Class("card-title"), gomponents.Text(t("Update mails"))),
			html.P(html.Class("small text-muted"), gomponents.Text(
				t("Get your messages (requests approved or declined, requested titles downloaded, reports handled) by mail as well. Leave it empty for messages in the web interface only."))),
			html.Form(hx.Post("/api/admin/account/email"), hx.Headers(wizardCSRFHeader(csrf)), hx.Target("#accountEmail"), hx.Swap("outerHTML"),
				html.Div(html.Class("mb-3"),
					html.Label(html.Class("form-label"), html.For("acc_email"), gomponents.Text(t("E-mail address"))),
					html.Input(html.Type("email"), html.Class("form-control"), html.ID("acc_email"), html.Name("email"), html.Value(email),
						gomponents.Attr("autocomplete", "email"))),
				html.Button(html.Type("submit"), html.Class("btn btn-primary"), gomponents.Text(t("Save")))))))
}

// accountEvents are the events an account gets messages of: requests only
// with the request feature, reports only with the report feature.
func accountEvents(acc *userAccess) []string {
	events := make([]string, 0, len(providers.Events))
	for _, e := range providers.Events {
		switch {
		case (e == providers.EventRequest || e == providers.EventAvailable) && !acc.CanRequest():
		case e == providers.EventReport && !acc.CanReport():
		default:
			events = append(events, e)
		}
	}

	return events
}

// pushTargetLabels name an account's recipient per push notification type.
var pushTargetLabels = map[string]string{"ntfy": t("ntfy topic"), "pushover": t("Pushover user key"), "telegram": t("Telegram chat ID")}

// HandleAccountMessages stores which channels the account gets each event's
// messages on and its push recipient (/admin/account/messages: <event>=<channel>
// per ticked box, push_target).
func HandleAccountMessages(c *gin.Context) {
	acc := accessOf(c)
	name := accName(acc)

	reply := func(msg, level string) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, renderComponentToString(renderMessagesForm(getCSRFToken(c), acc, renderAlert(msg, level))))
	}

	if err := c.Request.ParseForm(); err != nil {
		reply(t("Not saved: ")+err.Error(), "danger")
		return
	}

	pushCfg, _ := providers.PushNotification()
	// Channels the card can't offer now (greyed out or not shown) aren't
	// sent: their stored choice stays.
	kept := map[string]bool{
		providers.ChannelMail: providers.MailNotification() == nil || database.GetUserEmail(name) == "",
		providers.ChannelPush: pushCfg == nil,
	}

	for _, e := range accountEvents(acc) {
		chosen := slices.DeleteFunc(slices.Clone(c.Request.PostForm[e]), func(ch string) bool {
			return !slices.Contains(providers.Channels, ch) || kept[ch]
		})

		for _, ch := range providers.NotifyChannels(name, e) {
			if kept[ch] {
				chosen = append(chosen, ch)
			}
		}

		if err := providers.SetNotifyChannels(name, e, chosen); err != nil {
			reply(t("Not saved: ")+err.Error(), "danger")
			return
		}
	}

	if pushCfg != nil {
		if err := database.SetUserSetting(name, providers.PushTargetSetting, strings.TrimSpace(c.PostForm("push_target"))); err != nil {
			reply(t("Not saved: ")+err.Error(), "danger")
			return
		}
	}

	reply(t("Saved."), "success")
}

// HandleAccountPushTest pushes a test message to the entered recipient
// (/admin/account/push-test: push_target).
func HandleAccountPushTest(c *gin.Context) {
	target := strings.TrimSpace(c.PostForm("push_target"))
	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": t("Enter the recipient first.")})
		return
	}

	if err := providers.SendTestPush(target, "Go Media", t("Push messages reach you here.")); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": t("Test message sent.")})
}

// renderMessagesForm is My Account's choice of channels per event and the
// push recipient.
func renderMessagesForm(csrf string, acc *userAccess, alert string) gomponents.Node {
	events := accountEvents(acc)
	if len(events) == 0 {
		return nil
	}

	name := accName(acc)
	pushCfg, _ := providers.PushNotification()
	// Mail only when the admin set it up; greyed out until the account
	// has an address.
	mailOn := providers.MailNotification() != nil
	mailOff := mailOn && database.GetUserEmail(name) == ""

	labels := map[string]string{
		providers.EventRequest:   t("Request approved or declined"),
		providers.EventAvailable: t("Requested title downloaded"),
		providers.EventReport:    t("Reported file handled"),
	}
	channels := []struct {
		id, label string
		off       bool
	}{{providers.ChannelWeb, t("Web interface"), false}}

	if mailOn {
		channels = append(channels, struct {
			id, label string
			off       bool
		}{providers.ChannelMail, t("Mail"), mailOff})
	}

	if pushCfg != nil {
		channels = append(channels, struct {
			id, label string
			off       bool
		}{providers.ChannelPush, t("Push"), false})
	}

	head := []gomponents.Node{html.Th(gomponents.Text(t("Message")))}
	for _, ch := range channels {
		head = append(head, html.Th(html.Class("text-center"), gomponents.Text(ch.label)))
	}

	rows := make([]gomponents.Node, 0, len(events))
	for _, e := range events {
		on := providers.NotifyChannels(name, e)
		cells := []gomponents.Node{html.Td(gomponents.Text(labels[e]))}

		for _, ch := range channels {
			cells = append(cells, html.Td(html.Class("text-center"),
				html.Input(html.Type("checkbox"), html.Class("form-check-input"), html.Name(e), html.Value(ch.id),
					gomponents.Attr("aria-label", labels[e]+": "+ch.label),
					gomponents.If(slices.Contains(on, ch.id), html.Checked()), gomponents.If(ch.off, html.Disabled()))))
		}

		rows = append(rows, html.Tr(cells...))
	}

	var push gomponents.Node
	if pushCfg != nil {
		label := pushTargetLabels[pushCfg.NotificationType]
		push = html.Div(html.Class("mb-3"),
			html.Label(html.Class("form-label"), html.For("acc_push"), gomponents.Text(label)),
			html.Div(html.Class("input-group"),
				html.Input(html.Type("text"), html.Class("form-control"), html.ID("acc_push"), html.Name("push_target"),
					html.Value(database.GetUserSetting(name, providers.PushTargetSetting)), gomponents.Attr("autocomplete", "off")),
				html.Button(html.Type("button"), html.Class("btn btn-outline-secondary"), gomponents.Text(t("Send test")),
					gomponents.Attr("onclick", "accPushTest(this)"))),
			html.Div(html.ID("acc_push_result"), html.Class("small mt-1"), gomponents.Attr("aria-live", "polite")))
	}

	return html.Div(html.ID("accountMessages"), gomponents.Raw(alert),
		html.Div(html.Class("card mt-3"), html.Style("max-width: 32rem"), html.Div(html.Class("card-body"),
			html.H5(html.Class("card-title"), gomponents.Text(t("Messages"))),
			html.P(html.Class("small text-muted"), gomponents.Text(t("Choose where each kind of message reaches you."))),
			gomponents.If(mailOff, html.P(html.Class("small text-muted"), gomponents.Text(t("Mail needs an e-mail address above.")))),
			html.Form(hx.Post("/api/admin/account/messages"), hx.Headers(wizardCSRFHeader(csrf)), hx.Target("#accountMessages"), hx.Swap("outerHTML"),
				html.Table(html.Class("table table-sm align-middle"), html.THead(html.Tr(head...)), html.TBody(rows...)),
				push,
				html.Button(html.Type("submit"), html.Class("btn btn-primary"), gomponents.Text(t("Save")))))),
		gomponents.If(pushCfg != nil, html.Script(gomponents.Raw(`window.accPushTest = function (btn) {
	var v = document.getElementById('acc_push').value, out = document.getElementById('acc_push_result');
	var show = function (msg, ok) { out.textContent = msg; out.className = 'small mt-1 ' + (ok ? 'text-success' : 'text-danger'); };
	btn.disabled = true;
	fetch('/api/admin/account/push-test', { method: 'POST', credentials: 'same-origin',
		headers: { 'X-CSRF-Token': `+jsString(csrf)+` }, body: new URLSearchParams({ push_target: v }) })
		.then(function (r) { return r.json(); })
		.then(function (d) { show(d.success || d.error, !!d.success); })
		.catch(function (e) { show(String(e), false); })
		.finally(function () { btn.disabled = false; });
};`))))
}

// notificationBell is the top bar's bell with the account's open messages
// (accounts only: the built-in admin login gets none).
func notificationBell(acc *userAccess) gomponents.Node {
	if acc == nil || acc.Name == "" || acc.Name == DefaultUsername {
		return nil
	}

	return gomponents.Group([]gomponents.Node{
		html.Li(html.Class("nav-item dropdown me-2"),
			html.A(html.Class("nav-link position-relative"), html.Href("#"), html.ID("gmdBell"), html.Data("bs-toggle", "dropdown"),
				gomponents.Attr("data-bs-auto-close", "outside"), html.Title(t("Messages")), gomponents.Attr("aria-label", t("Messages")),
				html.I(html.Class("fa-solid fa-bell")),
				html.Span(html.Class("badge rounded-pill bg-danger position-absolute top-0 start-100 translate-middle d-none"), html.ID("gmdBellCount"))),
			html.Div(html.Class("dropdown-menu dropdown-menu-end p-0"), html.ID("gmdBellMenu"), html.Style("width:min(24rem,92vw)"))),
		html.Script(gomponents.Raw(bellScript)),
	})
}

const bellScript = `(function () {
	var KEY = 'gmd-seen-msg:' + (window.gmdUser || ''), menu, count;
	function csrf() { var m = /(?:^|; )csrf_token=([^;]+)/.exec(document.cookie); return m ? decodeURIComponent(m[1]) : ''; }
	function esc(t) { var d = document.createElement('div'); d.textContent = t || ''; return d.innerHTML; }
	function seen(v) { try { if (v === undefined) { return +(localStorage.getItem(KEY) || 0); } localStorage.setItem(KEY, String(v)); } catch (e) { return 0; } }
	function dismiss(id) {
		fetch('/api/admin/account/notifications/dismiss', { method: 'POST', credentials: 'same-origin',
			body: new URLSearchParams({ id: String(id), csrf_token: csrf() }), headers: { 'X-CSRF-Token': csrf() } }).then(load).catch(function () {});
	}
	function render(items) {
		count.textContent = items.length;
		count.classList.toggle('d-none', !items.length);
		var html = '<div class="d-flex align-items-center justify-content-between px-3 py-2 border-bottom"><strong>' + _t('Messages') + '</strong>' +
			(items.length ? '<button type="button" class="btn btn-link btn-sm p-0" data-dismiss="0">' + _t('Dismiss all') + '</button>' : '') + '</div>';
		html += items.length ? items.map(function (n) {
			return '<div class="px-3 py-2 border-bottom d-flex gap-2"><div class="flex-grow-1" style="min-width:0">' +
				(n.link ? '<a class="fw-semibold text-decoration-none" href="' + esc(n.link) + '">' + esc(n.title) + '</a>' : '<div class="fw-semibold">' + esc(n.title) + '</div>') +
				'<div class="small">' + esc(n.message) + '</div><div class="small text-muted">' + esc(n.created) + '</div></div>' +
				'<button type="button" class="btn-close btn-sm" aria-label="' + _t('Dismiss') + '" data-dismiss="' + n.id + '"></button></div>';
		}).join('') : '<div class="px-3 py-3 small text-muted">' + _t('No messages.') + '</div>';
		if (window.Notification && Notification.permission === 'default' && window.isSecureContext) {
			html += '<div class="px-3 py-2"><button type="button" class="btn btn-sm btn-outline-secondary w-100" data-notify="1">' + _t('Show messages as desktop notifications') + '</button></div>';
		}
		menu.innerHTML = html;
	}
	function announce(items) {
		var last = seen(), max = last;
		items.slice().reverse().forEach(function (n) {
			if (n.id <= last) { return; }
			max = Math.max(max, n.id);
			if (!last) { return; } // the first visit only marks them seen
			if (typeof showToaster === 'function') { showToaster('info', n.title + ' - ' + n.message); }
			if (window.Notification && Notification.permission === 'granted') {
				try { new Notification(n.title, { body: n.message, icon: '/static/img/icons/icon-192x192.png', tag: 'gmd-' + n.id }); } catch (e) {}
			}
		});
		if (max > last || !last) { seen(Math.max(max, 1)); }
	}
	function load() {
		return fetch('/api/admin/account/notifications', { credentials: 'same-origin' })
			.then(function (r) { return r.ok ? r.json() : { items: [] }; })
			.then(function (d) { render(d.items || []); announce(d.items || []); })
			.catch(function () {});
	}
	document.addEventListener('DOMContentLoaded', function () {
		menu = document.getElementById('gmdBellMenu'); count = document.getElementById('gmdBellCount');
		if (!menu) { return; }
		menu.addEventListener('click', function (e) {
			var b = e.target.closest('[data-dismiss]');
			if (b) { e.preventDefault(); dismiss(+b.dataset.dismiss); return; }
			if (e.target.closest('[data-notify]')) { Notification.requestPermission().then(load); }
		});
		load();
		setInterval(load, 60000);
	});
})();`
