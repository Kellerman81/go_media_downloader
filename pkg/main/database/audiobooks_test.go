package database

import (
	"errors"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

func seedAudiobooks(t *testing.T) {
	t.Helper()

	db := useMigratedMainDB(t)

	mustExec(t, db, "insert into dbauthors (id, name, slug) values (1, 'Stephen Fry', 'stephen-fry')")
	mustExec(t, db, "insert into dbaudiobooks (id, title, slug, asin, audible_id) values (10, 'Mythos', 'mythos', 'B0759MNMD2', '')")
	mustExec(t, db, "insert into dbaudiobooks (id, title, slug, asin, audible_id) values (11, 'Heroes', 'heroes', '', 'B07H8Q5SQD')")
	mustExec(t, db, "insert into dbaudiobooks (id, title, slug, asin, audible_id) values (12, 'The Fry Chronicles', 'the-fry-chronicles', '', '')")
	mustExec(t, db, "insert into dbaudiobook_titles (dbaudiobook_id, title, slug) values (11, 'Heroes: Mortals and Monsters', 'heroes-mortals-and-monsters')")
	mustExec(t, db, "insert into dbaudiobook_authors (dbaudiobook_id, dbauthor_id) values (10, 1), (11, 1), (12, 1)")
	mustExec(t, db, "insert into audiobooks (id, dbaudiobook_id, listname, rootpath, quality_profile) values (100, 10, 'Fantasy', '/data/books/Stephen Fry/Mythos', 'm4b')")
}

func TestAudiobookFileAndRootpathHelpers(t *testing.T) {
	seedAudiobooks(t)

	if res, err := FindAudiobookByASIN("B0759MNMD2"); err != nil || res.ID != 10 {
		t.Errorf("by ASIN = %+v, %v", res, err)
	}

	if res, err := FindAudiobookByASIN("B07H8Q5SQD"); err != nil || res.ID != 11 {
		t.Errorf("by Audible id = %+v, %v", res, err)
	}

	for _, missing := range []string{"", "B000000000"} {
		if _, err := FindAudiobookByASIN(missing); !errors.Is(err, logger.ErrNotFoundDbaudiobook) {
			t.Errorf("FindAudiobookByASIN(%q) err = %v", missing, err)
		}
	}

	root := "/data/books/Stephen Fry/Mythos"
	file := root + "/Mythos.m4b"

	if AudiobookExistsByRootpath(&root) {
		t.Error("audiobook without files reported as existing")
	}

	if err := InsertAudiobookFile(100, file, "Mythos.m4b", ".m4b", "m4b", "m4b", 500<<20, 64, 54_000_000, 1, 1, 10); err != nil {
		t.Fatal(err)
	}

	if !AudiobookFileExists(&file) || !AudiobookExistsByRootpath(&root) {
		t.Error("inserted file/rootpath not found")
	}

	ab, err := GetAudiobookByRootpath(root)
	if err != nil || ab.ID != 100 || ab.Listname != "Fantasy" {
		t.Errorf("GetAudiobookByRootpath = %+v, %v", ab, err)
	}

	if err := UpdateAudiobookRootpath(100, root+" (2017)"); err != nil {
		t.Fatal(err)
	}

	if _, err := GetAudiobookByRootpath(root); !errors.Is(err, logger.ErrNotFoundAudiobook) {
		t.Errorf("old rootpath still found: %v", err)
	}

	dbid, list := uint(10), "fantasy"
	if got := GetAudiobookListEntryID(&dbid, &list); got != 100 {
		t.Errorf("GetAudiobookListEntryID = %d, want 100", got)
	}
}

func TestFindAudiobooksByTitleAuthor(t *testing.T) {
	db := useMigratedMainDB(t)

	mustExec(t, db, "insert into dbaudiobooks (id, title, slug, series_name, series_position) values (1, 'Children of Dune', 'children-of-dune', '', '')")
	mustExec(t, db, "insert into dbaudiobooks (id, title, slug, series_name, series_position) values (2, 'Dune Messiah', 'dune-messiah', 'Dune', '2')")
	mustExec(t, db, "insert into dbaudiobooks (id, title, slug, series_name, series_position) values (3, 'Dune', 'dune', 'Dune', '1')")

	title := "Dune"

	// nil author with several matches used to dereference the nil pointer.
	res, err := FindAudiobooksByTitleAuthor(&title, nil, 2)
	if err != nil {
		t.Fatal(err)
	}

	if len(res) != 2 || res[0].ID != 3 || res[1].ID != 2 {
		ids := make([]uint, 0, len(res))
		for _, r := range res {
			ids = append(ids, r.ID)
		}

		t.Fatalf("got IDs %v, want [3 2] (exact, then closest LIKE match)", ids)
	}

	if res[0].Series != "Dune" || res[0].SeriesNum != "1" {
		t.Errorf("series not filled: %q #%q", res[0].Series, res[0].SeriesNum)
	}

	lower := "dune"

	one, err := FindAudiobookByTitleAuthor(&lower, nil)
	if err != nil || one.ID != 3 {
		t.Fatalf("FindAudiobookByTitleAuthor(%q) = %+v, %v; want ID 3 (case-insensitive exact match)", lower, one, err)
	}
}

func TestFindAudiobookByTitleAuthorUsesAuthor(t *testing.T) {
	db := useMigratedMainDB(t)

	mustExec(t, db, "insert into dbauthors (id, name, slug) values (1, 'Frank Herbert', 'frank-herbert'), (2, 'Someone Else', 'someone-else')")
	mustExec(t, db, "insert into dbaudiobooks (id, title, slug) values (1, 'Dune', 'dune'), (2, 'Dune', 'dune')")
	mustExec(t, db, "insert into dbaudiobook_authors (dbaudiobook_id, dbauthor_id) values (1, 2), (2, 1)")

	title, author, other := "Dune", "frank herbert", "Nobody Known"

	res, err := FindAudiobookByTitleAuthor(&title, &author)
	if err != nil || res.ID != 2 {
		t.Fatalf("with author: got %+v, %v; want ID 2", res, err)
	}

	if res, err := FindAudiobookByTitleAuthor(&title, &other); !errors.Is(err, logger.ErrNotFoundDbaudiobook) {
		t.Fatalf("unknown author must not fall back to title-only, got %+v", res)
	}

	res, err = FindAudiobookByTitleAuthor(&title, nil)
	if err != nil || res.ID != 1 {
		t.Fatalf("title-only: got %+v, %v; want ID 1", res, err)
	}
}
