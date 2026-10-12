package cliampstate

import "testing"

func TestParseCover(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		want  Track
		isErr bool
	}{
		{
			name: "https cover from track",
			line: `{"version":2,"ok":true,"snapshot":{"track":{"title":"T","artist":"A","album_art_url":"https://i.scdn.co/x"}}}`,
			want: Track{Title: "T", Artist: "A", Cover: "https://i.scdn.co/x"},
		},
		{
			name: "local file cover is dropped",
			line: `{"version":2,"ok":true,"snapshot":{"track":{"title":"T","artist":"A","album_art_url":"file:///tmp/c.jpg"}}}`,
			want: Track{Title: "T", Artist: "A"},
		},
		{
			name: "falls back to logical track",
			line: `{"version":2,"ok":true,"snapshot":{"logical_track":{"title":"T","artist":"A","album_art_url":"https://x/y.jpg"}}}`,
			want: Track{Title: "T", Artist: "A", Cover: "https://x/y.jpg"},
		},
		{
			name:  "no track is an error",
			line:  `{"version":2,"ok":true,"snapshot":{"state":"stopped"}}`,
			isErr: true,
		},
		{
			name:  "not ok is an error",
			line:  `{"version":2,"ok":false}`,
			isErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parse([]byte(tc.line))
			if tc.isErr {
				if err == nil {
					t.Fatalf("parse() = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse() error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("parse() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
