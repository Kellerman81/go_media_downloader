package newznab

import (
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"testing"
)

func TestBuildURL(t *testing.T) {
	qual := &config.QualityConfig{
		Name:    "q",
		Indexer: []config.QualityIndexerConfig{{TemplateIndexer: "ind", CategoriesIndexer: "2000"}},
	}

	tests := []struct {
		name      string
		rss       bool
		indexerid int
		row       config.IndexersConfig
		want      string
	}{
		{
			name: "default api",
			row:  config.IndexersConfig{Name: "ind", URL: "http://h", Apikey: "k"},
			want: "http://h/api?apikey=k&cat=2000&dl=1&t=search",
		},
		{
			name:      "missing indexer drops category but still searches",
			indexerid: -1,
			row:       config.IndexersConfig{Name: "ind", URL: "http://h", Apikey: "k"},
			want:      "http://h/api?apikey=k&dl=1&t=search",
		},
		{
			name:      "out of range indexer id",
			indexerid: 5,
			row:       config.IndexersConfig{Name: "ind", URL: "http://h", Apikey: "k"},
			want:      "http://h/api?apikey=k&dl=1&t=search",
		},
		{
			name: "custom url without query",
			row:  config.IndexersConfig{Name: "ind", Customurl: "http://c/search"},
			want: "http://c/search?dl=1&cat=2000&t=search",
		},
		{
			name: "custom url with query",
			row:  config.IndexersConfig{Name: "ind", Customurl: "http://c/search?x=1"},
			want: "http://c/search?x=1&cat=2000&dl=1&t=search",
		},
		{
			name: "custom url path below indexer url gets the api key",
			row:  config.IndexersConfig{Name: "ind", URL: "http://h/", Apikey: "k", Customurl: "/newznab/api"},
			want: "http://h/newznab/api?apikey=k&cat=2000&dl=1&t=search",
		},
		{
			name: "custom url path with custom api parameter",
			row: config.IndexersConfig{
				Name: "ind", URL: "http://h", Apikey: "k", Customapi: "token", Customurl: "api/v2",
			},
			want: "http://h/api/v2?token=k&cat=2000&dl=1&t=search",
		},
		{
			name: "full custom url gets the api key",
			row:  config.IndexersConfig{Name: "ind", URL: "http://h", Apikey: "k", Customurl: "https://c/api?x=1"},
			want: "https://c/api?x=1&apikey=k&cat=2000&dl=1&t=search",
		},
		{
			name: "full custom url already carrying the key",
			row:  config.IndexersConfig{Name: "ind", URL: "http://h", Apikey: "k", Customurl: "https://c/api?apikey=zz"},
			want: "https://c/api?apikey=zz&cat=2000&dl=1&t=search",
		},
		{
			name: "custom rss path below indexer url",
			rss:  true,
			row:  config.IndexersConfig{Name: "ind", URL: "http://h", Customrssurl: "/feed"},
			want: "http://h/feed?dl=1&t=2000&t=search",
		},
		{
			name: "customapi rss adds user id",
			rss:  true,
			row: config.IndexersConfig{
				Name: "ind", URL: "http://h", Apikey: "k", Customapi: "key", Userid: "u",
			},
			want: "http://h/rss?key=k&i=u&t=2000&dl=1&t=search",
		},
	}

	for _, tt := range tests {
		got := buildURL(tt.rss, tt.indexerid, qual, &tt.row, []byte("&t=search"))
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
