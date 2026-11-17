package api

import (
	"slices"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// useTestQualities installs a subset of the production qualities table.
func useTestQualities(t *testing.T) {
	t.Helper()

	prev := database.DBConnect
	t.Cleanup(func() { database.DBConnect = prev })

	q := func(entries ...any) []database.Qualities {
		var out []database.Qualities
		for i := 0; i < len(entries); i += 2 {
			out = append(out, database.Qualities{ID: uint(i/2 + 1), Name: entries[i].(string), Priority: entries[i+1].(int)})
		}

		return out
	}

	database.DBConnect.GetresolutionsIn = q("2160p", 70000, "1080p", 60000, "720p", 50000, "576p", 40000, "480p", 30000)
	database.DBConnect.GetqualitiesIn = q("bluray", 9300, "dvdrip", 9000, "webdl", 8300, "hdtv", 8000, "sdtv", 6000)
	database.DBConnect.GetcodecsIn = q("h265", 500, "h264", 300, "xvid", 200)
	database.DBConnect.GetaudiosIn = q("dts", 80, "ac3", 40, "aac", 20)
	database.DBConnect.GetaudioformatsIn = q("FLAC", 100, "AAC", 50, "MP3", 40)
}

// sdProfile mirrors the production SD profile: quality outranks resolution
// (position x110), 1080p/720p rank below an undetected resolution.
func sdProfile() *config.QualityConfig {
	return &config.QualityConfig{
		Name:                     "SD",
		WantedResolution:         []string{"480p", "576p", "720p", "1080p"},
		WantedQuality:            []string{"sdtv", "hdtv", "webdl", "dvdrip", "bluray"},
		UseForPriorityResolution: true,
		UseForPriorityQuality:    true,
		QualityReorder: []config.QualityReorderConfig{
			{Name: "quality", ReorderType: "position", Newpriority: 110},
			{Name: "1080p", ReorderType: "resolution", Newpriority: -1000000},
			{Name: "720p", ReorderType: "resolution", Newpriority: -500000},
		},
	}
}

func orderOf(q *config.QualityConfig, key string) []string {
	return entryNames(qpEntries(q, key))
}

func valueOf(q *config.QualityConfig, key, name string) int {
	for _, e := range qpEntries(q, key) {
		if e.Name == name {
			return e.Value
		}
	}

	return 0
}

func TestQualityPrioritiesLists(t *testing.T) {
	useTestQualities(t)

	q := sdProfile()

	lists := qpLists(q)
	if len(lists) != 1 || lists[0].Key != qpPairKey {
		t.Fatalf("lists = %+v; want only the resolution+source pairs (codec/audio off)", lists)
	}

	pairs := orderOf(q, qpPairKey)
	if len(pairs) != 4*5 || pairs[0] != "576p,bluray" || pairs[len(pairs)-1] != "1080p,sdtv" {
		t.Errorf("pairs = %v; want 20, 576p bluray first, 1080p sdtv last", pairs)
	}

	music := &config.QualityConfig{Name: "Music", UseForPriorityAudioFormat: true, QualityReorder: []config.QualityReorderConfig{
		{Name: "MP3", ReorderType: "audio_format", Newpriority: 110},
	}}
	if l := qpLists(music); len(l) != 1 || l[0].Key != "audio_format" {
		t.Errorf("music lists = %+v; want audio_format", l)
	}

	// Without the switch the parser ignores audio formats: no list to order.
	if l := qpLists(&config.QualityConfig{Name: "Off", QualityReorder: music.QualityReorder}); len(l) != 0 {
		t.Errorf("lists without Use Audio Format For Priority = %+v", l)
	}

	if got := orderOf(music, "audio_format"); !slices.Equal(got, []string{"MP3", "FLAC", "AAC"}) {
		t.Errorf("music order = %v", got)
	}
}

func TestQualityPrioritiesBuildRules(t *testing.T) {
	useTestQualities(t)

	q := sdProfile()
	order := orderOf(q, qpPairKey)

	// Unchanged: no rules, the profile stays as it is.
	merged, generated, err := qpBuildRules(q, map[string][]string{qpPairKey: order})
	if err != nil {
		t.Fatal(err)
	}

	if len(generated) != 0 || !slices.Equal(merged, q.QualityReorder) {
		t.Errorf("unchanged order: generated %+v, merged %+v", generated, merged)
	}

	// Move 720p bluray from the lower half to the top.
	moved := slices.DeleteFunc(slices.Clone(order), func(s string) bool { return s == "720p,bluray" })
	moved = append([]string{"720p,bluray"}, moved...)

	merged, generated, err = qpBuildRules(q, map[string][]string{qpPairKey: moved})
	if err != nil {
		t.Fatal(err)
	}

	after := *q
	after.QualityReorder = merged

	if got := orderOf(&after, qpPairKey); !slices.Equal(got, moved) {
		t.Errorf("order after rules = %v, want %v", got, moved)
	}

	// Only the moved entry gets a rule; everything else keeps its value.
	if len(generated) != 1 || generated[0].Name != "720p,bluray" || generated[0].ReorderType != "combined_res_qual" ||
		generated[0].Newpriority <= valueOf(q, qpPairKey, order[0]) {
		t.Errorf("generated %+v; want one combined rule above %d", generated, valueOf(q, qpPairKey, order[0]))
	}

	if !slices.Equal(merged[:3], q.QualityReorder) {
		t.Errorf("existing rules not kept in front: %+v", merged[:3])
	}

	// Moving it between two neighbours puts it between their values.
	between := slices.DeleteFunc(slices.Clone(order), func(s string) bool { return s == "720p,bluray" })
	between = slices.Insert(between, 3, "720p,bluray")

	_, generatedBetween, err := qpBuildRules(q, map[string][]string{qpPairKey: between})
	if err != nil {
		t.Fatal(err)
	}

	if hi, lo := valueOf(q, qpPairKey, between[2]), valueOf(q, qpPairKey, between[4]); len(generatedBetween) != 1 ||
		generatedBetween[0].Newpriority >= hi || generatedBetween[0].Newpriority <= lo {
		t.Errorf("between %d and %d: %+v", hi, lo, generatedBetween)
	}

	// Moving it back produces the original rules again (the editor's own rules
	// are replaced, not stacked).
	merged2, generated2, err := qpBuildRules(&after, map[string][]string{qpPairKey: order})
	if err != nil {
		t.Fatal(err)
	}

	if len(generated2) != 0 || !slices.Equal(merged2, q.QualityReorder) {
		t.Errorf("moving back: generated %+v, merged %+v", generated2, merged2)
	}

	if _, _, err := qpBuildRules(q, map[string][]string{qpPairKey: order[1:]}); err == nil {
		t.Error("missing entry: no error")
	}
}

func TestQualityPrioritiesSingleList(t *testing.T) {
	useTestQualities(t)

	q := &config.QualityConfig{Name: "HD", UseForPriorityCodec: true, QualityReorder: []config.QualityReorderConfig{
		{Name: "xvid", ReorderType: "codec", Newpriority: 1000},
	}}

	if got := orderOf(q, "codec"); !slices.Equal(got, []string{"xvid", "h265", "h264"}) {
		t.Fatalf("codec order = %v", got)
	}

	// h264 to the top: one typical step ((1000-300)/2) above xvid; xvid's
	// existing rule is re-created, h265 needs none.
	merged, generated, err := qpBuildRules(q, map[string][]string{"codec": {"h264", "xvid", "h265"}})
	if err != nil {
		t.Fatal(err)
	}

	want := []config.QualityReorderConfig{
		{Name: "h264", ReorderType: "codec", Newpriority: 1350},
		{Name: "xvid", ReorderType: "codec", Newpriority: 1000},
	}
	if !slices.Equal(generated, want) || !slices.Equal(merged, want) {
		t.Errorf("generated %+v, merged %+v; want %+v", generated, merged, want)
	}
}

func TestQualityPrioritiesTies(t *testing.T) {
	useTestQualities(t)

	q := &config.QualityConfig{Name: "T", UseForPriorityAudio: true, QualityReorder: []config.QualityReorderConfig{
		{Name: "aac", ReorderType: "audio", Newpriority: 40}, // ties with ac3
	}}

	order := orderOf(q, "audio")

	// The existing aac rule is re-created as it was.
	if merged, _, err := qpBuildRules(q, map[string][]string{"audio": order}); err != nil || !slices.Equal(merged, q.QualityReorder) {
		t.Errorf("untouched tie: %+v, %v", merged, err)
	}

	// Swap the tied pair: the new second one must end up strictly lower.
	swapped := []string{order[0], order[2], order[1]}

	merged, _, err := qpBuildRules(q, map[string][]string{"audio": swapped})
	if err != nil {
		t.Fatal(err)
	}

	after := *q
	after.QualityReorder = merged

	if got := orderOf(&after, "audio"); !slices.Equal(got, swapped) ||
		valueOf(&after, "audio", swapped[1]) <= valueOf(&after, "audio", swapped[2]) {
		t.Errorf("after swapping a tie: %v (%+v)", got, merged)
	}
}
