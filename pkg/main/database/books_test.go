package database

import "testing"

// TestGetDbbookByIDPNoSeries: a book without a series (dbbook_series_id
// NULL, as the author import stores it) is read with series 0.
func TestGetDbbookByIDPNoSeries(t *testing.T) {
	db := useMigratedMainDB(t)
	mustExec(t, db, "insert into dbbooks (id, title, slug, dbauthor_id, dbbook_series_id) values (5, 'Die Nadel', 'die-nadel', 1, NULL)")

	var b Dbbook
	if err := b.GetDbbookByIDP(new(uint(5))); err != nil || b.Title != "Die Nadel" || b.DbbookSeriesID != 0 {
		t.Fatalf("book = %+v, %v", b, err)
	}
}
