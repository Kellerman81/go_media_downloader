package database

import (
	"time"
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
}
