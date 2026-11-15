package api

import (
	"cmp"
	"fmt"
	stdhtml "html"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// The quality priority editor turns a drag-and-drop order into reorder rules.
//
// A movie/series release's priority is resolution+quality (or a
// combined_res_qual value for that pair) plus codec and audio, each taken from
// the qualities table and the profile's reorder rules. The editor lists the
// profile's wanted resolution+quality combinations, and codec, audio and audio
// format when they count, each ordered by its current value. Reordering keeps
// the list's set of values and hands them out in the new order: the entry
// moved into a slot takes that slot's value. Rules are written only where an
// entry's value differs from what it gets without an editor rule, so an
// unchanged list produces no rules, and values of entries that aren't listed
// (unwanted pairs, "not detected" = 0) keep their meaning.

const qpPairKey = "pair"

// qpList is one orderable list of the editor.
type qpList struct {
	Key   string // reorder rule type; qpPairKey for resolution+quality pairs
	Label string
}

var qpSingleLists = []struct {
	qpList
	entries func() []database.Qualities
	active  func(q *config.QualityConfig) bool
}{
	{qpList{"codec", "Video Codec"}, func() []database.Qualities { return database.DBConnect.GetcodecsIn },
		func(q *config.QualityConfig) bool { return q.UseForPriorityCodec }},
	{qpList{"audio", "Audio Codec"}, func() []database.Qualities { return database.DBConnect.GetaudiosIn },
		func(q *config.QualityConfig) bool { return q.UseForPriorityAudio }},
	// Audio formats rank music and audiobook releases and have no switch:
	// shown for profiles without video categories or with audio format rules.
	{qpList{"audio_format", "Audio Format"}, func() []database.Qualities { return database.DBConnect.GetaudioformatsIn },
		func(q *config.QualityConfig) bool {
			return !(q.UseForPriorityResolution || q.UseForPriorityQuality || q.UseForPriorityCodec || q.UseForPriorityAudio) ||
				slices.ContainsFunc(q.QualityReorder, func(r config.QualityReorderConfig) bool {
					return strings.EqualFold(r.ReorderType, "audio_format")
				})
		}},
}

// qpEntry is one list entry with its effective value in a profile.
type qpEntry struct {
	Name   string // pairs: "resolution,quality", as in combined_res_qual rules
	Value  int
	Wanted bool
}

func qpContainsFold(list []string, name string) bool {
	return slices.ContainsFunc(list, func(s string) bool { return strings.EqualFold(s, name) })
}

func qpWanted(list []string, name string) bool {
	return len(list) == 0 || qpContainsFold(list, name)
}

// qpLists returns the lists that count for the profile, in display order.
func qpLists(q *config.QualityConfig) []qpList {
	var lists []qpList
	if q.UseForPriorityResolution || q.UseForPriorityQuality {
		lists = append(lists, qpList{qpPairKey, "Resolution + Source"})
	}

	for i := range qpSingleLists {
		if qpSingleLists[i].active(q) {
			lists = append(lists, qpSingleLists[i].qpList)
		}
	}

	return lists
}

// qpEntries returns a list's entries with their values in q, best first.
func qpEntries(q *config.QualityConfig, key string) []qpEntry {
	var entries []qpEntry

	if key == qpPairKey {
		for _, r := range database.DBConnect.GetresolutionsIn {
			if q.UseForPriorityResolution && !qpWanted(q.WantedResolution, r.Name) {
				continue
			}

			for _, ql := range database.DBConnect.GetqualitiesIn {
				if q.UseForPriorityQuality && !qpWanted(q.WantedQuality, ql.Name) {
					continue
				}

				rv, qv := 0, 0
				if q.UseForPriorityResolution {
					rv = r.Gettypeidprioritysingle("resolution", q)
				}

				if q.UseForPriorityQuality {
					qv = ql.Gettypeidprioritysingle("quality", q)
				}

				rv, qv, _ = previewCombinedReorder(q, r.Name, ql.Name, rv, qv)
				entries = append(entries, qpEntry{Name: r.Name + "," + ql.Name, Value: rv + qv, Wanted: true})
			}
		}
	} else {
		for i := range qpSingleLists {
			if qpSingleLists[i].Key != key {
				continue
			}

			wanted := map[string][]string{"codec": q.WantedCodec, "audio": q.WantedAudio, "audio_format": q.WantedAudioFormats}[key]
			for _, e := range qpSingleLists[i].entries() {
				entries = append(entries, qpEntry{
					Name:   e.Name,
					Value:  e.Gettypeidprioritysingle(key, q),
					Wanted: qpWanted(wanted, e.Name),
				})
			}
		}
	}

	slices.SortStableFunc(entries, func(a, b qpEntry) int { return cmp.Compare(b.Value, a.Value) })

	return entries
}

// qpManaged reports whether rule r is one the editor writes for list key, so
// it is replaced on save: combined rules of listed pairs, and direct rules of
// a single list's entries.
func qpManaged(r *config.QualityReorderConfig, key string, names []string) bool {
	if key == qpPairKey {
		return strings.EqualFold(r.ReorderType, "combined_res_qual") && qpContainsFold(names, r.Name)
	}

	return strings.EqualFold(r.ReorderType, key) && qpContainsFold(names, r.Name)
}

// qpAssign returns each entry's value for the new order (best first). The
// largest set of entries whose relative order is unchanged keeps its values;
// moved entries get values between their new neighbours (or one typical step
// beyond the ends), so dragging one entry changes only that entry. If there
// is no integer room between two neighbours, the list's values are handed out
// by slot instead (qpAssignSlots).
func qpAssign(current []qpEntry, order []string, natural map[string]int) map[string]int {
	idx := make(map[string]int, len(current))
	for i, e := range current {
		idx[strings.ToLower(e.Name)] = i
	}

	seq := make([]int, len(order))
	for i, name := range order {
		seq[i] = idx[strings.ToLower(name)]
	}

	keep := qpLongestIncreasing(seq)
	values := make(map[string]int, len(order))

	for i, name := range order {
		if keep[i] {
			values[strings.ToLower(name)] = current[seq[i]].Value
		}
	}

	step := 1
	if n := len(current); n > 1 {
		step = max(1, (current[0].Value-current[n-1].Value)/(n-1))
	}

	for i := 0; i < len(order); {
		if keep[i] {
			i++
			continue
		}

		j := i
		for j < len(order) && !keep[j] {
			j++
		}

		count := j - i
		hasHi, hasLo := i > 0, j < len(order)

		var hi, lo int
		if hasHi {
			hi = values[strings.ToLower(order[i-1])]
		}

		if hasLo {
			lo = values[strings.ToLower(order[j])]
		}

		switch {
		case !hasHi:
			hi = lo + (count+1)*step
		case !hasLo:
			lo = hi - (count+1)*step
		}

		// Entries whose value without an editor rule already fits keep it, so
		// moving an entry back to where it came from removes its rule.
		fits := true
		for k, last := 0, hi; k < count; k++ {
			v, ok := natural[strings.ToLower(order[i+k])]
			if !ok || (hasHi || k > 0) && v >= last || hasLo && v <= lo {
				fits = false
				break
			}

			last = v
		}

		if fits {
			for k := range count {
				values[strings.ToLower(order[i+k])] = natural[strings.ToLower(order[i+k])]
			}

			i = j

			continue
		}

		if hi-lo < count+1 {
			return qpAssignSlots(current, order)
		}

		for k := range count {
			values[strings.ToLower(order[i+k])] = hi - (hi-lo)*(k+1)/(count+1)
		}

		i = j
	}

	return values
}

// qpLongestIncreasing marks a longest strictly increasing subsequence of seq.
func qpLongestIncreasing(seq []int) []bool {
	n := len(seq)
	length := make([]int, n)
	prev := make([]int, n)
	best := -1

	for i := range n {
		length[i], prev[i] = 1, -1

		for j := range i {
			if seq[j] < seq[i] && length[j]+1 > length[i] {
				length[i], prev[i] = length[j]+1, j
			}
		}

		if best == -1 || length[i] > length[best] {
			best = i
		}
	}

	keep := make([]bool, n)
	for i := best; i >= 0; i = prev[i] {
		keep[i] = true
	}

	return keep
}

// qpAssignSlots hands the current values (best first) to the entries in their
// new order. Where moved entries meet an equal value, it is lowered by 1 so
// the new order holds; ties nobody touched stay ties.
func qpAssignSlots(current []qpEntry, order []string) map[string]int {
	values := make(map[string]int, len(order))

	prev := 0
	for i, name := range order {
		v := current[i].Value

		moved := !strings.EqualFold(name, current[i].Name) || (i > 0 && !strings.EqualFold(order[i-1], current[i-1].Name))
		if i > 0 && v >= prev && moved {
			v = prev - 1
		}

		values[strings.ToLower(name)] = v
		prev = v
	}

	return values
}

func qpCheckOrder(label string, current []qpEntry, order []string) error {
	want := make(map[string]bool, len(current))
	for _, e := range current {
		want[strings.ToLower(e.Name)] = true
	}

	seen := make(map[string]bool, len(order))
	for _, name := range order {
		key := strings.ToLower(name)
		if !want[key] || seen[key] {
			return fmt.Errorf("%s: unexpected or repeated entry %q - reload the page", label, name)
		}

		seen[key] = true
	}

	if len(seen) != len(want) {
		return fmt.Errorf("%s: %d of %d entries sent - reload the page", label, len(seen), len(want))
	}

	return nil
}

// qpBuildRules returns the profile's new reorder list for the posted orders
// (list key -> entry names, best first) and the rules the editor generated.
// Rules the editor doesn't manage are kept in place; generated rules are
// appended, after any position rule, so their value is final.
func qpBuildRules(q *config.QualityConfig, orders map[string][]string) (merged, generated []config.QualityReorderConfig, err error) {
	lists := qpLists(q)
	current := make(map[string][]qpEntry, len(lists))

	for _, l := range lists {
		current[l.Key] = qpEntries(q, l.Key)
		if err := qpCheckOrder(l.Label, current[l.Key], orders[l.Key]); err != nil {
			return nil, nil, err
		}
	}

	// The profile without editor rules gives each entry's natural value; a
	// rule is only needed where the new value differs from it.
	natural := *q
	natural.QualityReorder = slices.DeleteFunc(slices.Clone(q.QualityReorder), func(r config.QualityReorderConfig) bool {
		return slices.ContainsFunc(lists, func(l qpList) bool { return qpManaged(&r, l.Key, entryNames(current[l.Key])) })
	})

	merged = slices.Clone(natural.QualityReorder)

	for _, l := range lists {
		naturalValue := make(map[string]int)
		for _, e := range qpEntries(&natural, l.Key) {
			naturalValue[strings.ToLower(e.Name)] = e.Value
		}

		values := qpAssign(current[l.Key], orders[l.Key], naturalValue)

		ruleType := l.Key
		if l.Key == qpPairKey {
			ruleType = "combined_res_qual"
		}

		for _, name := range orders[l.Key] {
			v := values[strings.ToLower(name)]
			if v == naturalValue[strings.ToLower(name)] {
				continue
			}

			generated = append(generated, config.QualityReorderConfig{Name: name, ReorderType: ruleType, Newpriority: v})
		}
	}

	return append(merged, generated...), generated, nil
}

func entryNames(entries []qpEntry) []string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}

	return names
}

// qpRulesTOML renders reorder rules the way they appear in config.toml.
func qpRulesTOML(rules []config.QualityReorderConfig) string {
	var b strings.Builder

	for i := range rules {
		if i > 0 {
			b.WriteByte('\n')
		}

		fmt.Fprintf(&b, "[[quality.reorder]]\nname = %q\ntype = %q\nnew_priority = %d\n",
			rules[i].Name, rules[i].ReorderType, rules[i].Newpriority)
	}

	return b.String()
}

func qpEntryLabel(key, name string) string {
	if key == qpPairKey {
		return strings.Replace(name, ",", " · ", 1)
	}

	return name
}

// qpFormRules reads the profile and the posted orders and builds the rules.
func qpFormRules(c *gin.Context) (q *config.QualityConfig, merged, generated []config.QualityReorderConfig, err error) {
	if err := c.Request.ParseForm(); err != nil {
		return nil, nil, nil, err
	}

	q = config.GetSettingsQuality(c.PostForm("profile"))
	if q == nil {
		return nil, nil, nil, fmt.Errorf("quality profile %q not found", c.PostForm("profile"))
	}

	orders := make(map[string][]string)
	for _, l := range qpLists(q) {
		orders[l.Key] = c.PostFormArray("order_" + l.Key)
	}

	merged, generated, err = qpBuildRules(q, orders)

	return q, merged, generated, err
}

// adminPageQualityPriorities renders the quality priority editor page.
// adminPageQualityPriorities is the priority editor page; ?profile= opens it
// on that profile, ?embed=1 renders it for the overlay (openPriorityEditor).
func adminPageQualityPriorities(ctx *gin.Context) {
	var names []string

	config.RangeSettingsQuality(func(name string, _ *config.QualityConfig) { names = append(names, name) })
	slices.Sort(names)

	selected := ctx.Query("profile")
	if !slices.Contains(names, selected) {
		selected = ""
	}

	options := []gomponents.Node{html.Option(html.Value(""), gomponents.Text("-- select a quality profile --"))}
	for _, n := range names {
		options = append(options, html.Option(html.Value(n), gomponents.If(n == selected, html.Selected()), gomponents.Text(n)))
	}

	trigger := "change"
	if selected != "" {
		trigger = "change, load"
	}

	content := html.Div(
		html.Class("config-section-enhanced"),
		renderEnhancedPageHeader(
			"fa-solid fa-arrow-down-wide-short",
			"Quality Priorities",
			"Drag resolution and source combinations, codecs and audio into the order you want; the reorder rules are generated for you.",
		),
		html.Div(
			html.Class("card border-0 shadow-sm mb-4"),
			html.Div(
				html.Class("card-body"),
				html.P(html.Class("text-muted small"), gomponents.Text(
					"Each list shows the profile's current order and values, best first. Drag entries (or use ↑ ↓) into the order you want: "+
						"an entry takes the value of the slot it is moved to, so only moved entries get rules and everything else keeps its value. "+
						"Releases where a part isn't recognized count 0 for that part; entries with negative values rank below them.")),
				html.Label(html.Class("form-label"), html.For("qpProfile"), gomponents.Text("Quality Profile")),
				html.Select(
					html.ID("qpProfile"), html.Name("profile"), html.Class("form-select"),
					hx.Get("/api/admin/quality-priorities/editor"),
					hx.Target("#qpEditor"),
					hx.Trigger(trigger),
					gomponents.Group(options),
				),
			),
		),
		html.Div(html.ID("qpEditor")),
		html.StyleEl(gomponents.Raw(qpStyles)),
		html.Script(gomponents.Raw(qpScript)),
	)

	renderToolPage(ctx, "Quality Priorities", content)
}

// HandleQualityPrioritiesEditor renders the sortable lists for one profile.
func HandleQualityPrioritiesEditor(c *gin.Context) {
	q := config.GetSettingsQuality(c.Query("profile"))
	if q == nil {
		c.String(http.StatusOK, "")
		return
	}

	lists := qpLists(q)
	if len(lists) == 0 {
		c.String(http.StatusOK, renderAlert("This profile uses no category for its priority.", "info"))
		return
	}

	columns := make([]gomponents.Node, 0, len(lists))
	for _, l := range lists {
		items := make([]gomponents.Node, 0)
		for _, e := range qpEntries(q, l.Key) {
			note := strconv.Itoa(e.Value)
			if !e.Wanted {
				note += " · not wanted"
			}

			items = append(items, qpItem(qpEntryLabel(l.Key, e.Name), "order_"+l.Key, e.Name, e.Wanted, note))
		}

		col := "col-12 col-md-6 col-xl-3"
		if l.Key == qpPairKey {
			col = "col-12 col-xl-6"
		}

		columns = append(columns, html.Div(
			html.Class(col+" mb-3"),
			html.Div(html.Class("mb-2"), html.Strong(gomponents.Text(l.Label))),
			html.Ul(html.Class("list-group qp-list"), html.Data("qp-sortable", "true"), gomponents.Group(items)),
		))
	}

	var off []string

	for _, s := range []struct {
		on    bool
		label string
	}{
		{q.UseForPriorityResolution, "resolution"}, {q.UseForPriorityQuality, "source"},
		{q.UseForPriorityCodec, "codec"}, {q.UseForPriorityAudio, "audio"},
	} {
		if !s.on {
			off = append(off, s.label)
		}
	}

	notes := []gomponents.Node{}
	if len(off) > 0 {
		notes = append(notes, html.P(html.Class("small text-muted"), gomponents.Text(
			"Not used for priority by this profile, so not listed: "+strings.Join(off, ", ")+
				" (the Use … For Priority switches in the quality config).")))
	}

	c.String(http.StatusOK, renderComponentToString(html.Form(
		html.ID("qpForm"),
		html.Input(html.Type("hidden"), html.Name("profile"), html.Value(q.Name)),
		html.Div(
			html.Class("card border-0 shadow-sm mb-4"),
			html.Div(html.Class("card-body"),
				html.H5(gomponents.Text("Order the entries (best first)")),
				gomponents.Group(notes),
				html.Div(html.Class("row"), gomponents.Group(columns)),
			),
		),
		html.Button(
			html.Type("button"), html.Class("btn btn-primary"),
			hx.Post("/api/admin/quality-priorities/generate"),
			hx.Include("#qpForm"), hx.Target("#qpResult"),
			hx.Headers(`{"X-CSRF-Token": "`+getCSRFToken(c)+`"}`),
			html.I(html.Class("fas fa-gears me-1")), gomponents.Text("Generate Rules"),
		),
		html.Button(
			html.Type("button"), html.Class("btn btn-secondary ms-2"),
			hx.Get("/api/admin/quality-priorities/editor?profile="+q.Name), hx.Target("#qpEditor"),
			html.I(html.Class("fas fa-rotate-left me-1")), gomponents.Text("Reset"),
		),
		html.Div(html.ID("qpResult"), html.Class("mt-4")),
	)))
}

// qpItem renders one draggable list entry with its hidden form value.
func qpItem(label, field, value string, wanted bool, note string) gomponents.Node {
	class := "list-group-item d-flex align-items-center py-1 qp-item"
	if !wanted {
		class += " text-muted"
	}

	return html.Li(
		html.Class(class),
		gomponents.Attr("draggable", "true"),
		html.I(html.Class("fas fa-grip-vertical me-2 text-muted")),
		html.Span(html.Class("flex-grow-1"), gomponents.Text(label)),
		html.Small(html.Class("text-muted me-2"), gomponents.Text(note)),
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-link p-0 me-1 qp-up"), gomponents.Attr("aria-label", "Move up"), gomponents.Text("↑")),
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-link p-0 qp-down"), gomponents.Attr("aria-label", "Move down"), gomponents.Text("↓")),
		html.Input(html.Type("hidden"), html.Name(field), html.Value(value)),
	)
}

// HandleQualityPrioritiesGenerate shows the generated rules, the resulting
// order of each list and a save button.
func HandleQualityPrioritiesGenerate(c *gin.Context) {
	q, merged, generated, err := qpFormRules(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert(err.Error(), "danger"))
		return
	}

	after := *q
	after.QualityReorder = merged

	tables := make([]gomponents.Node, 0)

	for _, l := range qpLists(q) {
		oldRank := make(map[string]int)
		for i, e := range qpEntries(q, l.Key) {
			oldRank[e.Name] = i + 1
		}

		rows := make([]gomponents.Node, 0)
		for i, e := range qpEntries(&after, l.Key) {
			change := ""
			if old := oldRank[e.Name]; old != i+1 {
				change = fmt.Sprintf("was %d", old)
			}

			rows = append(rows, html.Tr(
				html.Td(gomponents.Text(strconv.Itoa(i+1))),
				html.Td(gomponents.Text(qpEntryLabel(l.Key, e.Name))),
				html.Td(gomponents.Text(strconv.Itoa(e.Value))),
				html.Td(html.Class("text-muted"), gomponents.Text(change)),
			))
		}

		tables = append(tables, html.Div(
			html.Class("col-12 col-xl-6 mb-3"),
			html.Strong(gomponents.Text(l.Label)),
			html.Div(html.Style("max-height: 420px; overflow-y: auto;"),
				html.Table(html.Class("table table-sm"),
					html.THead(html.Tr(html.Th(gomponents.Text("#")), html.Th(gomponents.Text("Entry")),
						html.Th(gomponents.Text("Value")), html.Th(gomponents.Text("Before")))),
					html.TBody(gomponents.Group(rows)),
				)),
		))
	}

	var saveButton gomponents.Node = html.P(html.Class("text-muted"),
		gomponents.Text("The order matches the profile already; nothing to save."))
	if !slices.Equal(merged, q.QualityReorder) {
		saveButton = html.Button(
			html.Type("button"), html.Class("btn btn-success"),
			hx.Post("/api/admin/quality-priorities/save"),
			hx.Include("#qpForm"), hx.Target("#qpSaveResult"),
			hx.Headers(`{"X-CSRF-Token": "`+getCSRFToken(c)+`"}`),
			hx.Confirm("Replace the reorder rules of quality profile "+q.Name+" in config.toml?"),
			html.I(html.Class("fas fa-floppy-disk me-1")), gomponents.Text("Save to "+q.Name),
		)
	}

	c.String(http.StatusOK, renderComponentToString(html.Div(
		html.Div(html.Class("card border-0 shadow-sm mb-4"), html.Div(html.Class("card-body"),
			html.H5(gomponents.Text("Resulting order")),
			html.Div(html.Class("row"), gomponents.Group(tables)),
		)),
		html.Div(html.Class("card border-0 shadow-sm mb-4"), html.Div(html.Class("card-body"),
			html.H5(gomponents.Text("Reorder rules for "+q.Name)),
			html.P(html.Class("small text-muted"), gomponents.Text(fmt.Sprintf(
				"%d generated rule(s); the profile's other %d rule(s) are kept. This is the complete reorder list that will be saved:",
				len(generated), len(merged)-len(generated)))),
			html.Pre(html.Class("bg-light p-3 small"), html.Style("max-height: 360px; overflow-y: auto;"),
				gomponents.Text(qpRulesTOML(merged))),
			saveButton,
			html.Div(html.ID("qpSaveResult"), html.Class("mt-3")),
		)),
	)))
}

// HandleQualityPrioritiesSave writes the generated rules into the profile and
// rebuilds the priority tables.
func HandleQualityPrioritiesSave(c *gin.Context) {
	q, merged, _, err := qpFormRules(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert(err.Error(), "danger"))
		return
	}

	updated := *q
	updated.QualityReorder = merged

	if err := config.UpdateCfgEntry(config.Conf{Name: "quality_" + q.Name, Data: updated}); err != nil {
		c.String(http.StatusOK, renderAlert("Saving failed: "+err.Error(), "danger"))
		return
	}

	parser.GenerateAllQualityPriorities()
	parser.GenerateCutoffPriorities()

	// data-qp-saved tells the overlay (openPriorityEditor) that rules changed.
	c.String(http.StatusOK, `<div data-qp-saved="`+stdhtml.EscapeString(q.Name)+`">`+renderAlert(fmt.Sprintf(
		"Saved %d reorder rules to %s and rebuilt the priorities. Stored file priorities update on the next scan.",
		len(merged), q.Name), "success")+`</div>`)
}

const qpStyles = `
.qp-list .qp-item { cursor: grab; user-select: none; }
.qp-list .qp-item.qp-dragging { opacity: .4; }
`

// qpScript wires native drag and drop and the up/down buttons for every list
// marked data-qp-sortable. The hidden inputs follow the visual order, so the
// form submits the new order.
const qpScript = `
(function () {
	var dragged = null;
	function list(el) { return el && el.closest('[data-qp-sortable]'); }
	document.addEventListener('dragstart', function (e) {
		var item = e.target.closest && e.target.closest('.qp-item');
		if (!item || !list(item)) { return; }
		dragged = item;
		item.classList.add('qp-dragging');
		e.dataTransfer.effectAllowed = 'move';
		e.dataTransfer.setData('text/plain', '');
	});
	document.addEventListener('dragend', function () {
		if (dragged) { dragged.classList.remove('qp-dragging'); }
		dragged = null;
	});
	document.addEventListener('dragover', function (e) {
		if (!dragged) { return; }
		var over = e.target.closest && e.target.closest('.qp-item');
		if (!over || over === dragged || list(over) !== list(dragged)) { return; }
		e.preventDefault();
		var box = over.getBoundingClientRect();
		var after = e.clientY > box.top + box.height / 2;
		over.parentNode.insertBefore(dragged, after ? over.nextSibling : over);
	});
	document.addEventListener('drop', function (e) { if (dragged) { e.preventDefault(); } });
	document.addEventListener('click', function (e) {
		var btn = e.target.closest && e.target.closest('.qp-up, .qp-down');
		if (!btn) { return; }
		var item = btn.closest('.qp-item');
		if (!list(item)) { return; }
		if (btn.classList.contains('qp-up') && item.previousElementSibling) {
			item.parentNode.insertBefore(item, item.previousElementSibling);
		} else if (btn.classList.contains('qp-down') && item.nextElementSibling) {
			item.parentNode.insertBefore(item.nextElementSibling, item);
		}
	});
})();
`
