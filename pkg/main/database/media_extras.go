package database

import "time"

// The cast and related titles (TMDB recommendations and similar titles) of
// a movie or series, saved by the metadata import (media_cast,
// media_related): the Library's More tab, the Jellyfin items and the cast
// search read them instead of asking TMDB.

// MaxStoredCast is how many of the cast are saved per title.
const MaxStoredCast = 20

// The relations of media_related.
const (
	RelationRecommended = "recommended"
	RelationSimilar     = "similar"
)

// MediaCast is a cast member of a movie or series.
type MediaCast struct {
	PersonID  int    `db:"person_id"`
	Name      string `db:"name"`
	Character string `db:"character"`
	Profile   string `db:"profile"` // TMDB image path
}

// MediaRelated is a recommended or similar title.
type MediaRelated struct {
	TMDB   int     `db:"tmdb_id"`
	Title  string  `db:"title"`
	Year   int     `db:"year"`
	Rating float64 `db:"rating"`
	Poster string  `db:"poster"` // TMDB image path
}

// MediaCastRow and MediaRelatedRow are the rows as the admin database
// browser shows them.
type MediaCastRow struct {
	ID        uint      `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	Kind      string    `db:"kind"`
	DbID      uint      `db:"db_id"`
	Position  int       `db:"position"`
	PersonID  int       `db:"person_id"`
	Name      string    `db:"name"`
	Character string    `db:"character"`
	Profile   string    `db:"profile"`
}

type MediaRelatedRow struct {
	ID        uint      `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	Kind      string    `db:"kind"`
	DbID      uint      `db:"db_id"`
	Relation  string    `db:"relation"`
	Position  int       `db:"position"`
	TMDB      int       `db:"tmdb_id"`
	Title     string    `db:"title"`
	Year      int       `db:"year"`
	Rating    float64   `db:"rating"`
	Poster    string    `db:"poster"`
}

// SaveMediaExtras replaces the cast and related titles of a title (kind
// "movie" with a dbmovies id, "series" with a dbseries id).
func SaveMediaExtras(kind string, dbID uint, cast []MediaCast, recommended, similar []MediaRelated) error {
	if dbID == 0 {
		return nil
	}

	return ExecNTx(func(exec func(string, ...any) error) error {
		if err := exec("DELETE FROM media_cast WHERE kind = ? AND db_id = ?", kind, dbID); err != nil {
			return err
		}

		if err := exec("DELETE FROM media_related WHERE kind = ? AND db_id = ?", kind, dbID); err != nil {
			return err
		}

		for i := range cast[:min(len(cast), MaxStoredCast)] {
			c := &cast[i]
			if err := exec("INSERT INTO media_cast (kind, db_id, position, person_id, name, character, profile) VALUES (?, ?, ?, ?, ?, ?, ?)",
				kind, dbID, i, c.PersonID, c.Name, c.Character, c.Profile); err != nil {
				return err
			}
		}

		for relation, titles := range map[string][]MediaRelated{RelationRecommended: recommended, RelationSimilar: similar} {
			for i := range titles {
				r := &titles[i]
				if err := exec("INSERT INTO media_related (kind, db_id, relation, position, tmdb_id, title, year, rating, poster) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
					kind, dbID, relation, i, r.TMDB, r.Title, r.Year, r.Rating, r.Poster); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// DeleteMediaExtras removes the saved cast and related titles of a deleted
// title (no foreign key: db_id refers to dbmovies or dbseries by kind;
// dbID as the route gives it).
func DeleteMediaExtras(kind string, dbID any) {
	ExecN("DELETE FROM media_cast WHERE kind = ? AND db_id = ?", kind, dbID)
	ExecN("DELETE FROM media_related WHERE kind = ? AND db_id = ?", kind, dbID)
}

// MediaCastOf is the saved cast of a title, in billing order.
func MediaCastOf(kind string, dbID uint64) []MediaCast {
	return StructscanT[MediaCast](false, MaxStoredCast,
		"SELECT person_id, name, character, profile FROM media_cast WHERE kind = ? AND db_id = ? ORDER BY position", kind, dbID)
}

// MediaPerson is a person's saved name and picture (by TMDB person id;
// false when no saved cast names them).
func MediaPerson(personID uint64) (MediaCast, bool) {
	r := StructscanT[MediaCast](false, 1, "SELECT person_id, name, character, profile FROM media_cast WHERE person_id = ? "+
		"ORDER BY profile = '', position LIMIT 1", personID)
	if len(r) == 0 {
		return MediaCast{}, false
	}

	return r[0], true
}

// MediaRelatedOf is the saved related titles of a title (relation
// RelationRecommended or RelationSimilar), in TMDB's order.
func MediaRelatedOf(kind string, dbID uint64, relation string) []MediaRelated {
	return StructscanT[MediaRelated](false, 20,
		"SELECT tmdb_id, title, year, rating, poster FROM media_related WHERE kind = ? AND db_id = ? AND relation = ? ORDER BY position",
		kind, dbID, relation)
}
