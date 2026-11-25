package api

import (
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// The Help page explains the pages every account uses (Library, Calendar,
// Add Media, My Account). It leaves out what the account can't do, so a
// viewing account isn't told about requests or downloads.

type helpItem struct {
	title, text string
	examples    [][2]string // what to type, what it finds
	address     string      // shown to copy (the apps' server addresses)
}

type helpSection struct {
	id, icon, title, intro string
	items                  []helpItem
}

// helpSections are the sections for an account; base is the web
// interface's address (scheme://host) for the apps' addresses.
func helpSections(acc *userAccess, base string) []helpSection {
	when := func(on bool, items ...helpItem) []helpItem {
		if on {
			return items
		}

		return nil
	}

	library := helpSection{id: "help-library", icon: "fa-photo-film", title: t("Library"),
		intro: t("The Library shows the movies, series, music, audiobooks and books of your media groups and plays them in the browser.")}
	library.items = append(library.items,
		helpItem{title: t("Media groups"), text: t("At the top you pick the media group. You see the groups the admin gave your account.")},
		helpItem{title: t("Search"), text: t("The search box looks in titles, artists, authors, the TV network, descriptions and translations in every language; for movies and series also in the cast. Parts of words count: star finds Starship too. Upper and lower case don't matter, except for umlauts and other letters beyond A to Z.")},
		helpItem{title: t("Combining words"), text: t("Words separated by spaces must all match, in any order. OR and AND only work in capitals; a lowercase or is searched as a word. AND binds before OR."),
			examples: [][2]string{
				{"star wars", t("both words, in any order")},
				{"star & wars", t("the same; & or AND joins words explicitly")},
				{"alien | predator", t("either of them; | or OR separates alternatives")},
				{`"star wars"`, t("exactly this phrase")},
				{"christmas -hallmark", t("christmas, but not hallmark; - excludes a word")},
				{`bravo hits | kuschelrock | "top 100"`, t("bravo and hits, or kuschelrock, or the phrase top 100")},
			}},
		helpItem{title: t("Actors"), text: t("Click an actor in the details of a title to see all titles with them: the chip above the grid shows the filter, its × shows everything again.")},
		helpItem{title: t("Filter and order"), text: t("Filter shows what is downloaded, missing or all, by genre, year, resolution and more. Order sorts by release, rating, recently added and more; the arrow button reverses it. ← and → turn the pages.")},
		helpItem{title: t("Views"), text: t("Covers, covers with descriptions or a list. Series can also be shown as single episodes, music by artist, and movies by collection, such as all movies of a film series.")},
		helpItem{title: t("Continue and Next up"), text: t("Continue lists what you started, Next up the next episode of the series you watch.")},
		helpItem{title: t("Details"), text: t("Click a cover for the details: description, cast, files, and the More tab with trailers and similar or recommended titles. Series list their episodes, albums and audiobooks their tracks: filter them by text or watched state and sort them.")},
		helpItem{title: t("Playback"), text: t("Play starts the player in the page, Shuffle plays in random order, and Play all plays everything the search finds. Your position is saved: the next time it continues there, on every device.")},
		helpItem{title: t("Player keys"), text: t("Space pauses, ← and → jump 10 seconds, N and P go to the next or previous entry, M mutes, F switches to full screen, I shows information about the file. Speed, subtitles, quality and the sleep timer are in the player's buttons.")},
		helpItem{title: t("Watched state"), text: t("A title played to the end counts as watched or heard. The eye button next to an entry marks it by hand; a green check shows it is watched, and clicking it takes the mark back.")},
		helpItem{title: t("Favorites, playlists and saved views"), text: t("Add to favorites and Add to a playlist are in the details of a title. Saved views keep the current search and filters under a name.")},
		helpItem{title: t("Books"), text: t("Read book opens EPUB and PDF books like an e-reader: tap or swipe the page’s left or right side, or use the arrow keys, to turn pages; the list button shows the contents, the bar below jumps. The page you are on is saved and opens again on every device.")},
		helpItem{title: t("Random"), text: t("Random opens a random entry that matches the search and filters.")},
	)
	library.items = append(library.items, when(acc.CanDownload(),
		helpItem{title: t("Download"), text: t("Download saves a file or a whole title to your device.")})...)
	library.items = append(library.items, when(acc.CanReport(),
		helpItem{title: t("Report a file"), text: t("Is a file wrong (bad quality, wrong language or content)? Report it in the player. The admin gets the report, and you get a message when it is handled.")})...)
	library.items = append(library.items, when(acc.CanSearch(),
		helpItem{title: t("Search at the indexers"), text: t("Search in the details looks for a missing title or episode at the indexers right away.")})...)

	calendar := helpSection{id: "help-calendar", icon: "fa-calendar-alt", title: t("Calendar"),
		intro: t("The Calendar shows the release dates of the movies, episodes, albums, audiobooks and books in your lists.")}
	calendar.items = append(calendar.items,
		helpItem{title: t("Views"), text: t("Month shows a month, Agenda a list of the coming dates. Today, Previous and Next move through time.")},
		helpItem{title: t("Filters"), text: t("Pick the media type, media group and genre, or show only what is still missing.")},
		helpItem{title: t("Marks"), text: t("New marks what came out since yesterday, Overdue what is released but still missing.")},
		helpItem{title: t("Details"), text: t("Click an entry for its details. Titles that are downloaded open in the Library from there.")},
		helpItem{title: t("Your own calendar"), text: t("Download .ics saves the entries shown as a calendar file for Outlook, Google Calendar or Apple Calendar.")},
	)
	calendar.items = append(calendar.items, when(acc.CanSearch(),
		helpItem{title: t("Search missing"), text: t("Search all missing of this day and Search overdue look for missing titles at the indexers, one after the other.")})...)

	account := helpSection{id: "help-account", icon: "fa-user", title: t("My Account"),
		intro: t("My Account holds your password, your e-mail address and the addresses for other apps.")}
	account.items = append(account.items,
		helpItem{title: t("Password"), text: t("Change your password there. Your other sessions are logged out.")},
		helpItem{title: t("Messages"), text: t("The bell at the top shows your messages: requests approved or declined, requested titles downloaded, reports handled. In My Account you choose per kind whether it also comes by mail or as a push message to your phone (ntfy, Pushover or Telegram, when the admin set one up).")},
		helpItem{title: t("Theme and language"), text: t("The account menu at the top right switches between light and dark and sets the language. Both are saved with your account.")},
	)
	account.items = append(account.items, when(acc.CanUseApps(),
		helpItem{title: t("Apps and devices"), text: t("Apps and devices lists the addresses for Jellyfin apps, WebDAV (VLC, Infuse), Subsonic, OPDS and DLNA. Log in there with the same name and password: watched state and playlists are the same as here.")})...)

	sections := []helpSection{library, calendar}

	if acc.CanRequest() {
		add := helpSection{id: "help-add", icon: "fa-plus", title: t("Add Media"),
			intro: t("Add Media finds movies, series, music, audiobooks and books at the metadata providers and adds them to your lists.")}
		add.items = append(add.items,
			helpItem{title: t("Find"), text: t("Pick the kind, then search by title or by ID (IMDb, TVDB, Audible ASIN). Titles already in a list of the media group are marked.")},
			helpItem{title: t("Add"), text: t("Choose the list and add the match. The title is then searched and downloaded by itself.")},
		)

		if acc.Admin() {
			add.items = append(add.items,
				helpItem{title: t("Whole catalogues and manual entries"), text: t("Artists and authors can be added with all or selected albums, books or audiobooks. Titles no provider knows can be added by hand with their details.")})
		} else {
			add.items = append(add.items,
				helpItem{title: t("Requests"), text: t("What you add is a request: the title is added to the list but only searched once an admin approves it. You get a message when it is approved or declined and when it is downloaded.")})
		}

		sections = append(sections, add)
	}

	sections = append(sections, account)

	if apps := helpApps(acc, base); apps != nil {
		sections = append(sections, *apps)
	}

	return sections
}

// helpApps is the guide per app for the protocols that are on; nil for an
// account without the apps feature.
func helpApps(acc *userAccess, base string) *helpSection {
	g := config.GetSettingsGeneral()
	if !acc.CanUseApps() || !(g.ServeJellyfin || g.ServeWebDAV || g.ServeSubsonic || g.ServeOPDS || g.ServeDLNA) {
		return nil
	}

	s := helpSection{id: "help-apps", icon: "fa-mobile-screen", title: t("Apps and devices"),
		intro: t("Your library in other apps: log in there with the same name and password as here. Watched state and playlists are the same everywhere.")}

	add := func(on bool, address string, items ...helpItem) {
		if !on {
			return
		}

		for i := range items {
			items[i].address = address
		}

		s.items = append(s.items, items...)
	}

	add(g.ServeJellyfin, base+"/jellyfin",
		helpItem{title: t("Jellyfin app (phone, TV, browser)"), text: t("Add a server, enter this address and log in with your name and password.")},
		helpItem{title: "Infuse", text: t("Settings → Add Files → Jellyfin: enter this address, your name and password.")},
		helpItem{title: "Swiftfin, Findroid", text: t("Connect to a server with this address, then log in. Audiobooks are albums there: one entry per book, its parts in order.")},
	)
	add(g.ServeWebDAV, base+"/dav",
		helpItem{title: t("VLC on the computer"), text: t("Media → Open Network Stream, enter this address and log in when asked. A folder plays as a playlist, its folders as playlists inside it.")},
		helpItem{title: t("VLC on iPhone and iPad, nPlayer, Infuse"), text: t("Add a WebDAV server with this address, your name and password, then browse the folders.")},
		helpItem{title: "macOS Finder", text: t("Go → Connect to Server (⌘K), enter this address and log in: the folders open like a drive.")},
		helpItem{title: "Windows Explorer", text: t("Map network drive with this address. Windows refuses files over 50 MB there: play videos with VLC instead.")},
	)
	add(g.ServeSubsonic, base+"/subsonic",
		helpItem{title: "play:Sub, Substreamer, Amperfy, Symfonium", text: t("Add a Subsonic server with this address, your name and password. Switch on the plain password (legacy) login in the app: the token login works only for the built-in admin.")},
	)
	add(g.ServeOPDS, base+"/opds",
		helpItem{title: "KOReader", text: t("Search → OPDS catalog → +: enter a name, this address, your name and password.")},
		helpItem{title: "Moon+ Reader, Librera, Thorium", text: t("Add an OPDS catalog with this address, your name and password. The catalog has your books by group and author, the newest ones and a search.")},
	)
	add(g.ServeDLNA, "",
		helpItem{title: t("TVs and players (DLNA)"), text: tf("Open the TV’s media player or sources and pick “%s”, without a login. It shows the media of the account the admin chose, and the TV has to be in the same network as the server.", serverName())},
		helpItem{title: t("VLC (DLNA)"), text: tf("Local Network → Universal Plug’n’Play → “%s”.", serverName())},
	)

	return &s
}

// helpExamples is the table of what to type and what it finds.
func helpExamples(examples [][2]string) gomponents.Node {
	if len(examples) == 0 {
		return nil
	}

	rows := make([]gomponents.Node, 0, len(examples))
	for _, e := range examples {
		rows = append(rows, html.Tr(html.Td(html.Code(gomponents.Text(e[0]))), html.Td(gomponents.Text(e[1]))))
	}

	return html.Table(html.Class("table table-sm mt-2 mb-0 w-auto"),
		html.THead(html.Tr(html.Th(gomponents.Text(t("You type"))), html.Th(gomponents.Text(t("It finds"))))),
		html.TBody(rows...))
}

// HelpPage explains the pages (GET /admin/help).
func HelpPage(c *gin.Context) {
	base := "http://" + c.Request.Host
	if requestIsHTTPS(c) {
		base = "https://" + c.Request.Host
	}

	sections := helpSections(accessOf(c), base)

	jump := make([]gomponents.Node, 0, len(sections))
	cards := make([]gomponents.Node, 0, len(sections))

	for _, s := range sections {
		jump = append(jump, html.A(html.Class("btn btn-sm btn-outline-primary"), html.Href("#"+s.id),
			html.I(html.Class("fa-solid "+s.icon+" me-1")), gomponents.Text(s.title)))

		items := make([]gomponents.Node, 0, len(s.items))
		for _, it := range s.items {
			items = append(items, html.Dt(html.Class("col-sm-3"), gomponents.Text(it.title)),
				html.Dd(html.Class("col-sm-9"),
					gomponents.If(it.address != "", html.Div(html.Class("mb-1"), html.Code(html.Class("user-select-all"), gomponents.Text(it.address)))),
					gomponents.Text(it.text), helpExamples(it.examples)))
		}

		cards = append(cards, html.Section(html.ID(s.id), html.Class("card mb-4"), html.Style("scroll-margin-top: 1rem"),
			html.Div(html.Class("card-body"),
				html.H4(html.Class("card-title"), html.I(html.Class("fa-solid "+s.icon+" me-2 text-primary")), gomponents.Text(s.title)),
				html.P(html.Class("text-muted"), gomponents.Text(s.intro)),
				html.Dl(html.Class("row mb-0"), gomponents.Group(items)))))
	}

	renderPageIn(c, t("Help"), false, html.Div(html.Class("config-section-enhanced"), html.Style("max-width: 60rem"),
		renderEnhancedPageHeader("fa-solid fa-circle-question", t("Help"), t("How the Library, the Calendar, Add Media and your account work.")),
		html.Nav(html.Class("d-flex flex-wrap gap-2 mb-4"), gomponents.Attr("aria-label", t("Sections")), gomponents.Group(jump)),
		gomponents.Group(cards)))
}
