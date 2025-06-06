package database

import (
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// type 1 reso 2 qual 3 codec 4 audio.
type Qualities struct {
	Name         string
	Regex        string
	Strings      string
	StringsLower string
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
	QualityType  int       `db:"type"`
	Priority     int
	Regexgroup   int
	ID           uint
	UseRegex     bool `db:"use_regex"`
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
