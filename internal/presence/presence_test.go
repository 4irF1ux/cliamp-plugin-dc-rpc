package presence

import (
	"strings"
	"testing"
	"time"

	"github.com/4irF1ux/cliamp-plugin-discord-rpc/internal/discord"
)

var now = time.Unix(1_800_000_000, 0)

func TestPausedAndStoppedClearTheCard(t *testing.T) {
	for _, status := range []string{"paused", "stopped", ""} {
		s := Snapshot{Status: status, Title: "Song", Artist: "Artist", Duration: 200, Position: 10}
		if a := Activity(s, now); a != nil {
			t.Fatalf("status %q: activity = %+v, want nil", status, a)
		}
	}
}

func TestNoTitleClearsTheCard(t *testing.T) {
	// A hidden stream publishes no title; the card must not show a blank line.
	s := Snapshot{Status: "playing", Duration: 0, Position: 3}
	if a := Activity(s, now); a != nil {
		t.Fatalf("activity = %+v, want nil", a)
	}
}

func TestPlayingIsListeningAnchoredToPosition(t *testing.T) {
	s := Snapshot{Status: "playing", Title: "Song", Artist: "Artist", Duration: 200, Position: 30}
	a := Activity(s, now)
	if a == nil {
		t.Fatal("activity = nil, want Listening")
	}
	if a.Type != discord.TypeListening {
		t.Fatalf("type = %d, want %d", a.Type, discord.TypeListening)
	}
	if a.Details != "Song" || a.State != "Artist" {
		t.Fatalf("details/state = %q/%q", a.Details, a.State)
	}
	wantStart := now.Add(-30 * time.Second).Unix()
	if a.Timestamps.Start != wantStart {
		t.Fatalf("start = %d, want %d (position 30s before now)", a.Timestamps.Start, wantStart)
	}
	if a.Timestamps.End != wantStart+200 {
		t.Fatalf("end = %d, want start+duration %d", a.Timestamps.End, wantStart+200)
	}
}

func TestUnknownDurationOmitsEnd(t *testing.T) {
	s := Snapshot{Status: "playing", Title: "Live", Position: 5}
	a := Activity(s, now)
	if a == nil || a.Timestamps.End != 0 {
		t.Fatalf("activity = %+v, want no end timestamp", a)
	}
}

func TestCoverArtOrFallbackIsLargeImage(t *testing.T) {
	with := Snapshot{Status: "playing", Title: "Song", Image: "https://example.com/cover.jpg"}
	if a := Activity(with, now); a == nil || a.Assets == nil || a.Assets.LargeImage != with.Image {
		t.Fatalf("activity = %+v, want large_image %q", a, with.Image)
	}
	without := Snapshot{Status: "playing", Title: "Song"}
	if a := Activity(without, now); a == nil || a.Assets == nil || a.Assets.LargeImage != FallbackAsset {
		t.Fatalf("activity = %+v, want fallback large_image %q", a, FallbackAsset)
	}
}

func TestLongFieldsAreClippedToDiscordLimit(t *testing.T) {
	long := strings.Repeat("é", maxField+10)
	s := Snapshot{Status: "playing", Title: long, Artist: long, Duration: 60}
	a := Activity(s, now)
	if got := len([]rune(a.Details)); got != maxField {
		t.Fatalf("details runes = %d, want %d", got, maxField)
	}
	if got := len([]rune(a.State)); got != maxField {
		t.Fatalf("state runes = %d, want %d", got, maxField)
	}
}
