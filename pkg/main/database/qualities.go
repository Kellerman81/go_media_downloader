package database

import (
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// type 1 reso 2 qual 3 codec 4 audio.
type Qualities struct {
	Name                 string    `displayname:"Quality Name"      comment:"Quality identifier name"`
	Regex                string    `displayname:"Regex Pattern"     comment:"Regular expression pattern"`
	Strings              string    `displayname:"Match Strings"     comment:"String matching patterns"`
	StringsLower         string    `displayname:"Lowercase Strings" comment:"Lowercase string patterns"`
	StringsLowerSplitted []string  `displayname:"Split Patterns"    comment:"Split lowercase patterns"    json:"-"`
	CreatedAt            time.Time `displayname:"Date Created"      comment:"Record creation timestamp"            db:"created_at"`
	UpdatedAt            time.Time `displayname:"Last Updated"      comment:"Last modification timestamp"          db:"updated_at"`
	QualityType          int       `displayname:"Quality Type"      comment:"Quality category type"                db:"type"`
	Priority             int       `displayname:"Priority Level"    comment:"Quality priority ranking"`
	Regexgroup           int       `displayname:"Regex Group"       comment:"Regex capture group"`
	ID                   uint      `displayname:"Quality ID"        comment:"Unique quality identifier"`
	UseRegex             bool      `displayname:"Use Regex"         comment:"Enable regex matching"                db:"use_regex"`
}

// Gettypeidprioritysingle returns the priority for the given Qualities struct
// after applying any reorder rules that match the given quality string type and name.
// It checks each QualityReorderConfig in the reordergroup, looking for matches on
// ReorderType and Name. If found, it will update the priority based on Newpriority.
func (qual *Qualities) Gettypeidprioritysingle(
	qualitystringtype string,
	reorder *config.QualityConfig,
) int {
	priority := qual.Priority
	for idx := range reorder.QualityReorder {
		if (reorder.QualityReorder[idx].ReorderType == qualitystringtype || strings.EqualFold(reorder.QualityReorder[idx].ReorderType, qualitystringtype)) &&
			(reorder.QualityReorder[idx].Name == qual.Name || strings.EqualFold(reorder.QualityReorder[idx].Name, qual.Name)) {
			priority = reorder.QualityReorder[idx].Newpriority
		}

		if (reorder.QualityReorder[idx].ReorderType == "position" || strings.EqualFold(reorder.QualityReorder[idx].ReorderType, "position")) &&
			(reorder.QualityReorder[idx].Name == qualitystringtype || strings.EqualFold(reorder.QualityReorder[idx].Name, qualitystringtype)) {
			priority *= reorder.QualityReorder[idx].Newpriority
		}
	}

	return priority
}
