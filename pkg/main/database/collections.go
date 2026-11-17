package database

// MonitoredCollection is a TMDB collection whose new movies are added to a
// list (movie_collections).
type MonitoredCollection struct {
	ID           uint   `db:"id"`
	CollectionID int    `db:"collection_id"`
	Name         string `db:"name"`
	Listname     string `db:"listname"`
	LastCheck    string `db:"last_check"`
}

// MonitorCollection adds new movies of the collection to the list from now
// on (on=false stops it).
func MonitorCollection(collectionID int, name, listname string, on bool) error {
	if !on {
		return ExecNErr("DELETE FROM movie_collections WHERE collection_id = ? AND listname = ? COLLATE NOCASE", collectionID, listname)
	}

	return ExecNErr("INSERT INTO movie_collections (collection_id, name, listname) VALUES (?, ?, ?) "+
		"ON CONFLICT (collection_id, listname) DO UPDATE SET name = excluded.name", collectionID, name, listname)
}

// CollectionMonitored reports whether the list gets the collection's new
// movies.
func CollectionMonitored(collectionID int, listname string) bool {
	return Getdatarow[bool](false, "SELECT EXISTS(SELECT 1 FROM movie_collections WHERE collection_id = ? AND listname = ? COLLATE NOCASE)",
		collectionID, listname)
}

// GetMonitoredCollections returns all monitored collections.
func GetMonitoredCollections() []MonitoredCollection {
	return StructscanT[MonitoredCollection](false, 0,
		"SELECT id, collection_id, name, listname, COALESCE(last_check, '') AS last_check FROM movie_collections ORDER BY name")
}

// TouchCollectionCheck records that the collection was just checked.
func TouchCollectionCheck(id *uint) {
	ExecN("UPDATE movie_collections SET last_check = datetime('now','localtime') WHERE id = ?", id)
}
