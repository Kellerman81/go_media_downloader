package structure

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

func TestPlanRootpathRepairs(t *testing.T) {
	rows := []database.DbstaticTwoStringOneRInt{
		// Correct already.
		{Str1: "/m/A/Artist/Album (2001)", Str2: "/m/A/Artist/Album (2001)/01.mp3", Num: 1},
		{Str1: "/m/A/Artist/Album (2001)", Str2: "/m/A/Artist/Album (2001)/02.mp3", Num: 1},
		// Artist folder stored by the old scan: repaired to the album folder.
		{Str1: "/m/J/Joe Colley", Str2: "/m/J/Joe Colley/Trance Tapes (2021)/01.mp3", Num: 2},
		// Empty rootpath gets filled.
		{Str1: "", Str2: "/m/B/Band/Live (1990)/01.flac", Num: 3},
		// Multi-disc: the album folder above CD1/CD2 is already right.
		{Str1: "/m/M/Moby/Play (1999)", Str2: "/m/M/Moby/Play (1999)/CD1/01.mp3", Num: 4},
		{Str1: "/m/M/Moby/Play (1999)", Str2: "/m/M/Moby/Play (1999)/CD2/01.mp3", Num: 4},
		// Files in two different album folders: left alone.
		{Str1: "/m/L/La Caina/Vue Mer (2006)", Str2: "/m/L/La Caina/Vue Mer (2006)/01.mp3", Num: 5},
		{Str1: "/m/L/La Caina/Vue Mer (2006)", Str2: "/m/L/La Caina/Other (2006)/01.mp3", Num: 5},
		// Album folder named like a disc directly below the artist.
		{Str1: "/m/S/Santana/Moonflower (Disc 1)", Str2: "/m/S/Santana/Moonflower (Disc 1)/01.flac", Num: 6},
	}

	changes, ambiguous := planRootpathRepairs("albums", rows, audioFileFolder)
	if ambiguous != 1 {
		t.Errorf("ambiguous = %d, want 1", ambiguous)
	}

	want := map[uint]string{2: "/m/J/Joe Colley/Trance Tapes (2021)", 3: "/m/B/Band/Live (1990)"}
	if len(changes) != len(want) {
		t.Fatalf("changes = %+v, want %v", changes, want)
	}

	for _, c := range changes {
		if want[c.ID] != c.New || c.Table != "albums" {
			t.Errorf("change %+v, want new %q", c, want[c.ID])
		}
	}
}

func TestPlanRootpathRepairsBooks(t *testing.T) {
	rows := []database.DbstaticTwoStringOneRInt{
		// Author folder stored by the old first-folder scan.
		{Str1: "/books/Tolkien", Str2: "/books/Tolkien/The Hobbit (1937)/The Hobbit.epub", Num: 1},
		// Already the book folder; a second format beside it changes nothing.
		{Str1: "/books/Tolkien/Silmarillion", Str2: "/books/Tolkien/Silmarillion/s.epub", Num: 2},
		{Str1: "/books/Tolkien/Silmarillion", Str2: "/books/Tolkien/Silmarillion/s.pdf", Num: 2},
		// Loose file in the library root: no book folder, left alone.
		{Str1: "/books/loose.epub", Str2: "/books/loose.epub", Num: 3},
		// A disc-like folder name is not special for books.
		{Str1: "", Str2: "/books/Author/Part 1/p.epub", Num: 4},
	}

	changes, ambiguous := planRootpathRepairs("books", rows, bookFileFolder([]string{"/books"}))
	if ambiguous != 0 {
		t.Errorf("ambiguous = %d, want 0", ambiguous)
	}

	want := map[uint]string{1: "/books/Tolkien/The Hobbit (1937)", 4: "/books/Author/Part 1"}
	if len(changes) != len(want) {
		t.Fatalf("changes = %+v, want %v", changes, want)
	}

	for _, c := range changes {
		if want[c.ID] != c.New || c.Table != "books" {
			t.Errorf("change %+v, want new %q", c, want[c.ID])
		}
	}
}
