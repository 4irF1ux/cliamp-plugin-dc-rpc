// Package cliampsub subscribes to one plugin topic on cliamp's IPC socket and
// hands each event payload to the caller. It reconnects with bounded backoff.
//
// Protocol (docs/plugins.md, "Plugin event pub/sub"): send one NDJSON request,
// read an ack line, then one NDJSON event per line. Retained values arrive right
// after the ack, so a late subscriber gets the current state at once.
package cliampsub

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

const (
	dialTimeout = 3 * time.Second
	minBackoff  = time.Second
	maxBackoff  = 30 * time.Second
	// maxLine matches cliamp's 64 KiB payload cap plus the envelope.
	maxLine = 128 << 10
)

// Handlers receive the subscription's events. Both run on the Run goroutine.
type Handlers struct {
	// Data is called with each payload published on the topic.
	Data func(json.RawMessage)
	// Down is called when a connection attempt ends, with the reason. It is
	// called before every retry, so the caller can clear state it derived from
	// the stream.
	Down func(err error)
}

type envelope struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type ack struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

// Run keeps a subscription to topic on the socket at sock until ctx is done.
func Run(ctx context.Context, sock, topic string, h Handlers) {
	backoff := minBackoff
	for {
		subscribed, err := stream(ctx, sock, topic, h.Data)
		if ctx.Err() != nil {
			return
		}
		if h.Down != nil {
			h.Down(err)
		}
		if subscribed {
			backoff = minBackoff
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

// stream runs one connection. It reports whether the subscription was acked,
// and the error that ended the connection.
func stream(ctx context.Context, sock, topic string, data func(json.RawMessage)) (bool, error) {
	conn, err := net.DialTimeout("unix", sock, dialTimeout)
	if err != nil {
		return false, fmt.Errorf("dial cliamp socket: %w", err)
	}
	defer conn.Close()

	// A cancelled context closes the connection, which unblocks the scanner.
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	req, err := json.Marshal(map[string]any{
		"version": 2,
		"id":      "events",
		"method":  "subscribe",
		"topics":  []string{topic},
	})
	if err != nil {
		return false, err
	}
	if _, err := conn.Write(append(req, '\n')); err != nil {
		return false, fmt.Errorf("send subscribe: %w", err)
	}

	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)
	if !sc.Scan() {
		return false, scanErr(sc, "read ack")
	}
	var a ack
	if err := json.Unmarshal(sc.Bytes(), &a); err != nil {
		return false, fmt.Errorf("decode ack: %w", err)
	}
	if !a.OK {
		return false, fmt.Errorf("subscribe rejected: %s", a.Error)
	}

	for sc.Scan() {
		var ev envelope
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			return true, fmt.Errorf("decode event: %w", err)
		}
		if ev.Event == topic && data != nil {
			data(ev.Data)
		}
	}
	return true, scanErr(sc, "read event")
}

func scanErr(sc *bufio.Scanner, what string) error {
	if err := sc.Err(); err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return fmt.Errorf("%s: %w", what, errors.New("connection closed"))
}
