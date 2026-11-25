package api

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// The translations of a metadata row are edited below its database edit
// form (renderTranslationsEditor, loaded with the form): each saved text is
// a manual one, which the refreshes keep; an emptied one is removed.

// translationLanguages are the languages offered: the media groups'
// metadata languages and the page languages.
func translationLanguages() []string {
	langs := slices.Clone(languages)

	for _, t := range []uint{config.MediaTypeMovie, config.MediaTypeSeries, config.MediaTypeBook, config.MediaTypeAudiobook, config.MediaTypeMusic} {
		for _, l := range config.MetadataLanguages(t) {
			if !slices.Contains(langs, l) {
				langs = append(langs, l)
			}
		}
	}

	slices.Sort(langs)

	return langs
}

// renderTranslationsEditor lists a row's translations, each editable, and a
// line to add one.
func renderTranslationsEditor(mediaType string, mediaID uint) gomponents.Node {
	fields := database.TranslationFields[mediaType]
	trs := database.GetTranslations(mediaType, mediaID)

	rows := make([]gomponents.Node, 0, len(trs)+1)
	for _, tr := range trs {
		rows = append(rows, html.Tr(html.Data("lang", tr.Language), html.Data("field", tr.Field),
			html.Td(html.Class("text-nowrap"), html.Code(gomponents.Text(tr.Language))),
			html.Td(html.Class("text-nowrap"), gomponents.Text(tr.Field),
				gomponents.If(tr.Manual, html.Span(html.Class("badge bg-info ms-1"), html.Title("Edited by hand: refreshes keep it"), gomponents.Text("manual")))),
			html.Td(html.Textarea(html.Class("form-control form-control-sm"), html.Rows("2"), gomponents.Text(tr.Text))),
			html.Td(html.Class("text-nowrap"),
				html.Button(html.Type("button"), html.Class("btn btn-sm btn-primary me-1"), gomponents.Attr("onclick", "gmdTrSave(this)"),
					html.Title("Save"), html.I(html.Class("fa-solid fa-floppy-disk"))),
				html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-danger"), gomponents.Attr("onclick", "gmdTrSave(this, true)"),
					html.Title("Remove"), html.I(html.Class("fa-solid fa-trash"))))))
	}

	langOpts := []gomponents.Node{}
	for _, l := range translationLanguages() {
		langOpts = append(langOpts, html.Option(html.Value(l)))
	}

	fieldOpts := make([]gomponents.Node, 0, len(fields))
	for _, f := range fields {
		fieldOpts = append(fieldOpts, html.Option(html.Value(f.Field), gomponents.Text(f.Field)))
	}

	rows = append(rows, html.Tr(html.Class("gmd-tr-new"),
		html.Td(html.Input(html.Class("form-control form-control-sm"), html.Style("width:5rem"), html.Placeholder("de"),
			html.MaxLength("8"), gomponents.Attr("list", "gmdTrLangs"), html.Name("lang"))),
		html.Td(html.Select(html.Class("form-select form-select-sm"), html.Name("field"), gomponents.Group(fieldOpts))),
		html.Td(html.Textarea(html.Class("form-control form-control-sm"), html.Rows("2"), html.Placeholder("Translation"))),
		html.Td(html.Button(html.Type("button"), html.Class("btn btn-sm btn-success"), gomponents.Attr("onclick", "gmdTrSave(this)"),
			html.I(html.Class("fa-solid fa-plus me-1")), gomponents.Text("Add")))))

	return html.Div(html.ID("gmdTranslations"), html.Class("card card-body mt-3 shadow-sm"),
		html.Data("type", mediaType), html.Data("id", strconv.FormatUint(uint64(mediaID), 10)),
		html.H6(html.Class("mb-1"), html.I(html.Class("fa-solid fa-language me-2")), gomponents.Text("Translations")),
		html.P(html.Class("small text-muted mb-2"),
			gomponents.Text("Shown instead of the stored text to accounts with that page language. Saved texts are kept by metadata refreshes; remove one to let the next refresh fetch it again.")),
		html.DataList(html.ID("gmdTrLangs"), gomponents.Group(langOpts)),
		html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm align-middle mb-0"),
			html.THead(html.Tr(html.Th(gomponents.Text("Language")), html.Th(gomponents.Text("Field")), html.Th(gomponents.Text("Text")), html.Th())),
			html.TBody(gomponents.Group(rows)))),
		html.Script(gomponents.Raw(translationsEditorJS)))
}

// translationsEditorJS saves one line (remove: an empty text) and reloads
// the list.
const translationsEditorJS = `window.gmdTrSave = function (btn, remove) {
	var box = document.getElementById('gmdTranslations'), tr = btn.closest('tr');
	var isNew = tr.classList.contains('gmd-tr-new');
	var lang = isNew ? tr.querySelector('[name=lang]').value.trim().toLowerCase() : tr.dataset.lang;
	var field = isNew ? tr.querySelector('[name=field]').value : tr.dataset.field;
	var text = remove ? '' : tr.querySelector('textarea').value;
	if (!lang) { alert('Enter a language code, e.g. de'); return; }
	var csrf = (document.querySelector('input[name="csrf_token"]') || {}).value || '';
	var body = new URLSearchParams({ type: box.dataset.type, id: box.dataset.id, lang: lang, field: field, text: text });
	fetch('/api/admin/translations', { method: 'POST', body: body, headers: { 'X-CSRF-Token': csrf }, credentials: 'same-origin' })
		.then(function (r) { return r.text().then(function (t) { if (!r.ok) { throw new Error(t || r.statusText); } return t; }); })
		.then(function (html) { box.outerHTML = html; })
		.catch(function (e) { alert('Saving the translation failed: ' + e.message); });
};`

// HandleTranslationsEditor renders a row's translations editor
// (GET /admin/translations?type=dbmovie&id=1).
func HandleTranslationsEditor(c *gin.Context) {
	mediaType, id, ok := translationTarget(c, c.Query("type"), c.Query("id"))
	if !ok {
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = renderTranslationsEditor(mediaType, id).Render(c.Writer)
}

// HandleTranslationSave stores one translation by hand (POST
// /admin/translations: type, id, lang, field, text; an empty text removes
// it) and answers with the editor.
func HandleTranslationSave(c *gin.Context) {
	mediaType, id, ok := translationTarget(c, c.PostForm("type"), c.PostForm("id"))
	if !ok {
		return
	}

	lang, field := config.LanguageCode(c.PostForm("lang")), c.PostForm("field")
	if len(lang) != 2 || strings.ContainsAny(lang, " '\"<>") {
		c.String(http.StatusBadRequest, "language must be a two-letter code (de, en, ...)")
		return
	}

	if !slices.ContainsFunc(database.TranslationFields[mediaType], func(f struct{ Field, Column string }) bool { return f.Field == field }) {
		c.String(http.StatusBadRequest, "unknown field "+field)
		return
	}

	if err := database.SetManualTranslation(mediaType, id, field, lang, c.PostForm("text")); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = renderTranslationsEditor(mediaType, id).Render(c.Writer)
}

// translationTarget checks a media type and the id of one of its rows.
func translationTarget(c *gin.Context, mediaType, idStr string) (string, uint, bool) {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if _, known := database.TranslationFields[mediaType]; !known || err != nil || id == 0 {
		c.String(http.StatusBadRequest, "unknown type or id")
		return "", 0, false
	}

	return mediaType, uint(id), true
}
