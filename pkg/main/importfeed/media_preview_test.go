package importfeed

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func TestScoreMediaCandidate(t *testing.T) {
	movie := &MediaQuery{IsType: config.MediaTypeMovie, Title: "Wonderland", Year: 2024, Imdb: "tt29245904"}

	// The file's IMDb id decides: a translated title and a year off by one
	// still leave the right movie strong.
	same := &MediaCandidate{Title: "Wunderland", Year: 2023, ExternalID: "tt29245904"}
	scoreMediaCandidate(movie, same)

	if same.Dist >= strongRecThresh*3 {
		t.Errorf("id match scored %.3f", same.Dist)
	}

	other := &MediaCandidate{Title: "Wonderland", Year: 2024, ExternalID: "tt31254554"}
	scoreMediaCandidate(movie, other)

	if other.Dist <= same.Dist {
		t.Errorf("other id (%.3f) scored not worse than the id match (%.3f)", other.Dist, same.Dist)
	}

	// Without an id the title and year count; an alternate title can match.
	noID := &MediaQuery{IsType: config.MediaTypeMovie, Title: "Der Pate", Year: 1972}
	alt := &MediaCandidate{Title: "The Godfather", Year: 1972, altTitles: []string{"Der Pate"}}
	scoreMediaCandidate(noID, alt)

	if alt.Dist != 0 || alt.MatchedAs != "Der Pate" {
		t.Errorf("alternate title: dist %.3f, matched as %q", alt.Dist, alt.MatchedAs)
	}

	// Episodes of a series not yet in the database are unknown, so such a
	// candidate cannot score strong even with a perfect title.
	ep := &MediaQuery{IsType: config.MediaTypeSeries, Title: "Ghosts", Season: "2", Episodes: []int{12}}
	external := &MediaCandidate{Title: "Ghosts", Episodes: []MediaEpisode{{Identifier: "S02E12"}}}
	scoreMediaCandidate(ep, external)

	if external.Dist < strongRecThresh {
		t.Errorf("series with unknown episodes scored strong (%.3f)", external.Dist)
	}

	known := &MediaCandidate{DBID: 1, Title: "Ghosts", Episodes: []MediaEpisode{{Identifier: "S02E12", DBEpisodeID: 7}}}
	scoreMediaCandidate(ep, known)

	if known.Dist != 0 {
		t.Errorf("series with the episode scored %.3f", known.Dist)
	}
}

func TestMediaCandidateSet(t *testing.T) {
	var empty *MediaCandidateSet
	if empty.Best() != nil || empty.Candidate("x") != nil || empty.Matcher() != nil {
		t.Fatal("nil set returned a candidate")
	}

	set := &MediaCandidateSet{Candidates: []*MediaCandidate{{Key: "a"}, {Key: "b", Matcher: true}}}
	if set.Best().Key != "a" || set.Candidate("b") == nil || set.Matcher().Key != "b" {
		t.Error("set lookups returned the wrong candidate")
	}
}
