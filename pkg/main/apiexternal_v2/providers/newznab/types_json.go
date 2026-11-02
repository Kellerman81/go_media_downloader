package newznab

import "github.com/goccy/go-json"

//
// JSON Response Types - Supporting multiple JSON formats from different Newznab implementations
//

// searchResponseJSON1 represents the first JSON format (with nested channel structure).
type searchResponseJSON1 struct {
	Title   string `json:"title,omitempty"`
	Channel struct {
		Item []json1Item `json:"item"`
	} `json:"channel"`
}

// json1Item represents an item in the first JSON format.
type json1Item struct {
	Title string `json:"title,omitempty"`
	Link  string `json:"link,omitempty"`
	GUID  struct {
		Text string `json:"_text,omitempty"`
	} `json:"guid"`
	Size    int64  `json:"size,omitempty"`
	PubDate string `json:"pubDate,omitempty"`

	Enclosure struct {
		Attributes struct {
			URL    string `json:"url"`
			Length string `json:"length,omitempty"`
		} `json:"@attributes"`
	} `json:"enclosure"`

	Attributes []struct {
		Attribute struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"@attributes"`
	} `json:"attr,omitempty"`
}

// searchResponseJSON2 represents the second JSON format (flat structure).
type searchResponseJSON2 struct {
	Item []json2Item `json:"item"`
}

// json2Item represents an item in the second JSON format.
type json2Item struct {
	Title       string           `json:"title,omitempty"`
	Link        string           `json:"link,omitempty"`
	Size        int64            `json:"size,omitempty"`
	PubDate     string           `json:"pubDate,omitempty"`
	GUID        guidField        `json:"guid"`
	Enclosure   enclosureField   `json:"enclosure"`
	Attributes  []attributeField `json:"newznab:attr,omitempty"`
	Attributes2 []attributeField `json:"nntmux:attr,omitempty"`
}

// guidField represents GUID in JSON format 2.
type guidField struct {
	Text string `json:"_text,omitempty"`
}

// enclosureField represents enclosure in JSON format 2.
type enclosureField struct {
	URL    string `json:"_url,omitempty"`
	Length string `json:"_length,omitempty"`
}

// attributeField represents custom attributes in JSON format 2.
type attributeField struct {
	Name  string `json:"_name"`
	Value string `json:"_value"`
}

// jsonErrorDoc covers the JSON error shapes indexers send with HTTP 200:
// {"error":{"code":..,"description":..}}, the same nested in "@attributes",
// and a bare {"code":..,"description":..}.
type jsonErrorDoc struct {
	Error       *jsonErrorBody `json:"error"`
	Attributes  *jsonErrorBody `json:"@attributes"`
	Code        flexString     `json:"code"`
	Description string         `json:"description"`
}

type jsonErrorBody struct {
	Code        flexString     `json:"code"`
	Description string         `json:"description"`
	Attributes  *jsonErrorBody `json:"@attributes"`
}

func (d *jsonErrorDoc) details() (code, description string, ok bool) {
	for _, b := range []*jsonErrorBody{d.Error, d.Attributes} {
		if b == nil {
			continue
		}

		if b.Attributes != nil {
			b = b.Attributes
		}

		if b.Code != "" || b.Description != "" {
			return string(b.Code), b.Description, true
		}
	}

	if d.Code != "" && d.Description != "" {
		return string(d.Code), d.Description, true
	}

	return "", "", false
}

// flexString accepts a JSON string or number.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}

		*f = flexString(s)

		return nil
	}

	if string(b) == "null" {
		return nil
	}

	*f = flexString(b)

	return nil
}
