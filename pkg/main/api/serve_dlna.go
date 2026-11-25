package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"hash/crc32"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// DLNA (/dlna) is a UPnP media server for TVs and other players in the
// local network: the device description, the ContentDirectory (browse
// the media groups of the dlna_user account) and the files. DLNA has no
// login, so only local requests are answered (dlnaGuard). The SSDP
// announcements are in serve_dlna_ssdp.go.
//
// Object IDs: "0" the root, "g/<crc of group>", "s/<series>",
// "se/<series>/<season>", "ar/<crc of group>/<artist>", "al/<album>",
// "b/<audiobook>" are folders; "m/<movie>", "e/<episode>", "t/<file>",
// "bp/<file>" items.

const dlnaFlags = "DLNA.ORG_OP=01;DLNA.ORG_CI=0;DLNA.ORG_FLAGS=01700000000000000000000000000000"

// dlnaUUID is the server's stable device ID.
func dlnaUUID() string {
	sum := sha256.Sum256(append([]byte("dlna-device"), rememberServerSecret()...))
	h := hex.EncodeToString(sum[:16])

	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// dlnaAccess is the account whose groups DLNA shows.
func dlnaAccess() *userAccess {
	name := strings.TrimSpace(config.GetSettingsGeneral().DLNAUser)
	if name == "" || strings.EqualFold(name, DefaultUsername) {
		return adminAccess
	}

	u, err := database.GetUserByName(&name)
	if err != nil || u.ID == 0 {
		return &userAccess{Name: name}
	}

	return accessOfUser(u)
}

// dlnaGuard answers local requests only, as the dlna_user account.
func dlnaGuard(c *gin.Context) {
	if !localRequest(c.Request) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	setClientSession(c, dlnaAccess())
	c.Next()
}

// HandleDLNADevice is the device description.
func HandleDLNADevice(c *gin.Context) {
	c.Header("Cache-Control", "max-age=1800")
	c.Data(http.StatusOK, `text/xml; charset="utf-8"`, []byte(xml.Header+
		`<root xmlns="urn:schemas-upnp-org:device-1-0" xmlns:dlna="urn:schemas-dlna-org:device-1-0">`+
		`<specVersion><major>1</major><minor>0</minor></specVersion><device>`+
		`<deviceType>urn:schemas-upnp-org:device:MediaServer:1</deviceType>`+
		`<friendlyName>`+xmlEscape(serverName())+`</friendlyName>`+
		`<manufacturer>go_media_downloader</manufacturer><modelName>Media Downloader</modelName><modelNumber>1</modelNumber>`+
		`<UDN>uuid:`+dlnaUUID()+`</UDN><dlna:X_DLNADOC>DMS-1.50</dlna:X_DLNADOC>`+
		`<iconList><icon><mimetype>image/png</mimetype><width>48</width><height>48</height><depth>24</depth>`+
		`<url>/dlna/icon.png</url></icon></iconList><serviceList>`+
		`<service><serviceType>urn:schemas-upnp-org:service:ContentDirectory:1</serviceType>`+
		`<serviceId>urn:upnp-org:serviceId:ContentDirectory</serviceId><SCPDURL>/dlna/cd.xml</SCPDURL>`+
		`<controlURL>/dlna/cd/control</controlURL><eventSubURL>/dlna/cd/event</eventSubURL></service>`+
		`<service><serviceType>urn:schemas-upnp-org:service:ConnectionManager:1</serviceType>`+
		`<serviceId>urn:upnp-org:serviceId:ConnectionManager</serviceId><SCPDURL>/dlna/cm.xml</SCPDURL>`+
		`<controlURL>/dlna/cm/control</controlURL><eventSubURL>/dlna/cm/event</eventSubURL></service>`+
		`</serviceList></device></root>`))
}

// scpd builds a service description from its actions ("Name:in Arg
// Variable,out Arg Variable") and state variables ("Name type [events]").
func scpd(actions []string, vars []string) []byte {
	var b strings.Builder

	b.WriteString(xml.Header + `<scpd xmlns="urn:schemas-upnp-org:service-1-0"><specVersion><major>1</major><minor>0</minor></specVersion><actionList>`)

	for _, a := range actions {
		name, args, _ := strings.Cut(a, ":")
		b.WriteString("<action><name>" + name + "</name><argumentList>")

		for arg := range strings.SplitSeq(args, ",") {
			f := strings.Fields(arg)
			if len(f) == 3 {
				b.WriteString("<argument><name>" + f[1] + "</name><direction>" + f[0] + "</direction><relatedStateVariable>" +
					f[2] + "</relatedStateVariable></argument>")
			}
		}

		b.WriteString("</argumentList></action>")
	}

	b.WriteString("</actionList><serviceStateTable>")

	for _, v := range vars {
		f := strings.Fields(v)
		events := "no"

		if len(f) > 2 && f[2] == "events" {
			events = "yes"
		}

		b.WriteString(`<stateVariable sendEvents="` + events + `"><name>` + f[0] + "</name><dataType>" + f[1] + "</dataType></stateVariable>")
	}

	b.WriteString("</serviceStateTable></scpd>")

	return []byte(b.String())
}

var (
	dlnaCDSCPD = scpd([]string{
		"GetSearchCapabilities:out SearchCaps SearchCapabilities",
		"GetSortCapabilities:out SortCaps SortCapabilities",
		"GetSystemUpdateID:out Id SystemUpdateID",
		"Browse:in ObjectID A_ARG_TYPE_ObjectID,in BrowseFlag A_ARG_TYPE_BrowseFlag,in Filter A_ARG_TYPE_Filter," +
			"in StartingIndex A_ARG_TYPE_Index,in RequestedCount A_ARG_TYPE_Count,in SortCriteria A_ARG_TYPE_SortCriteria," +
			"out Result A_ARG_TYPE_Result,out NumberReturned A_ARG_TYPE_Count,out TotalMatches A_ARG_TYPE_Count," +
			"out UpdateID A_ARG_TYPE_UpdateID",
	}, []string{
		"SearchCapabilities string", "SortCapabilities string", "SystemUpdateID ui4 events", "A_ARG_TYPE_ObjectID string",
		"A_ARG_TYPE_BrowseFlag string", "A_ARG_TYPE_Filter string", "A_ARG_TYPE_Index ui4", "A_ARG_TYPE_Count ui4",
		"A_ARG_TYPE_SortCriteria string", "A_ARG_TYPE_Result string", "A_ARG_TYPE_UpdateID ui4",
	})
	dlnaCMSCPD = scpd([]string{
		"GetProtocolInfo:out Source SourceProtocolInfo,out Sink SinkProtocolInfo",
		"GetCurrentConnectionIDs:out ConnectionIDs CurrentConnectionIDs",
		"GetCurrentConnectionInfo:in ConnectionID A_ARG_TYPE_ConnectionID,out RcsID A_ARG_TYPE_RcsID," +
			"out AVTransportID A_ARG_TYPE_AVTransportID,out ProtocolInfo A_ARG_TYPE_ProtocolInfo," +
			"out PeerConnectionManager A_ARG_TYPE_ConnectionManager,out PeerConnectionID A_ARG_TYPE_ConnectionID," +
			"out Direction A_ARG_TYPE_Direction,out Status A_ARG_TYPE_ConnectionStatus",
	}, []string{
		"SourceProtocolInfo string events", "SinkProtocolInfo string events", "CurrentConnectionIDs string events",
		"A_ARG_TYPE_ConnectionID i4", "A_ARG_TYPE_RcsID i4", "A_ARG_TYPE_AVTransportID i4", "A_ARG_TYPE_ProtocolInfo string",
		"A_ARG_TYPE_ConnectionManager string", "A_ARG_TYPE_Direction string", "A_ARG_TYPE_ConnectionStatus string",
	})
)

func HandleDLNACDSCPD(c *gin.Context) { c.Data(http.StatusOK, `text/xml; charset="utf-8"`, dlnaCDSCPD) }
func HandleDLNACMSCPD(c *gin.Context) { c.Data(http.StatusOK, `text/xml; charset="utf-8"`, dlnaCMSCPD) }

// HandleDLNAEvent accepts event subscriptions; the content never changes
// in a way worth an event, so none are sent.
func HandleDLNAEvent(c *gin.Context) {
	if c.Request.Method == "SUBSCRIBE" {
		sid := c.GetHeader("SID")
		if sid == "" {
			sid = "uuid:" + generateSecureToken(16)
		}

		c.Header("SID", sid)
		c.Header("TIMEOUT", "Second-1800")
	}

	c.Status(http.StatusOK)
}

// soapArgs reads the arguments of a SOAP action: its name and the values
// of its child elements.
func soapArgs(r io.Reader) (string, map[string]string) {
	dec := xml.NewDecoder(io.LimitReader(r, 64<<10))
	args := map[string]string{}
	action, depth, current := "", 0, ""

	for {
		tok, err := dec.Token()
		if err != nil {
			return action, args
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++

			switch {
			case depth == 3:
				action = t.Name.Local
			case depth == 4:
				current = t.Name.Local
			}
		case xml.CharData:
			if depth == 4 && current != "" {
				args[current] += string(t)
			}
		case xml.EndElement:
			depth--
		}
	}
}

func soapReply(c *gin.Context, service, action, body string) {
	c.Header("EXT", "")
	c.Data(http.StatusOK, `text/xml; charset="utf-8"`, []byte(xml.Header+
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">`+
		`<s:Body><u:`+action+`Response xmlns:u="urn:schemas-upnp-org:service:`+service+`:1">`+body+
		`</u:`+action+`Response></s:Body></s:Envelope>`))
}

func soapFault(c *gin.Context, code int, desc string) {
	c.Data(http.StatusInternalServerError, `text/xml; charset="utf-8"`, []byte(xml.Header+
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">`+
		`<s:Body><s:Fault><faultcode>s:Client</faultcode><faultstring>UPnPError</faultstring><detail>`+
		`<UPnPError xmlns="urn:schemas-upnp-org:control-1-0"><errorCode>`+strconv.Itoa(code)+`</errorCode>`+
		`<errorDescription>`+desc+`</errorDescription></UPnPError></detail></s:Fault></s:Body></s:Envelope>`))
}

// dlnaProtocols are the formats DLNA lists as served.
var dlnaProtocols = []string{
	"video/x-matroska", "video/mp4", "video/x-msvideo", "video/mp2t", "video/quicktime", "video/webm", "video/mpeg",
	"audio/mpeg", "audio/flac", "audio/mp4", "audio/ogg", "audio/wav", "image/jpeg",
}

// HandleDLNACMControl answers the ConnectionManager actions.
func HandleDLNACMControl(c *gin.Context) {
	action, _ := soapArgs(c.Request.Body)

	switch action {
	case "GetProtocolInfo":
		src := make([]string, len(dlnaProtocols))
		for i, p := range dlnaProtocols {
			src[i] = "http-get:*:" + p + ":*"
		}

		soapReply(c, "ConnectionManager", action, "<Source>"+strings.Join(src, ",")+"</Source><Sink></Sink>")
	case "GetCurrentConnectionIDs":
		soapReply(c, "ConnectionManager", action, "<ConnectionIDs>0</ConnectionIDs>")
	case "GetCurrentConnectionInfo":
		soapReply(c, "ConnectionManager", action, "<RcsID>-1</RcsID><AVTransportID>-1</AVTransportID><ProtocolInfo></ProtocolInfo>"+
			"<PeerConnectionManager></PeerConnectionManager><PeerConnectionID>-1</PeerConnectionID><Direction>Output</Direction>"+
			"<Status>OK</Status>")
	default:
		soapFault(c, 401, "Invalid Action")
	}
}

// HandleDLNACDControl answers the ContentDirectory actions.
func HandleDLNACDControl(c *gin.Context) {
	action, args := soapArgs(c.Request.Body)

	switch action {
	case "GetSearchCapabilities":
		soapReply(c, "ContentDirectory", action, "<SearchCaps></SearchCaps>")
	case "GetSortCapabilities":
		soapReply(c, "ContentDirectory", action, "<SortCaps></SortCaps>")
	case "GetSystemUpdateID":
		soapReply(c, "ContentDirectory", action, "<Id>1</Id>")
	case "Browse":
		start, _ := strconv.Atoi(strings.TrimSpace(args["StartingIndex"]))
		count, _ := strconv.Atoi(strings.TrimSpace(args["RequestedCount"]))

		d := &dlnaBrowser{c: c, base: "http://" + c.Request.Host, groups: groupsOf(accessOf(c), "movie", "series", "album", "audiobook"),
			lang: requestLanguage(c)}

		var (
			total int
			ok    bool
		)

		id := strings.TrimSpace(args["ObjectID"])
		if strings.TrimSpace(args["BrowseFlag"]) == "BrowseMetadata" {
			ok = d.metadata(id)
			total = 1
		} else {
			total, ok = d.children(id, max(start, 0), max(count, 0))
		}

		if !ok {
			soapFault(c, 701, "No such object")
			return
		}

		soapReply(c, "ContentDirectory", action, "<Result>"+xmlEscape(d.didl())+"</Result><NumberReturned>"+strconv.Itoa(d.n)+
			"</NumberReturned><TotalMatches>"+strconv.Itoa(total)+"</TotalMatches><UpdateID>1</UpdateID>")
	default:
		soapFault(c, 401, "Invalid Action")
	}
}

// dlnaBrowser collects the DIDL-Lite objects of a Browse answer.
type dlnaBrowser struct {
	c      *gin.Context
	base   string
	groups []libGroup
	lang   string
	b      strings.Builder
	n      int
}

func (d *dlnaBrowser) didl() string {
	return `<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" ` +
		`xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:dlna="urn:schemas-dlna-org:metadata-1-0/">` + d.b.String() + `</DIDL-Lite>`
}

func dlnaGroupKey(name string) string {
	return strconv.FormatUint(uint64(crc32.ChecksumIEEE([]byte(name))), 16)
}

func (d *dlnaBrowser) group(key string) *libGroup {
	for i := range d.groups {
		if dlnaGroupKey(d.groups[i].Name) == key {
			return &d.groups[i]
		}
	}

	return nil
}

func (d *dlnaBrowser) container(id, parent, title, class string, children int, art string) {
	d.n++
	d.b.WriteString(`<container id="` + xmlEscape(id) + `" parentID="` + xmlEscape(parent) + `" restricted="1" searchable="0" childCount="` +
		strconv.Itoa(children) + `"><dc:title>` + xmlEscape(title) + `</dc:title><upnp:class>` + class + `</upnp:class>`)

	if art != "" {
		d.b.WriteString(`<upnp:albumArtURI dlna:profileID="JPEG_TN">` + xmlEscape(d.base+art) + `</upnp:albumArtURI>`)
	}

	d.b.WriteString(`</container>`)
}

// item writes a playable object with a resource per file.
func (d *dlnaBrowser) item(id, parent, class string, it *jfItem, r *jfRow, playKind, art string) {
	d.n++
	d.b.WriteString(`<item id="` + xmlEscape(id) + `" parentID="` + xmlEscape(parent) + `" restricted="1"><dc:title>` +
		xmlEscape(it.Name) + `</dc:title><upnp:class>` + class + `</upnp:class>`)

	if r.Year > 0 {
		d.b.WriteString(`<dc:date>` + strconv.Itoa(r.Year) + `-01-01</dc:date>`)
	}

	if it.Overview != "" {
		d.b.WriteString(`<dc:description>` + xmlEscape(it.Overview) + `</dc:description>`)
	}

	for _, g := range it.Genres {
		d.b.WriteString(`<upnp:genre>` + xmlEscape(g) + `</upnp:genre>`)
	}

	if r.Artist != "" {
		d.b.WriteString(`<upnp:artist>` + xmlEscape(r.Artist) + `</upnp:artist><dc:creator>` + xmlEscape(r.Artist) + `</dc:creator>`)
	}

	if it.Album != "" {
		d.b.WriteString(`<upnp:album>` + xmlEscape(it.Album) + `</upnp:album>`)
	}

	if it.IndexNumber != nil && *it.IndexNumber > 0 {
		tag := "upnp:originalTrackNumber"
		if playKind == "episode" {
			tag = "upnp:episodeNumber"
		}

		d.b.WriteString(`<` + tag + `>` + strconv.Itoa(*it.IndexNumber) + `</` + tag + `>`)
	}

	if art != "" {
		d.b.WriteString(`<upnp:albumArtURI dlna:profileID="JPEG_TN">` + xmlEscape(d.base+art) + `</upnp:albumArtURI>`)
	}

	for _, f := range jfFilesOf(it.ID) {
		ct := mediaContentType(f.Location)
		if ct == "" {
			ct = "application/octet-stream"
		}

		d.b.WriteString(`<res protocolInfo="http-get:*:` + ct + `:` + dlnaFlags + `"`)

		if f.Size > 0 {
			d.b.WriteString(` size="` + strconv.FormatInt(f.Size, 10) + `"`)
		}

		if r.Runtime > 0 {
			d.b.WriteString(` duration="` + dlnaDuration(r.Runtime) + `"`)
		}

		name := path.Base(strings.ReplaceAll(f.Location, "\\", "/"))
		d.b.WriteString(`>` + xmlEscape(d.base+"/dlna/res/"+f.Kind+"/"+strconv.FormatUint(f.ID, 10)+"/"+url.PathEscape(name)) + `</res>`)
	}

	d.b.WriteString(`</item>`)
}

// dlnaDuration formats seconds as H:MM:SS.mmm.
func dlnaDuration(s float64) string {
	ms := int64(s * 1000)
	h, m, sec := ms/3600000, ms/60000%60, ms/1000%60

	return strconv.FormatInt(h, 10) + ":" + twoDigits(int(m)) + ":" + twoDigits(int(sec)) + "." + strconv.FormatInt(1000+ms%1000, 10)[1:]
}

// rows runs an item query of the browser's groups and returns the
// translated items.
func (d *dlnaBrowser) rows(code byte, groups []libGroup, cond string, args []any, order string, start, count int) ([]jfRow, []jfItem, int) {
	k := jfKinds[code]

	rows, total := jfRows(k, jfQuery{Groups: groups, Cond: cond, Args: args, Order: order, Start: start, Limit: count})

	return rows, jfItems(k, rows, "", d.lang), total
}

func (d *dlnaBrowser) cover(typ string, db uint64) string {
	if db == 0 {
		return ""
	}

	return "/dlna/cover/" + typ + "/" + strconv.FormatUint(db, 10) + ".jpg"
}

// emit writes rows of a kind as objects under parent.
func (d *dlnaBrowser) emit(code byte, parent string, rows []jfRow, items []jfItem) {
	for i := range rows {
		r, it := &rows[i], &items[i]
		n := strconv.FormatUint(r.ID, 10)

		switch code {
		case jfMovie:
			d.item("m/"+n, parent, "object.item.videoItem.movie", it, r, "movie", d.cover("movie", r.DbID))
		case jfSeries:
			d.container("s/"+n, parent, it.Name, "object.container.storageFolder", len(jfSeasons(d.groups, r.ID)), d.cover("series", r.DbID))
		case jfEpisode:
			title := it.Name
			if r.Idx > 0 {
				title = "S" + twoDigits(r.Pidx) + "E" + twoDigits(r.Idx) + " " + title
			}

			it.Name = title
			d.item("e/"+n, parent, "object.item.videoItem", it, r, "episode", d.cover("series", r.Db2))
		case jfArtist:
			d.container("ar/"+strings.TrimPrefix(parent, "g/")+"/"+n, parent, it.Name, "object.container.person.musicArtist", r.Children, "")
		case jfAlbum:
			d.container("al/"+n, parent, it.Name, "object.container.album.musicAlbum", r.Children, d.cover("album", r.DbID))
		case jfTrack:
			d.item("t/"+n, parent, "object.item.audioItem.musicTrack", it, r, "track", d.cover("album", r.Db2))
		case jfBook:
			d.container("b/"+n, parent, it.Name, "object.container.album", r.Children, d.cover("audiobook", r.DbID))
		case jfBookPart:
			d.item("bp/"+n, parent, "object.item.audioItem.audioBook", it, r, "audiobook", d.cover("audiobook", r.Db2))
		}
	}
}

// children writes the objects in a folder (a page of them) and returns
// their total.
func (d *dlnaBrowser) children(id string, start, count int) (int, bool) {
	parts := strings.Split(id, "/")
	num := func(i int) uint64 {
		if i >= len(parts) {
			return 0
		}

		n, _ := strconv.ParseUint(parts[i], 10, 64)

		return n
	}

	switch parts[0] {
	case "0":
		playlists := database.GetPlaylists(accName(accessOf(d.c)))

		total := len(d.groups)
		if len(playlists) > 0 {
			total++
		}

		end := total
		if count > 0 {
			end = min(start+count, end)
		}

		for i := min(start, end); i < end; i++ {
			if i == len(d.groups) {
				d.container("pl", "0", trText(d.lang, t("Playlists")), "object.container.storageFolder", len(playlists), "")
				continue
			}

			g := &d.groups[i]
			_, n := jfRows(jfKinds[map[string]byte{"movie": jfMovie, "series": jfSeries, "album": jfArtist, "audiobook": jfBook}[g.Kind.Type]],
				jfQuery{Groups: []libGroup{*g}, Limit: 1})
			d.container("g/"+dlnaGroupKey(g.Name), "0", groupTitle(g, d.lang), "object.container.storageFolder", n, "")
		}

		return total, true
	case "pl":
		user := accName(accessOf(d.c))
		playlists, views := database.GetPlaylists(user), savedViews(user)

		total := len(playlists) + len(views)

		end := total
		if count > 0 {
			end = min(start+count, end)
		}

		for i := min(start, end); i < end; i++ {
			if i >= len(playlists) {
				v := &views[i-len(playlists)]
				d.container("sv/"+strconv.FormatUint(smartID(v), 10), "pl", v.Name, "object.container.playlistContainer", 0, "")

				continue
			}

			p := &playlists[i]
			d.container("p/"+strconv.FormatUint(uint64(p.ID), 10), "pl", p.Name, "object.container.playlistContainer", p.Items, "")
		}

		return total, true
	case "sv":
		v, ok := smartView(accName(accessOf(d.c)), num(1))
		if !ok {
			return 0, false
		}

		code, rows := smartRows(accessOf(d.c), d.lang, v)
		if code == 0 {
			return 0, true
		}

		end := len(rows)
		if count > 0 {
			end = min(start+count, end)
		}

		page := rows[min(start, end):end]
		d.emit(code, id, page, jfItems(jfKinds[code], page, "", d.lang))

		return len(rows), true
	case "p":
		pid := uint(num(1))
		if p, err := database.GetPlaylist(accName(accessOf(d.c)), &pid); err != nil || p.ID == 0 {
			return 0, false
		}

		entries := playlistEntries(accessOf(d.c), d.groups, pid, d.lang)

		end := len(entries)
		if count > 0 {
			end = min(start+count, end)
		}

		for i := min(start, end); i < end; i++ {
			e := &entries[i]
			d.emit(e.Code, id, []jfRow{e.Row}, []jfItem{e.Item})
		}

		return len(entries), true
	case "g":
		g := d.group(parts[1])
		if g == nil {
			return 0, false
		}

		code := map[string]byte{"movie": jfMovie, "series": jfSeries, "album": jfArtist, "audiobook": jfBook}[g.Kind.Type]
		rows, items, total := d.rows(code, []libGroup{*g}, "", nil, "", start, count)
		d.emit(code, id, rows, items)

		return total, true
	case "s":
		seasons := jfSeasons(d.groups, num(1))
		if len(seasons) == 0 {
			return 0, false
		}

		end := len(seasons)
		if count > 0 {
			end = min(start+count, end)
		}

		for i := min(start, end); i < end; i++ {
			s := &seasons[i]
			d.container("se/"+parts[1]+"/"+strconv.Itoa(s.Season), id, jfSeasonName(s.Season), "object.container.storageFolder", s.Files,
				d.cover("series", s.DbID))
		}

		return len(seasons), true
	case "se":
		rows, items, total := d.rows(jfEpisode, d.groups, " AND x.parent_id = ? AND x.pidx = ?", []any{num(1), num(2)}, "x.idx", start, count)
		d.emit(jfEpisode, id, rows, items)

		return total, true
	case "ar":
		g := d.group(parts[1])
		if g == nil {
			return 0, false
		}

		rows, items, total := d.rows(jfAlbum, []libGroup{*g}, " AND x.artist_id = ?", []any{num(2)}, "x.year, x.name", start, count)
		d.emit(jfAlbum, id, rows, items)

		return total, true
	case "al":
		rows, items, total := d.rows(jfTrack, d.groups, " AND x.parent_id = ?", []any{num(1)}, "x.pidx, x.idx, x.loc", start, count)
		d.emit(jfTrack, id, rows, items)

		return total, true
	case "b":
		rows, items, total := d.rows(jfBookPart, d.groups, " AND x.parent_id = ?", []any{num(1)}, "x.pidx, x.idx, x.loc", start, count)
		d.emit(jfBookPart, id, rows, items)

		return total, true
	}

	return 0, false
}

// metadata writes the object itself (BrowseMetadata).
func (d *dlnaBrowser) metadata(id string) bool {
	parts := strings.Split(id, "/")

	one := func(code byte, parent string, n string) bool {
		rows, items, _ := d.rows(code, d.groups, " AND x.id = ?", []any{n}, "", 0, 1)
		if len(rows) == 0 {
			return false
		}

		d.emit(code, parent, rows, items)

		return true
	}

	switch {
	case id == "0":
		d.container("0", "-1", serverName(), "object.container.storageFolder", len(d.groups), "")
		return true
	case parts[0] == "g" && len(parts) == 2:
		if g := d.group(parts[1]); g != nil {
			d.container(id, "0", groupTitle(g, d.lang), "object.container.storageFolder", 0, "")
			return true
		}
	case parts[0] == "se" && len(parts) == 3:
		n, _ := strconv.Atoi(parts[2])
		d.container(id, "s/"+parts[1], jfSeasonName(n), "object.container.storageFolder", 0, "")

		return true
	case parts[0] == "ar" && len(parts) == 3:
		rows, items, _ := d.rows(jfArtist, d.groups, " AND x.id = ?", []any{parts[2]}, "", 0, 1)
		if len(rows) > 0 {
			d.container(id, "g/"+parts[1], items[0].Name, "object.container.person.musicArtist", rows[0].Children, "")
			return true
		}
	case len(parts) == 2:
		code := map[string]byte{"m": jfMovie, "s": jfSeries, "e": jfEpisode, "al": jfAlbum, "t": jfTrack, "b": jfBook, "bp": jfBookPart}[parts[0]]
		if code != 0 {
			return one(code, "0", parts[1])
		}
	}

	return false
}

// HandleDLNAResource sends a file (/dlna/res/:kind/:id/*name).
func HandleDLNAResource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.Header("transferMode.dlna.org", "Streaming")
	c.Header("contentFeatures.dlna.org", dlnaFlags)
	serveMediaFile(c, c.Param("kind"), id)
}

// HandleDLNACover sends a cover (/dlna/cover/:type/:id.jpg, a db* ID).
func HandleDLNACover(c *gin.Context) {
	id, err := strconv.ParseUint(strings.TrimSuffix(c.Param("id"), ".jpg"), 10, 64)

	k := libKindByType(c.Param("type"))
	if err != nil || k == nil || !accessOf(c).allowsDB(k, &id) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	serveCover(c, c.Param("type"), id, false)
}
