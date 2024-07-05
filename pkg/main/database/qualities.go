package database

import (
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// type 1 reso 2 qual 3 codec 4 audio
type Qualities struct {
	ID           uint
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
	QualityType  int       `db:"type"`
	Name         string
	Regex        string
	Strings      string
	StringsLower string
	Priority     int
	UseRegex     bool `db:"use_regex"`
	Regexgroup   int
}

// gettypeidprioritysingle returns the priority for the given Qualities struct
// after applying any reorder rules that match the given quality string type and name.
// It checks each QualityReorderConfig in the reordergroup, looking for matches on
// ReorderType and Name. If found, it will update the priority based on Newpriority.
func (qual *Qualities) Gettypeidprioritysingle(qualitystringtype string, reordergroup []config.QualityReorderConfig) int {
	priority := qual.Priority
	for idxreorder := range reordergroup {
		if (reordergroup[idxreorder].ReorderType == qualitystringtype || strings.EqualFold(reordergroup[idxreorder].ReorderType, qualitystringtype)) && (reordergroup[idxreorder].Name == qual.Name || strings.EqualFold(reordergroup[idxreorder].Name, qual.Name)) {
			priority = reordergroup[idxreorder].Newpriority
		}
		if (reordergroup[idxreorder].ReorderType == "position" || strings.EqualFold(reordergroup[idxreorder].ReorderType, "position")) && (reordergroup[idxreorder].Name == qualitystringtype || strings.EqualFold(reordergroup[idxreorder].Name, qualitystringtype)) {
			priority *= reordergroup[idxreorder].Newpriority
		}
	}
	return priority
}
