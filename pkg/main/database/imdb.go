package database

import (
	"time"
)

type ImdbTitle struct {
	Tconst         string
	TitleType      string `db:"title_type"`
	PrimaryTitle   string `db:"primary_title"`
	Slug           string
	OriginalTitle  string `db:"original_title"`
	IsAdult        bool   `db:"is_adult"`
	StartYear      uint16 `db:"start_year"`
	EndYear        int    `db:"end_year"`
	RuntimeMinutes int    `db:"runtime_minutes"`
	Genres         string
}

type ImdbAka struct {
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
	NumVotes      int32   `db:"num_votes"`
	AverageRating float32 `db:"average_rating"`
}

// type ImdbGenres struct {
// 	ID        uint
// 	CreatedAt time.Time `db:"created_at"`
// 	UpdatedAt time.Time `db:"updated_at"`
// 	Tconst    string
// 	Genre     string
// }
