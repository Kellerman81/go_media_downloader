package database

import (
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
)

type ImdbTitle struct {
	Tconst         string
	TitleType      string `db:"title_type"`
	PrimaryTitle   string `db:"primary_title"`
	Slug           string
	OriginalTitle  string `db:"original_title"`
	IsAdult        bool   `db:"is_adult"`
	StartYear      int    `db:"start_year"`
	EndYear        int    `db:"end_year"`
	RuntimeMinutes int    `db:"runtime_minutes"`
	Genres         string
}

type imdbAka struct {
	ID              uint
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
	Tconst          string
	Ordering        int
	Title           string
	Slug            string
	Region          string
	Language        string
	Types           string
	Attributes      string
	IsOriginalTitle bool `db:"is_original_title"`
}

type ImdbRatings struct {
	ID            uint
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
	Tconst        string
	NumVotes      int     `db:"num_votes"`
	AverageRating float32 `db:"average_rating"`
}

// type ImdbGenres struct {
// 	ID        uint
// 	CreatedAt time.Time `db:"created_at"`
// 	UpdatedAt time.Time `db:"updated_at"`
// 	Tconst    string
// 	Genre     string
// }

// Close cleans up the imdbTitle struct by zeroing out fields when variable cleanup is enabled.
// This allows the struct to be garbage collected and avoids potential memory leaks.
func (s *ImdbTitle) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || s == nil {
		return
	}
	*s = ImdbTitle{}
}

// Close cleans up the imdbRatings struct by zeroing it after use.
// This avoids leaving sensitive data in memory.
func (s *ImdbRatings) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || s == nil {
		return
	}
	*s = ImdbRatings{}
}
