package main

import "testing"

func TestAllowBinary(t *testing.T) {
	const bin = "cliamp-rpcd"
	tests := []struct {
		name    string
		in      string
		want    string
		changed bool
	}{
		{
			name:    "empty file creates section",
			in:      "",
			want:    "[plugins]\nallowed_binaries = \"cliamp-rpcd\"\n",
			changed: true,
		},
		{
			name:    "appends section after existing keys",
			in:      "theme = \"x\"\n",
			want:    "theme = \"x\"\n\n[plugins]\nallowed_binaries = \"cliamp-rpcd\"\n",
			changed: true,
		},
		{
			name:    "appends to existing list",
			in:      "[plugins]\nallowed_binaries = \"yt-dlp\"\n",
			want:    "[plugins]\nallowed_binaries = \"yt-dlp,cliamp-rpcd\"\n",
			changed: true,
		},
		{
			name:    "already listed is unchanged",
			in:      "[plugins]\nallowed_binaries = \"yt-dlp,cliamp-rpcd\"\n",
			changed: false,
			want:    "[plugins]\nallowed_binaries = \"yt-dlp,cliamp-rpcd\"\n",
		},
		{
			name:    "adds key to existing plugins table",
			in:      "[plugins]\n[plugins.dc-rpc]\nenabled = true\n",
			want:    "[plugins]\nallowed_binaries = \"cliamp-rpcd\"\n[plugins.dc-rpc]\nenabled = true\n",
			changed: true,
		},
		{
			name:    "other table with same prefix is not plugins",
			in:      "[plugins.other]\nallowed_binaries = \"ffmpeg\"\n",
			want:    "[plugins.other]\nallowed_binaries = \"ffmpeg\"\n\n[plugins]\nallowed_binaries = \"cliamp-rpcd\"\n",
			changed: true,
		},
		{
			name:    "key in other table is not touched",
			in:      "[spotify]\nallowed_binaries = \"x\"\n[plugins]\n",
			want:    "[spotify]\nallowed_binaries = \"x\"\n[plugins]\nallowed_binaries = \"cliamp-rpcd\"\n",
			changed: true,
		},
		{
			name:    "header with comment is recognized",
			in:      "[plugins] # plugin settings\nallowed_binaries = \"ffmpeg\" # tools\n",
			want:    "[plugins] # plugin settings\nallowed_binaries = \"ffmpeg,cliamp-rpcd\"\n",
			changed: true,
		},
		{
			name:    "CRLF line endings are kept",
			in:      "[plugins]\r\nallowed_binaries = \"yt-dlp\"\r\n",
			want:    "[plugins]\r\nallowed_binaries = \"yt-dlp,cliamp-rpcd\"\r\n",
			changed: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := allowBinary(tc.in, bin)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tc.changed {
				t.Errorf("changed = %v, want %v", changed, tc.changed)
			}
			if got != tc.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

func TestAllowBinaryRefusesNonStringList(t *testing.T) {
	in := "[plugins]\nallowed_binaries = [\"yt-dlp\"]\n"
	if _, _, err := allowBinary(in, "cliamp-rpcd"); err == nil {
		t.Fatal("expected error for array value, got nil")
	}
}
