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
	Genres         string
	EndYear        int    `db:"end_year"`
	RuntimeMinutes int    `db:"runtime_minutes"`
	StartYear      uint16 `db:"start_year"`
	IsAdult        bool   `db:"is_adult"`
}

type ImdbAka struct {
	Tconst          string
	Title           string
	Slug            string
	Region          string
	Language        string
	Types           string
	Attributes      string
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
	Ordering        int
	ID              uint
	IsOriginalTitle bool `db:"is_original_title"`
}

type ImdbRatings struct {
	Tconst        string
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
	AverageRating float32   `db:"average_rating"`
	ID            uint
	NumVotes      int32 `db:"num_votes"`
}

// type ImdbGenres struct {
// 	ID        uint
// 	CreatedAt time.Time `db:"created_at"`
// 	UpdatedAt time.Time `db:"updated_at"`
// 	Tconst    string
// 	Genre     string
// }
