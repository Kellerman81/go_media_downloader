package newznab

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
