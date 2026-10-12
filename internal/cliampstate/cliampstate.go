// Package cliampstate asks cliamp's IPC socket which track it is playing. The
// playback event does not carry the cover URL; the track in state.get does.
//
// Protocol (docs/plugins.md, IPC v2): send one NDJSON request, read one NDJSON
// response with the runtime snapshot.
package cliampstate

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"time"
)

const (
	dialTimeout = 3 * time.Second
	// maxLine bounds the response, matching cliampsub's line cap.
	maxLine = 128 << 10
)

// Track is the track cliamp is playing.
type Track struct {
	Title  string
	Artist string
	// Cover is the track's cover URL when it is a public https URL, else "".
	// Discord cannot fetch local files, so other schemes are dropped.
	Cover string
}

type trackInfo struct {
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	AlbumArtURL string `json:"album_art_url"`
}

type response struct {
	OK       bool `json:"ok"`
	Snapshot struct {
		Track        *trackInfo `json:"track"`
		LogicalTrack *trackInfo `json:"logical_track"`
	} `json:"snapshot"`
}

// Current returns the track cliamp is playing, read with one state.get request
// on the socket at sock.
func Current(sock string) (Track, error) {
	conn, err := net.DialTimeout("unix", sock, dialTimeout)
	if err != nil {
		return Track{}, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(dialTimeout)); err != nil {
		return Track{}, err
	}
	if _, err := conn.Write([]byte(`{"version":2,"id":"cover","method":"state.get"}` + "\n")); err != nil {
		return Track{}, err
	}
	line, err := bufio.NewReader(io.LimitReader(conn, maxLine)).ReadBytes('\n')
	if err != nil {
		return Track{}, err
	}
	return parse(line)
}

func parse(line []byte) (Track, error) {
	var r response
	if err := json.Unmarshal(line, &r); err != nil {
		return Track{}, err
	}
	if !r.OK {
		return Track{}, errors.New("state.get: not ok")
	}
	t := r.Snapshot.Track
	if t == nil {
		t = r.Snapshot.LogicalTrack
	}
	if t == nil {
		return Track{}, errors.New("state.get: no track")
	}
	cover := ""
	if strings.HasPrefix(t.AlbumArtURL, "https://") {
		cover = t.AlbumArtURL
	}
	return Track{Title: t.Title, Artist: t.Artist, Cover: cover}, nil
}
