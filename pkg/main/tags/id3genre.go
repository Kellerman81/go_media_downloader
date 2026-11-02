package tags

import (
	"strconv"
	"strings"
)

// id3v1Genres is the ID3v1 genre table including the Winamp extensions (0-191).
var id3v1Genres = [...]string{
	"Blues", "Classic Rock", "Country", "Dance", "Disco", "Funk", "Grunge", "Hip-Hop",
	"Jazz", "Metal", "New Age", "Oldies", "Other", "Pop", "R&B", "Rap",
	"Reggae", "Rock", "Techno", "Industrial", "Alternative", "Ska", "Death Metal", "Pranks",
	"Soundtrack", "Euro-Techno", "Ambient", "Trip-Hop", "Vocal", "Jazz+Funk", "Fusion", "Trance",
	"Classical", "Instrumental", "Acid", "House", "Game", "Sound Clip", "Gospel", "Noise",
	"Alternative Rock", "Bass", "Soul", "Punk", "Space", "Meditative", "Instrumental Pop", "Instrumental Rock",
	"Ethnic", "Gothic", "Darkwave", "Techno-Industrial", "Electronic", "Pop-Folk", "Eurodance", "Dream",
	"Southern Rock", "Comedy", "Cult", "Gangsta", "Top 40", "Christian Rap", "Pop/Funk", "Jungle",
	"Native American", "Cabaret", "New Wave", "Psychedelic", "Rave", "Showtunes", "Trailer", "Lo-Fi",
	"Tribal", "Acid Punk", "Acid Jazz", "Polka", "Retro", "Musical", "Rock & Roll", "Hard Rock",
	"Folk", "Folk-Rock", "National Folk", "Swing", "Fast Fusion", "Bebop", "Latin", "Revival",
	"Celtic", "Bluegrass", "Avantgarde", "Gothic Rock", "Progressive Rock", "Psychedelic Rock", "Symphonic Rock", "Slow Rock",
	"Big Band", "Chorus", "Easy Listening", "Acoustic", "Humour", "Speech", "Chanson", "Opera",
	"Chamber Music", "Sonata", "Symphony", "Booty Bass", "Primus", "Porn Groove", "Satire", "Slow Jam",
	"Club", "Tango", "Samba", "Folklore", "Ballad", "Power Ballad", "Rhythmic Soul", "Freestyle",
	"Duet", "Punk Rock", "Drum Solo", "A Cappella", "Euro-House", "Dance Hall", "Goa", "Drum & Bass",
	"Club-House", "Hardcore", "Terror", "Indie", "BritPop", "Afro-Punk", "Polsk Punk", "Beat",
	"Christian Gangsta Rap", "Heavy Metal", "Black Metal", "Crossover", "Contemporary Christian", "Christian Rock", "Merengue", "Salsa",
	"Thrash Metal", "Anime", "JPop", "Synthpop", "Abstract", "Art Rock", "Baroque", "Bhangra",
	"Big Beat", "Breakbeat", "Chillout", "Downtempo", "Dub", "EBM", "Eclectic", "Electro",
	"Electroclash", "Emo", "Experimental", "Garage", "Global", "IDM", "Illbient", "Industro-Goth",
	"Jam Band", "Krautrock", "Leftfield", "Lounge", "Math Rock", "New Romantic", "Nu-Breakz", "Post-Punk",
	"Post-Rock", "Psytrance", "Shoegaze", "Space Rock", "Trop Rock", "World Music", "Neoclassical", "Audiobook",
	"Audio Theatre", "Neue Deutsche Welle", "Podcast", "Indie Rock", "G-Funk", "Dubstep", "Garage Rock", "Psybient",
}

// id3GenreRef resolves one ID3 genre reference ("17", "RX", "CR") to its name.
func id3GenreRef(ref string) (string, bool) {
	switch ref {
	case "RX":
		return "Remix", true
	case "CR":
		return "Cover", true
	}

	n, err := strconv.Atoi(ref)
	if err != nil || n < 0 || n >= len(id3v1Genres) {
		return "", false
	}

	return id3v1Genres[n], true
}

// normalizeID3Genre maps the ID3 TCON forms "(17)", "(17)Rock", "17", "(RX)",
// "(CR)" and "(4)(17)" to genre names. A refinement text after the references
// wins over them (ID3v2.3), "((" escapes a literal "(", and free-text genres
// are returned unchanged. ID3v2.4 NUL-separated values are each mapped and
// joined with "/".
func normalizeID3Genre(s string) string {
	if strings.IndexByte(s, 0) >= 0 {
		parts := strings.Split(s, "\x00")
		out := parts[:0]

		for _, p := range parts {
			if g := normalizeID3Genre(p); g != "" {
				out = append(out, g)
			}
		}

		return strings.Join(out, "/")
	}

	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	if g, ok := id3GenreRef(s); ok && s != "RX" && s != "CR" {
		return g
	}

	var names []string

	rest := s
	for strings.HasPrefix(rest, "(") && !strings.HasPrefix(rest, "((") {
		end := strings.IndexByte(rest, ')')
		if end < 0 {
			break
		}

		g, ok := id3GenreRef(rest[1:end])
		if !ok {
			break
		}

		names = append(names, g)
		rest = rest[end+1:]
	}

	if strings.HasPrefix(rest, "((") {
		rest = rest[1:]
	}

	if rest = strings.TrimSpace(rest); rest != "" {
		return rest
	}

	return strings.Join(names, "/")
}
