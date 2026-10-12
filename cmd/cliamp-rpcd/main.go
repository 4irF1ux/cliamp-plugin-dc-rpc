// Command cliamp-rpcd shows cliamp playback as Discord Rich Presence.
//
// It subscribes to the "playback" snapshots that discord-rpc.lua publishes on
// cliamp's IPC socket and mirrors them to the local Discord client.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/4irF1ux/cliamp-plugin-discord-rpc/internal/cliampsub"
	"github.com/4irF1ux/cliamp-plugin-discord-rpc/internal/discord"
	"github.com/4irF1ux/cliamp-plugin-discord-rpc/internal/presence"
)

// topic is "plugin.<plugin file name>.<published name>" (docs/plugins.md).
const topic = "plugin.discord-rpc.playback"

// retryEvery is how often a failed Discord update is retried.
const retryEvery = 5 * time.Second

// orphanAfter is how long cliamp may stay unreachable before the daemon exits.
// The daemon only outlives cliamp when cliamp died without stopping its plugins.
const orphanAfter = time.Minute

// settleAfter is how long activity must stay unchanged before it is pushed.
// A held seek key sends an update per repeat; pushing each one can exceed
// Discord's update limits, so only the settled state is sent.
const settleAfter = time.Second

// defaultClientID is the project's Discord application. Its "cliamp" art asset
// is the fallback image for tracks without a cover.
const defaultClientID = "1558987895333724210"

func main() {
	sock := flag.String("sock", "", "cliamp IPC socket (default: cliamp's config dir)")
	flag.Parse()
	if *sock == "" {
		p, err := defaultSocket()
		if err != nil {
			log.Fatalf("socket path: %v", err)
		}
		*sock = p
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	d := newDiscordSink(defaultClientID)
	go d.run(ctx)

	cliampsub.Run(ctx, *sock, topic, cliampsub.Handlers{
		Data: func(raw json.RawMessage) {
			var snap presence.Snapshot
			if err := json.Unmarshal(raw, &snap); err != nil {
				log.Printf("bad playback snapshot: %v", err)
				return
			}
			d.set(presence.Activity(snap, time.Now()))
		},
		Down: func(err error) {
			// cliamp is gone or unreachable: clear the card so a stale track is
			// not shown after cliamp exits.
			log.Printf("cliamp socket down: %v", err)
			d.set(nil)
		},
		GiveUpAfter: orphanAfter,
	})
}

// discordSink keeps the latest wanted activity and pushes it to Discord. A
// failed push is retried every retryEvery, so a Discord restart is recovered
// without waiting for the next track event.
type discordSink struct {
	clientID string
	wake     chan struct{}

	mu     sync.Mutex
	want   *discord.Activity
	dirty  bool
	client *discord.Client
}

func newDiscordSink(clientID string) *discordSink {
	return &discordSink{clientID: clientID, wake: make(chan struct{}, 1)}
}

// set records the activity to show. It never blocks on Discord.
func (d *discordSink) set(a *discord.Activity) {
	d.mu.Lock()
	d.want = a
	d.dirty = true
	d.mu.Unlock()
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *discordSink) run(ctx context.Context) {
	tick := time.NewTicker(retryEvery)
	defer tick.Stop()
	defer d.close()
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.wake:
			if !d.awaitQuiet(ctx) {
				return
			}
		case <-tick.C:
		}
		d.flush()
	}
}

// awaitQuiet blocks until no new activity has been set for settleAfter. It
// returns false when ctx is done.
func (d *discordSink) awaitQuiet(ctx context.Context) bool {
	t := time.NewTimer(settleAfter)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-d.wake:
			if !t.Stop() {
				<-t.C
			}
			t.Reset(settleAfter)
		case <-t.C:
			return true
		}
	}
}

func (d *discordSink) flush() {
	d.mu.Lock()
	if !d.dirty {
		d.mu.Unlock()
		return
	}
	a := d.want
	d.mu.Unlock()

	if err := d.push(a); err != nil {
		log.Printf("discord: %v (retrying)", err)
		return
	}
	d.mu.Lock()
	// Only clear dirty if no newer activity arrived during the push.
	if d.want == a {
		d.dirty = false
	}
	d.mu.Unlock()
}

func (d *discordSink) push(a *discord.Activity) error {
	if d.client == nil {
		c, err := discord.Connect(d.clientID)
		if err != nil {
			return err
		}
		d.client = c
	}
	if _, err := d.client.SetActivity(a); err != nil {
		d.close()
		return err
	}
	return nil
}

func (d *discordSink) close() {
	if d.client != nil {
		d.client.Close()
		d.client = nil
	}
}

// defaultSocket mirrors cliamp's appdir.Dir lookup order so the daemon finds
// the same socket without importing cliamp's internal packages.
func defaultSocket() (string, error) {
	if dir := os.Getenv("CLIAMP_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "cliamp.sock"), nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cliamp", "cliamp.sock"), nil
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "cliamp", "cliamp.sock"), nil
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "cliamp", "cliamp.sock"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Join(errors.New("no config dir"), err)
	}
	return filepath.Join(home, ".config", "cliamp", "cliamp.sock"), nil
}
