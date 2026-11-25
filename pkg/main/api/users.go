package api

import (
	"encoding/json"
	"net/http"
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

// The user management page (/admin/users, admins only) and the own account
// page (/admin/account, everyone).

var featureLabels = map[string]string{
	database.FeatureSearch:        "Search and download releases",
	database.FeatureRequest:       "Request new media (an admin approves)",
	database.FeatureDownload:      "Download media files",
	database.FeatureRequestDelete: "Report files as wrong (quality, language, content)",
	database.FeatureApps:          "Use apps and devices (Jellyfin, WebDAV, Subsonic, OPDS)",
}

var roleLabels = map[string]string{
	database.RoleUser:           t("User: view and play"),
	database.RoleSearchingUser:  t("Searching user: view, play and search"),
	database.RoleRequestingUser: t("Requesting user: view, play and request media"),
	database.RoleAdmin:          t("Admin: everything"),
}

// UsersPage renders the user list and the add/edit form (?edit=<id>).
func UsersPage(c *gin.Context) {
	var edit *database.User

	if id, err := strconv.ParseUint(c.Query("edit"), 10, 64); err == nil {
		uid := uint(id)
		if u, err := database.GetUserByID(&uid); err == nil && u.ID != 0 {
			edit = u
		}
	}

	renderPageIn(c, "Users", false, html.Div(html.Class("config-section-enhanced"),
		renderEnhancedPageHeader("fa-solid fa-users", "Users",
			"Accounts for the web interface. Users see and play only the media groups you give them; admins see everything. "+
				"The built-in login (admin + web API key) always stays an admin."),
		html.Div(html.ID("usersContent"), renderUsersContent(getCSRFToken(c), edit, ""))))
}

func renderUsersContent(csrf string, edit *database.User, alert string) gomponents.Node {
	users := database.GetUsers()
	activity := database.UserActivities()

	rows := make([]gomponents.Node, 0, len(users))
	for i := range users {
		u := &users[i]
		groups := strings.Join(u.Groups(), ", ")

		if u.Role == database.RoleAdmin {
			groups = "all"
		} else if groups == "" {
			groups = "none"
		}

		id := strconv.FormatUint(uint64(u.ID), 10)
		rows = append(rows, html.Tr(
			html.Td(html.Strong(gomponents.Text(u.Username))),
			html.Td(gomponents.Text(roleLabels[u.Role]), userFeatureBadges(u)),
			html.Td(html.Class("small"), gomponents.Text(groups)),
			html.Td(html.Class("small text-muted"), gomponents.Text(orDash(shortTime(u.LastLogin)))),
			html.Td(html.Class("text-nowrap"), userActivityButtons(id, u.Username, activity[strings.ToLower(u.Username)])),
			html.Td(html.Class("text-end text-nowrap"),
				html.A(html.Class("btn btn-sm btn-outline-primary me-1"), html.Href("/api/admin/users?edit="+id),
					html.I(html.Class("fa-solid fa-pen me-1")), gomponents.Text("Edit")),
				html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-warning me-1"),
					gomponents.Attr("onclick", "usersResetPassword("+id+", "+jsString(u.Username)+")"),
					html.I(html.Class("fa-solid fa-key me-1")), gomponents.Text("Reset password")),
				html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-danger"),
					hx.Post("/api/admin/users/delete"), hx.Vals(`{"id": "`+id+`"}`), hx.Headers(wizardCSRFHeader(csrf)),
					hx.Target("#usersContent"), hx.Swap("innerHTML"),
					hx.Confirm("Delete the account "+u.Username+"? It is logged out at once."),
					html.I(html.Class("fa-solid fa-trash me-1")), gomponents.Text("Delete"))),
		))
	}

	list := gomponents.Node(html.P(html.Class("text-muted"), gomponents.Text("No accounts yet.")))
	if len(rows) > 0 {
		list = html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm align-middle"),
			html.THead(html.Tr(html.Th(gomponents.Text("Name")), html.Th(gomponents.Text("Role")),
				html.Th(gomponents.Text("Media groups")), html.Th(gomponents.Text("Last login")), html.Th(gomponents.Text("Activity")), html.Th())),
			html.TBody(gomponents.Group(rows))))
	}

	return html.Div(
		gomponents.Raw(alert),
		html.Div(html.Class("card mb-4"), html.Div(html.Class("card-body"), list)),
		renderUserForm(csrf, edit),
		userActivityModal(),
		html.Script(gomponents.Raw(`function usersActivity(id, what, title) {
	var modal = document.getElementById('usersActivityModal');
	modal.querySelector('.modal-title').textContent = title;
	var body = modal.querySelector('.modal-body');
	body.innerHTML = '<div class="text-center p-4"><div class="spinner-border text-primary" role="status"></div></div>';
	bootstrap.Modal.getOrCreateInstance(modal).show();
	fetch('/api/admin/users/activity?id=' + id + '&what=' + what, { credentials: 'same-origin' })
		.then(function (r) { return r.text().then(function (t) { if (!r.ok) { throw new Error(t || ('HTTP ' + r.status)); } return t; }); })
		.then(function (html) { body.innerHTML = html; })
		.catch(function (e) { body.innerHTML = '<div class="alert alert-danger mb-0"></div>'; body.firstChild.textContent = e.message; });
}
function usersResetPassword(id, name) {
	var pw = prompt('New password for ' + name + ' (at least 8 characters):');
	if (!pw) { return; }
	htmx.ajax('POST', '/api/admin/users/password', {target: '#usersContent', swap: 'innerHTML', values: {id: id, password: pw}});
}`)))
}

// shortTime shows a stored local time ("2026-11-15 09:43:33", which the
// driver may return as "2026-11-15T09:43:33Z") as "2026-11-15 09:43".
func shortTime(s string) string {
	s = strings.TrimSuffix(strings.Replace(s, "T", " ", 1), "Z")
	if len(s) >= 16 {
		return s[:16]
	}

	return s
}

func orDash(s string) string {
	if s == "" {
		return "–"
	}

	return s
}

// userFeatureBadges lists the features of a non-admin account.
func userFeatureBadges(u *database.User) gomponents.Node {
	if u.Role == database.RoleAdmin {
		return nil
	}

	nodes := make([]gomponents.Node, 0, 4)
	for _, f := range u.FeatureList() {
		nodes = append(nodes, html.Span(html.Class("badge text-bg-light border ms-1"), gomponents.Text(f)))
	}

	return gomponents.Group(nodes)
}

// renderUserForm is the add form, or the edit form of u.
func renderUserForm(csrf string, u *database.User) gomponents.Node {
	title, button := "Add an account", "Add"
	if u != nil {
		title, button = "Edit "+u.Username, "Save"
	} else {
		u = &database.User{Role: database.RoleUser}
	}

	roles := make([]gomponents.Node, 0, len(database.Roles))
	for _, r := range database.Roles {
		roles = append(roles, html.Option(html.Value(r), gomponents.If(r == u.Role, html.Selected()), gomponents.Text(roleLabels[r])))
	}

	features := make([]gomponents.Node, 0, len(database.Features))
	for _, f := range database.Features {
		id := "userFeature_" + f
		features = append(features, html.Div(html.Class("col-sm-6 col-xl-3"), html.Div(html.Class("form-check"),
			html.Input(html.Type("checkbox"), html.Class("form-check-input user-feature"), html.ID(id), html.Name("features"), html.Value(f),
				gomponents.If(slices.Contains(u.FeatureList(), f), html.Checked())),
			html.Label(html.Class("form-check-label"), html.For(id), gomponents.Text(featureLabels[f])))))
	}

	presets, _ := json.Marshal(database.RoleFeatures)

	groups := make([]gomponents.Node, 0, 8)

	for _, g := range libraryGroups() {
		id := "userGroup_" + g.Name
		groups = append(groups, html.Div(html.Class("col-sm-6 col-md-4 col-xl-3"), html.Div(html.Class("form-check"),
			html.Input(html.Type("checkbox"), html.Class("form-check-input"), html.ID(id), html.Name("groups"), html.Value(g.Name),
				gomponents.If(slices.ContainsFunc(u.Groups(), func(x string) bool { return strings.EqualFold(x, g.Name) }), html.Checked())),
			html.Label(html.Class("form-check-label"), html.For(id), gomponents.Text(g.Name)))))
	}

	return html.Div(html.Class("card"), html.Div(html.Class("card-body"),
		html.H5(html.Class("card-title"), gomponents.Text(title)),
		html.Form(html.ID("userForm"),
			hx.Post("/api/admin/users/save"), hx.Headers(wizardCSRFHeader(csrf)), hx.Target("#usersContent"), hx.Swap("innerHTML"),
			gomponents.If(u.ID != 0, html.Input(html.Type("hidden"), html.Name("id"), html.Value(strconv.FormatUint(uint64(u.ID), 10)))),
			html.Div(html.Class("row g-3"),
				html.Div(html.Class("col-md-4"),
					html.Label(html.Class("form-label"), html.For("userName"), gomponents.Text("Name")),
					html.Input(html.Type("text"), html.Class("form-control"), html.ID("userName"), html.Name("username"),
						html.Value(u.Username), html.Required(), gomponents.Attr("autocomplete", "off"))),
				html.Div(html.Class("col-md-4"),
					html.Label(html.Class("form-label"), html.For("userRole"), gomponents.Text("Role")),
					html.Select(html.Class("form-select"), html.ID("userRole"), html.Name("role"), gomponents.Group(roles),
						gomponents.Attr("onchange", "userRolePreset(this.value)"))),
				gomponents.If(u.ID == 0, html.Div(html.Class("col-md-4"),
					html.Label(html.Class("form-label"), html.For("userPassword"), gomponents.Text("Password")),
					html.Input(html.Type("password"), html.Class("form-control"), html.ID("userPassword"), html.Name("password"),
						html.MinLength("8"), html.Required(), gomponents.Attr("autocomplete", "new-password")),
					html.Div(html.Class("form-text"), gomponents.Text("At least 8 characters.")))),
				html.Div(html.Class("col-12"),
					html.Div(html.Class("form-label"), gomponents.Text("Media groups")),
					html.Div(html.Class("row g-1"), gomponents.Group(groups)),
					html.Div(html.Class("form-text"), gomponents.Text("What the account sees in the Library, Calendar and Add Media. Admins always see everything."))),
				html.Div(html.Class("col-12"),
					html.Div(html.Class("form-label"), gomponents.Text("Features")),
					html.Div(html.Class("row g-1"), gomponents.Group(features)),
					html.Div(html.Class("form-text"), gomponents.Text("Choosing a role ticks its features; switch single ones as needed. Admins may do everything."))),
			),
			html.Script(gomponents.Raw(`var userRoleFeatures = `+string(presets)+`;
function userRolePreset(role) {
	var on = userRoleFeatures[role] || [];
	document.querySelectorAll('#userForm .user-feature').forEach(function (c) { c.checked = on.indexOf(c.value) >= 0; });
}`)),
			html.Div(html.Class("mt-3"),
				html.Button(html.Type("submit"), html.Class("btn btn-primary me-2"), gomponents.Text(button)),
				gomponents.If(u.ID != 0, html.A(html.Class("btn btn-outline-secondary"), html.Href("/api/admin/users"), gomponents.Text("Cancel")))),
		)))
}

// usersReply re-renders the page content with an alert.
func usersReply(c *gin.Context, edit *database.User, msg, level string) {
	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, renderComponentToString(renderUsersContent(getCSRFToken(c), edit, renderAlert(msg, level))))
}

// HandleUserSave creates or updates an account (/admin/users/save).
func HandleUserSave(c *gin.Context) {
	u := database.User{
		Username:    strings.TrimSpace(c.PostForm("username")),
		Role:        c.PostForm("role"),
		MediaGroups: strings.Join(c.PostFormArray("groups"), ","),
		Features:    strings.Join(c.PostFormArray("features"), ","),
	}

	if id, err := strconv.ParseUint(c.PostForm("id"), 10, 64); err == nil {
		u.ID = uint(id)
	}

	if strings.EqualFold(u.Username, DefaultUsername) {
		usersReply(c, &u, database.ErrUserReserved.Error(), "danger")
		return
	}

	if u.ID == 0 {
		if err := database.InsertUser(&u, c.PostForm("password")); err != nil {
			usersReply(c, nil, "Not added: "+err.Error(), "danger")
			return
		}

		usersReply(c, nil, "Account "+u.Username+" added.", "success")

		return
	}

	old, err := database.GetUserByID(&u.ID)
	if err != nil || old.ID == 0 {
		usersReply(c, nil, "The account no longer exists.", "warning")
		return
	}

	if err := database.UpdateUser(&u); err != nil {
		usersReply(c, &u, "Not saved: "+err.Error(), "danger")
		return
	}

	// Sessions carry name, role and groups: log the account out.
	sessionStore.deleteUserSessions(old.Username)
	usersReply(c, nil, "Account "+u.Username+" saved. It has to log in again.", "success")
}

// HandleUserDelete deletes an account and ends its sessions (/admin/users/delete).
func HandleUserDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.PostForm("id"), 10, 64)
	if err != nil {
		usersReply(c, nil, "Unknown account.", "warning")
		return
	}

	uid := uint(id)

	u, err := database.GetUserByID(&uid)
	if err != nil || u.ID == 0 {
		usersReply(c, nil, "The account no longer exists.", "warning")
		return
	}

	if err := database.DeleteUser(&uid); err != nil {
		usersReply(c, nil, "Not deleted: "+err.Error(), "danger")
		return
	}

	sessionStore.deleteUserSessions(u.Username)
	usersReply(c, nil, "Account "+u.Username+" deleted.", "success")
}

// HandleUserPassword sets a new password for an account and ends its
// sessions; its remembered logins stop working too (/admin/users/password).
func HandleUserPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.PostForm("id"), 10, 64)
	if err != nil {
		usersReply(c, nil, "Unknown account.", "warning")
		return
	}

	uid := uint(id)

	u, err := database.GetUserByID(&uid)
	if err != nil || u.ID == 0 {
		usersReply(c, nil, "The account no longer exists.", "warning")
		return
	}

	if err := database.SetUserPassword(&uid, c.PostForm("password")); err != nil {
		usersReply(c, nil, "Password not changed: "+err.Error(), "danger")
		return
	}

	sessionStore.deleteUserSessions(u.Username)
	usersReply(c, nil, "New password set for "+u.Username+". It has to log in again.", "success")
}

// AccountPage lets an account change its own password (/admin/account).
func AccountPage(c *gin.Context) {
	acc := accessOf(c)

	var body gomponents.Node

	if acc.Name == DefaultUsername {
		body = gomponents.Raw(renderAlert(t("This is the built-in admin login. Its password is the web API key: change it under Configuration → General."), "info"))
	} else {
		body = html.Div(html.Div(html.ID("accountContent"), renderAccountForm(getCSRFToken(c), "")),
			gomponents.If(providers.MailNotification() != nil, renderEmailForm(getCSRFToken(c), database.GetUserEmail(acc.Name), "")),
			renderMessagesForm(getCSRFToken(c), acc, ""))
	}

	renderPageIn(c, t("My Account"), false, html.Div(html.Class("config-section-enhanced"),
		renderEnhancedPageHeader("fa-solid fa-user", t("My Account"), tf("Signed in as %s (%s).", acc.Name, roleLabels[acc.Role])),
		body, renderAccountApps(c, acc)))
}

func renderAccountForm(csrf, alert string) gomponents.Node {
	field := func(name, label string) gomponents.Node {
		return html.Div(html.Class("mb-3"),
			html.Label(html.Class("form-label"), html.For("acc_"+name), gomponents.Text(label)),
			html.Input(html.Type("password"), html.Class("form-control"), html.ID("acc_"+name), html.Name(name), html.Required(),
				gomponents.Attr("autocomplete", map[bool]string{true: "current-password", false: "new-password"}[name == "current"])))
	}

	return html.Div(gomponents.Raw(alert),
		html.Div(html.Class("card"), html.Style("max-width: 32rem"), html.Div(html.Class("card-body"),
			html.H5(html.Class("card-title"), gomponents.Text(t("Change password"))),
			html.Form(hx.Post("/api/admin/account"), hx.Headers(wizardCSRFHeader(csrf)), hx.Target("#accountContent"), hx.Swap("innerHTML"),
				field("current", t("Current password")),
				field("password", t("New password (at least 8 characters)")),
				field("confirm", t("New password again")),
				html.Button(html.Type("submit"), html.Class("btn btn-primary"), gomponents.Text(t("Change password")))))))
}

// HandleAccountPassword changes the own password (POST /admin/account). The
// account stays logged in here; other sessions end.
func HandleAccountPassword(c *gin.Context) {
	acc := accessOf(c)
	reply := func(msg, level string) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, renderComponentToString(renderAccountForm(getCSRFToken(c), renderAlert(msg, level))))
	}

	u, err := database.GetUserByName(&acc.Name)
	if err != nil || u.ID == 0 {
		reply(t("The built-in admin's password is the web API key (Configuration → General)."), "info")
		return
	}

	if !database.CheckPassword(u.PasswordHash, c.PostForm("current")) {
		reply(t("The current password is wrong."), "danger")
		return
	}

	if c.PostForm("password") != c.PostForm("confirm") {
		reply(t("The new passwords differ."), "danger")
		return
	}

	if err := database.SetUserPassword(&u.ID, c.PostForm("password")); err != nil {
		reply(tf("Not changed: %s", t(err.Error())), "danger")
		return
	}

	// Remembered logins of the old password stop working; this browser's
	// session goes on.
	keep, _ := c.Cookie("session_id")

	sessionStore.mutex.Lock()
	for id, s := range sessionStore.sessions {
		if id != keep && strings.EqualFold(s.UserID, u.Username) {
			delete(sessionStore.sessions, id)
		}
	}
	sessionStore.mutex.Unlock()

	setRememberCookie(c, "", -1)
	reply(t("Password changed."), "success")
}
