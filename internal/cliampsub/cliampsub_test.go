package cliampsub

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeCliamp serves one connection: it reads the subscribe request, answers
// with ack, then writes each line in events.
func fakeCliamp(t *testing.T, ack string, events ...string) (sock string, requested chan string) {
	t.Helper()
	// Unix socket paths are length-limited; t.TempDir can exceed the limit on Windows.
	dir, err := os.MkdirTemp("", "cs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock = filepath.Join(dir, "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	requested = make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		line, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			return
		}
		requested <- line
		conn.Write([]byte(ack + "\n"))
		for _, e := range events {
			conn.Write([]byte(e + "\n"))
		}
	}()
	return sock, requested
}

func TestRunDeliversOnlyTopicPayloads(t *testing.T) {
	const topic = "plugin.discord-rpc.playback"
	sock, requested := fakeCliamp(t, `{"version":2,"id":"events","ok":true}`,
		`{"event":"plugin.other.playback","seq":1,"time":1,"data":{"title":"wrong"}}`,
		`{"event":"plugin.discord-rpc.playback","seq":2,"time":2,"retained":true,"data":{"title":"right"}}`,
	)

	got := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Run(ctx, sock, topic, Handlers{
		Data: func(raw json.RawMessage) {
			select {
			case got <- string(raw):
			default:
			}
		},
	})

	select {
	case req := <-requested:
		var r struct {
			Method string   `json:"method"`
			Topics []string `json:"topics"`
		}
		if err := json.Unmarshal([]byte(req), &r); err != nil {
			t.Fatal(err)
		}
		if r.Method != "subscribe" || len(r.Topics) != 1 || r.Topics[0] != topic {
			t.Fatalf("request = %s, want subscribe to %q", req, topic)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no subscribe request")
	}

	select {
	case payload := <-got:
		if payload != `{"title":"right"}` {
			t.Fatalf("payload = %s, want the topic's payload only", payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no payload delivered")
	}
}

func TestRejectedSubscribeReportsDown(t *testing.T) {
	sock, _ := fakeCliamp(t, `{"version":2,"id":"events","ok":false,"error":"unknown topic"}`)

	down := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Run(ctx, sock, "plugin.x.playback", Handlers{
		Down: func(err error) {
			select {
			case down <- err:
			default:
			}
		},
	})

	select {
	case err := <-down:
		if err == nil {
			t.Fatal("Down called with nil error for rejected subscribe")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Down not called")
	}
}
