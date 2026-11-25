package database

import (
	"strings"
)

// Translations of metadata texts (metadata_translations): media_type is
// the metadata table without its plural s (dbmovie, dbserie_episode, ...),
// field the text (title, overview, description, tagline, bio), language an
// ISO 639-1 code. Fetched rows follow the source; manual rows were edited
// by hand and stay until they're edited again.

// Translation is a translated metadata text.
type Translation struct {
	ID        uint   `db:"id"`
	MediaType string `db:"media_type"`
	MediaID   uint   `db:"media_id"`
	Field     string `db:"field"`
	Language  string `db:"language"`
	Text      string `db:"translation"`
	Manual    bool   `db:"manual"`
}

// TranslationFields are the translatable texts of each metadata table
// (media_type), with the column each translates.
var TranslationFields = map[string][]struct{ Field, Column string }{
	"dbmovie":         {{"title", "title"}, {"overview", "overview"}, {"tagline", "tagline"}},
	"dbserie":         {{"title", "seriename"}, {"overview", "overview"}},
	"dbserie_episode": {{"title", "title"}, {"overview", "overview"}},
	"dbbook":          {{"title", "title"}, {"description", "description"}},
	"dbaudiobook":     {{"title", "title"}, {"description", "description"}},
	"dbbook_series":   {{"title", "name"}, {"description", "description"}},
	"dbartist":        {{"bio", "bio"}},
	"dbauthor":        {{"bio", "bio"}},
	"dbnarrator":      {{"bio", "bio"}},
}

// TranslationMediaType is the media_type of a metadata table ("dbmovies":
// "dbmovie"); "" for a table without translations.
func TranslationMediaType(table string) string {
	for _, t := range []string{table, strings.TrimSuffix(table, "s")} {
		if _, ok := TranslationFields[t]; ok {
			return t
		}
	}

	return ""
}

// SetTranslation stores a fetched translation. One that is empty or the
// same as the original text removes the fetched row instead; a manual row
// is left alone either way.
func SetTranslation(mediaType string, mediaID uint, field, lang, text, original string) {
	text = strings.TrimSpace(text)

	if text == "" || text == strings.TrimSpace(original) {
		ExecN("DELETE FROM metadata_translations WHERE media_type = ? AND media_id = ? AND field = ? AND language = ? AND manual = 0",
			mediaType, mediaID, field, lang)

		return
	}

	ExecN(`INSERT INTO metadata_translations (media_type, media_id, field, language, translation) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (media_type, media_id, field, language) DO UPDATE SET translation = excluded.translation, updated_at = current_timestamp
		WHERE manual = 0 AND translation != excluded.translation`,
		mediaType, mediaID, field, lang, text)
}

// SetManualTranslation stores a translation edited by hand; an empty one
// removes the row (the next refresh may fetch it again).
func SetManualTranslation(mediaType string, mediaID uint, field, lang, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return ExecNErr("DELETE FROM metadata_translations WHERE media_type = ? AND media_id = ? AND field = ? AND language = ?",
			mediaType, mediaID, field, lang)
	}

	return ExecNErr(`INSERT INTO metadata_translations (media_type, media_id, field, language, translation, manual) VALUES (?, ?, ?, ?, ?, 1)
		ON CONFLICT (media_type, media_id, field, language) DO UPDATE SET translation = excluded.translation, manual = 1,
			updated_at = current_timestamp`,
		mediaType, mediaID, field, lang, text)
}

// GetTranslations are the translations of one entry, by language and field.
func GetTranslations(mediaType string, mediaID uint) []Translation {
	return StructscanT[Translation](false, 8,
		"SELECT id, media_type, media_id, field, language, translation, manual FROM metadata_translations "+
			"WHERE media_type = ? AND media_id = ? ORDER BY language, field", mediaType, mediaID)
}

// TranslationsIn are the translations into lang of the entries ids, by
// entry and field.
func TranslationsIn(mediaType, lang string, ids []uint) map[uint]map[string]string {
	out := map[uint]map[string]string{}
	if lang == "" || len(ids) == 0 {
		return out
	}

	const chunk = 500

	for start := 0; start < len(ids); start += chunk {
		part := ids[start:min(start+chunk, len(ids))]

		args := make([]any, 0, len(part)+2)
		args = append(args, mediaType, lang)

		for i := range part {
			args = append(args, part[i])
		}

		rows := StructscanT[Translation](false, uint(len(part)),
			"SELECT media_id, field, translation FROM metadata_translations WHERE media_type = ? AND language = ? AND media_id IN (?"+
				strings.Repeat(",?", len(part)-1)+")", args...)

		for _, r := range rows {
			if out[r.MediaID] == nil {
				out[r.MediaID] = map[string]string{}
			}

			out[r.MediaID][r.Field] = r.Text
		}
	}

	return out
}
