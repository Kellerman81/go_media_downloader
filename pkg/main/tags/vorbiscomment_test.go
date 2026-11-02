package tags

import (
	"testing"
)

func TestParseVorbisCommentListMalformed(t *testing.T) {
	bad := [][]byte{
		{},
		{0xFF, 0xFF, 0xFF, 0xFF},
		{0, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0xFF}, // hostile count
		{0, 0, 0, 0, 1, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0xFF}, // comment length past end
	}

	for i, b := range bad {
		if _, _, _, ok := parseVorbisCommentList(b); ok {
			t.Errorf("case %d: expected !ok", i)
		}
	}
}

func TestApplyVorbisCommentTotals(t *testing.T) {
	cases := []struct {
		comments              [][2]string
		track, tTot, disc, dT int
	}{
		{[][2]string{{"TRACKNUMBER", "3/12"}, {"DISCNUMBER", "1/2"}}, 3, 12, 1, 2},
		{[][2]string{{"TRACKTOTAL", "14"}, {"TRACKNUMBER", "3/12"}}, 3, 14, 0, 0},
		{[][2]string{{"TRACKNUMBER", "3/12"}, {"TRACKTOTAL", " 14 "}}, 3, 14, 0, 0},
		{[][2]string{{"TRACKNUMBER", " 7 "}, {"TRACKC", "9"}, {"DISCC", "3"}}, 7, 9, 0, 3},
	}

	for i, c := range cases {
		var tags AudioTags
		for _, kv := range c.comments {
			applyVorbisComment(kv[0], kv[1], &tags)
		}

		if tags.TrackNumber != c.track || tags.TotalTracks != c.tTot ||
			tags.DiscNumber != c.disc || tags.TotalDiscs != c.dT {
			t.Errorf("case %d: got %d/%d %d/%d", i, tags.TrackNumber, tags.TotalTracks,
				tags.DiscNumber, tags.TotalDiscs)
		}
	}

	var tags AudioTags

	(&OGGHandler{}).setTagValue("LABELNO", "CAT-1", &tags)

	if tags.CatalogNum != "CAT-1" {
		t.Error("OGG does not accept LABELNO alias")
	}
}
