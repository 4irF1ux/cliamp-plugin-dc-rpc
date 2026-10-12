// Command discord-spike is the phase 1 spike: handshake, SET_ACTIVITY, clear.
//
//	go run ./cmd/discord-spike -client-id 1558987895333724210 -hold 20s
//
// Disconnect check: run with -clear=false, then watch whether the card
// disappears after the process exits.
package main

import (
	"flag"
	"log"
	"time"

	"github.com/4irF1ux/cliamp-plugin-discord-rpc/internal/discord"
)

func main() {
	clientID := flag.String("client-id", "", "Discord application ID (required)")
	details := flag.String("details", "cliamp spike", "activity details line")
	state := flag.String("state", "IPC spike", "activity state line")
	hold := flag.Duration("hold", 20*time.Second, "how long to keep the activity visible")
	clearFirst := flag.Bool("clear", true, "send SET_ACTIVITY with null activity before exit")
	flag.Parse()

	if *clientID == "" {
		log.Fatal("-client-id is required")
	}

	c, err := discord.Connect(*clientID)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer c.Close()
	log.Printf("handshake ok; READY: %s", c.Ready)

	act := &discord.Activity{
		Details:    *details,
		State:      *state,
		Timestamps: &discord.Timestamps{Start: time.Now().Unix()},
	}
	reply, err := c.SetActivity(act)
	if err != nil {
		log.Fatalf("set activity: %v", err)
	}
	log.Printf("activity set; reply: %s", reply)
	log.Printf("holding %s", *hold)
	time.Sleep(*hold)

	if *clearFirst {
		reply, err := c.SetActivity(nil)
		if err != nil {
			log.Fatalf("clear activity: %v", err)
		}
		log.Printf("activity cleared; reply: %s", reply)
	}
}
