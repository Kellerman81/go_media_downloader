package apiexternal

type ImdbCsv struct {
	Position    int    `csv:"Position"`
	Const       string `csv:"const"`
	Created     string `csv:"Created"`
	Modified    string `csv:"Modified"`
	Description string `csv:"Description"`
	Title       string `csv:"Title"`
	URL         string `csv:"URL"`
	TitleType   string `csv:"Title Type"`
	IMDbRating  string `csv:"IMDb Rating"`
	RuntimeMins string `csv:"Runtime (mins)"`
	Year        string `csv:"Year"`
	Genres      string `csv:"Genres"`
	NumVotes    string `csv:"Num Votes"`
	ReleaseDate string `csv:"Release Date"`
	Directors   string `csv:"Directors"`
}
