package newznab

import (
	"time"
)

// NZB represents an NZB found on the index
type NZB struct {
	ID          string    `json:"id,omitempty"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Size        int64     `json:"size,omitempty"`
	AirDate     time.Time `json:"air_date,omitempty"`
	PubDate     time.Time `json:"pub_date,omitempty"`
	UsenetDate  time.Time `json:"usenet_date,omitempty"`
	NumGrabs    int       `json:"num_grabs,omitempty"`

	SourceEndpoint string `json:"source_endpoint"`
	SourceAPIKey   string `json:"source_apikey"`

	Category []string `json:"category,omitempty"`
	Info     string   `json:"info,omitempty"`
	Genre    string   `json:"genre,omitempty"`

	Resolution string `json:"resolution,omitempty"`
	Poster     string `json:"poster,omitempty"`
	Group      string `json:"group,omitempty"`

	// TV Specific stuff
	TVDBID  string `json:"tvdbid,omitempty"`
	Season  string `json:"season,omitempty"`
	Episode string `json:"episode,omitempty"`
	TVTitle string `json:"tvtitle,omitempty"`
	Rating  int    `json:"rating,omitempty"`

	// Movie Specific stuff
	IMDBID    string  `json:"imdb,omitempty"`
	IMDBTitle string  `json:"imdbtitle,omitempty"`
	IMDBYear  int     `json:"imdbyear,omitempty"`
	IMDBScore float32 `json:"imdbscore,omitempty"`
	CoverURL  string  `json:"coverurl,omitempty"`

	// Torznab specific stuff
	Seeders     int    `json:"seeders,omitempty"`
	Peers       int    `json:"peers,omitempty"`
	InfoHash    string `json:"infohash,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	IsTorrent   bool   `json:"is_torrent,omitempty"`
}

// SearchResponse is a RSS version of the response.
type SearchResponse struct {
	NZBs []RawNZB `xml:"channel>item"`
}

type SearchResponseJson1 struct {
	Title   string `json:"title,omitempty"`
	Channel struct {
		Item []RawNZBJson1 `json:"item"`
	} `json:"channel"`
}
type SearchResponseJson2 struct {
	Item []RawNZBJson2 `json:"item"`
}

// RawNZB represents a single NZB item in search results.
type RawNZB struct {
	Title string `xml:"title,omitempty"`
	Link  string `xml:"link,omitempty"`
	Size  int64  `xml:"size,omitempty"`

	GUID struct {
		GUID string `xml:",chardata"`
	} `xml:"guid,omitempty"`

	Source struct {
		URL string `xml:"url,attr"`
	} `xml:"source,omitempty"`

	Date string `xml:"pubDate,omitempty"`

	Enclosure struct {
		URL string `xml:"url,attr"`
	} `xml:"enclosure,omitempty"`

	Attributes []struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	} `xml:"attr"`
}

type RawNZBJson1 struct {
	Title     string `json:"title,omitempty"`
	Link      string `json:"link,omitempty"`
	Guid      string `json:"guid,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Date      string `json:"pubDate,omitempty"`
	Enclosure struct {
		Attributes struct {
			URL string `json:"url"`
		} `json:"@attributes,omitempty"`
	} `json:"enclosure,omitempty"`

	Attributes []struct {
		Attribute struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"@attributes,omitempty"`
	} `json:"attr,omitempty"`
}

type RawNZBJson2 struct {
	Title string `json:"title,omitempty"`
	Link  string `json:"link,omitempty"`
	Size  int64  `json:"size,omitempty"`
	GUID  struct {
		GUID string `json:"text,omitempty"`
	} `json:"guid,omitempty"`
	Date      string `json:"pubDate,omitempty"`
	Enclosure struct {
		URL string `json:"_url"`
	} `json:"enclosure,omitempty"`

	Attributes []struct {
		Name  string `json:"_name"`
		Value string `json:"_value"`
	} `json:"newznab:attr,omitempty"`
	Attributes2 []struct {
		Name  string `json:"_name"`
		Value string `json:"_value"`
	} `json:"nntmux:attr,omitempty"`
}
