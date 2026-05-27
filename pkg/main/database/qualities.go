package database

import (
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// Qualities represents type 1 reso 2 qual 3 codec 4 audio.
type Qualities struct {
	Name                 string    `comment:"Quality identifier name"     displayname:"Quality Name"`
	Regex                string    `comment:"Regular expression pattern"  displayname:"Regex Pattern"`
	Strings              string    `comment:"String matching patterns"    displayname:"Match Strings"`
	StringsLower         string    `comment:"Lowercase string patterns"   displayname:"Lowercase Strings"`
	StringsLowerSplitted []string  `comment:"Split lowercase patterns"    displayname:"Split Patterns"    json:"-"`
	CreatedAt            time.Time `comment:"Record creation timestamp"   displayname:"Date Created"               db:"created_at"`
	UpdatedAt            time.Time `comment:"Last modification timestamp" displayname:"Last Updated"               db:"updated_at"`
	QualityType          int       `comment:"Quality category type"       displayname:"Quality Type"               db:"type"`
	Priority             int       `comment:"Quality priority ranking"    displayname:"Priority Level"`
	Regexgroup           int       `comment:"Regex capture group"         displayname:"Regex Group"`
	ID                   uint      `comment:"Unique quality identifier"   displayname:"Quality ID"`
	UseRegex             bool      `comment:"Enable regex matching"       displayname:"Use Regex"                  db:"use_regex"`
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

// GetAudioFormatID looks up the audio format ID from the qualities table (type 5)
// for the given format string. Returns 0 if no match is found.
func GetAudioFormatID(format string) uint {
	for idx := range DBConnect.GetaudioformatsIn {
		qual := &DBConnect.GetaudioformatsIn[idx]
		if qual.Strings != "" && logger.SlicesContainsI(qual.StringsLowerSplitted, format) {
			return qual.ID
		}
	}

	return 0
}
