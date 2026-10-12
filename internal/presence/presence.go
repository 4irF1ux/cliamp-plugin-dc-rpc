// Package presence maps a cliamp playback snapshot to a Discord activity.
package presence

import (
	"time"

	"github.com/4irF1ux/dc-rpc/internal/discord"
)

// FallbackAsset is the key of the Rich Presence art asset uploaded in the
// Discord developer portal. It is shown when the track has no public cover.
const FallbackAsset = "cliamp"

// maxField is Discord's limit for details and state. Longer values are cut.
const maxField = 128

// Snapshot is the playback state published by dc-rpc.lua.
type Snapshot struct {
	Status   string  `json:"status"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Image    string  `json:"image"`
	Duration float64 `json:"duration"`
	Position float64 `json:"position"`
}

// Activity returns the Listening activity for s, or nil when the card must be
// cleared. Only "playing" shows a card: Discord cannot freeze a timer, so pause
// and stop clear it too. The start timestamp is anchored so that elapsed time
// matches the position at now.
func Activity(s Snapshot, now time.Time) *discord.Activity {
	if s.Status != "playing" || s.Title == "" {
		return nil
	}
	start := now.Add(-seconds(s.Position))
	ts := &discord.Timestamps{Start: start.Unix()}
	if s.Duration > 0 {
		ts.End = start.Add(seconds(s.Duration)).Unix()
	}
	large := s.Image
	if large == "" {
		large = FallbackAsset
	}
	assets := &discord.Assets{LargeImage: large}
	return &discord.Activity{
		Type:       discord.TypeListening,
		Details:    clip(s.Title),
		State:      clip(s.Artist),
		Timestamps: ts,
		Assets:     assets,
	}
}

func seconds(f float64) time.Duration {
	return time.Duration(f * float64(time.Second))
}

func clip(s string) string {
	r := []rune(s)
	if len(r) <= maxField {
		return s
	}
	return string(r[:maxField])
}
