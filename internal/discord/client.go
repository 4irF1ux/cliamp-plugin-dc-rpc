// Package discord is a minimal client for Discord's local RPC over IPC.
//
// Wire format (see https://docs.discord.com/developers/topics/rpc#rpc-over-ipc):
// every message is an 8-byte header (opcode uint32 LE, payload length uint32 LE)
// followed by a JSON payload.
package discord

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	opHandshake uint32 = 0
	opFrame     uint32 = 1
	opClose     uint32 = 2
	opPing      uint32 = 3
	opPong      uint32 = 4

	// maxFrame bounds allocations when reading a frame header from the peer.
	maxFrame = 1 << 20

	maxIPCIndex = 10
)

// Client is an open, handshaken connection to the local Discord client.
type Client struct {
	conn  io.ReadWriteCloser
	nonce int
	// Ready is the data of the READY dispatch: the logged-in account and config.
	Ready json.RawMessage
}

// Activity is the subset of the Rich Presence activity used by the daemon.
// Without Type 2 (Listening) Discord shows "Playing".
type Activity struct {
	Type       int         `json:"type"`
	Details    string      `json:"details,omitempty"`
	State      string      `json:"state,omitempty"`
	Timestamps *Timestamps `json:"timestamps,omitempty"`
	Assets     *Assets     `json:"assets,omitempty"`
}

// Assets are the images shown on the card. LargeImage is either a public https
// URL that Discord fetches itself or the key of an app art asset.
type Assets struct {
	LargeImage string `json:"large_image,omitempty"`
}

// TypeListening is the Activity.Type for "Listening to".
const TypeListening = 2

// Timestamps are unix seconds. Start makes Discord show elapsed time; End
// makes it show remaining time.
type Timestamps struct {
	Start int64 `json:"start,omitempty"`
	End   int64 `json:"end,omitempty"`
}

// Connect finds the first available Discord IPC endpoint, performs the
// handshake for clientID, and waits for the READY dispatch.
func Connect(clientID string) (*Client, error) {
	conn, err := dialFirst()
	if err != nil {
		return nil, err
	}
	c := &Client{conn: conn}
	if err := c.handshake(clientID); err != nil {
		conn.Close()
		return nil, err
	}
	return c, nil
}

// SetActivity sends SET_ACTIVITY and returns Discord's reply data.
// A nil activity clears the presence.
func (c *Client) SetActivity(a *Activity) (json.RawMessage, error) {
	c.nonce++
	args := struct {
		PID      int       `json:"pid"`
		Activity *Activity `json:"activity"`
	}{PID: os.Getpid(), Activity: a}
	payload, err := json.Marshal(struct {
		Cmd   string `json:"cmd"`
		Args  any    `json:"args"`
		Nonce string `json:"nonce"`
	}{Cmd: "SET_ACTIVITY", Args: args, Nonce: strconv.Itoa(c.nonce)})
	if err != nil {
		return nil, err
	}
	if err := writeFrame(c.conn, opFrame, payload); err != nil {
		return nil, fmt.Errorf("send SET_ACTIVITY: %w", err)
	}
	return c.awaitReply(strconv.Itoa(c.nonce))
}

// Close closes the connection. Discord removes the presence once the
// connection drops; verified on Windows, see docs/plan.md.
func (c *Client) Close() error {
	return c.conn.Close()
}

type message struct {
	Cmd   string          `json:"cmd"`
	Evt   string          `json:"evt"`
	Nonce string          `json:"nonce"`
	Data  json.RawMessage `json:"data"`
}

func (c *Client) handshake(clientID string) error {
	payload, err := json.Marshal(struct {
		V        int    `json:"v"`
		ClientID string `json:"client_id"`
	}{V: 1, ClientID: clientID})
	if err != nil {
		return err
	}
	if err := writeFrame(c.conn, opHandshake, payload); err != nil {
		return fmt.Errorf("send handshake: %w", err)
	}
	for {
		op, body, err := readFrame(c.conn)
		if err != nil {
			return fmt.Errorf("read READY: %w", err)
		}
		if op != opFrame {
			return fmt.Errorf("handshake: unexpected opcode %d: %s", op, body)
		}
		var m message
		if err := json.Unmarshal(body, &m); err != nil {
			return fmt.Errorf("handshake: decode: %w", err)
		}
		if m.Evt == "READY" {
			c.Ready = m.Data
			return nil
		}
		if m.Evt == "ERROR" {
			return fmt.Errorf("handshake rejected: %s", m.Data)
		}
	}
}

// awaitReply reads frames until the reply with the given nonce arrives and
// returns its data. Dispatch events without a matching nonce are skipped.
func (c *Client) awaitReply(nonce string) (json.RawMessage, error) {
	for {
		op, body, err := readFrame(c.conn)
		if err != nil {
			return nil, fmt.Errorf("read reply: %w", err)
		}
		switch op {
		case opPing:
			if err := writeFrame(c.conn, opPong, body); err != nil {
				return nil, fmt.Errorf("send pong: %w", err)
			}
			continue
		case opClose:
			return nil, fmt.Errorf("discord closed the connection: %s", body)
		case opFrame:
		default:
			continue
		}
		var m message
		if err := json.Unmarshal(body, &m); err != nil {
			return nil, fmt.Errorf("decode reply: %w", err)
		}
		if m.Nonce != nonce {
			continue
		}
		if m.Evt == "ERROR" {
			return nil, fmt.Errorf("discord error: %s", m.Data)
		}
		return m.Data, nil
	}
}

func writeFrame(w io.Writer, op uint32, payload []byte) error {
	buf := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint32(buf[0:4], op)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(payload)))
	copy(buf[8:], payload)
	// One Write so a frame is not interleaved with another writer's bytes.
	_, err := w.Write(buf)
	return err
}

func readFrame(r io.Reader) (uint32, []byte, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	op := binary.LittleEndian.Uint32(hdr[0:4])
	n := binary.LittleEndian.Uint32(hdr[4:8])
	if n > maxFrame {
		return 0, nil, fmt.Errorf("frame too large: %d bytes", n)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return op, payload, nil
}

// dialFirst tries discord-ipc-0 .. discord-ipc-9 and returns the first that opens.
func dialFirst() (io.ReadWriteCloser, error) {
	var errs []string
	for i := range maxIPCIndex {
		conn, err := dialIPC(i)
		if err == nil {
			return conn, nil
		}
		errs = append(errs, err.Error())
	}
	return nil, fmt.Errorf("no Discord IPC endpoint (is the Discord desktop app running?): %s",
		strings.Join(errs, "; "))
}

func dialIPC(index int) (io.ReadWriteCloser, error) {
	name := "discord-ipc-" + strconv.Itoa(index)
	if runtime.GOOS == "windows" {
		return os.OpenFile(`\\.\pipe\`+name, os.O_RDWR, 0)
	}
	var lastErr error = errors.New("no unix socket dir")
	for _, dir := range unixDirs() {
		conn, err := net.Dial("unix", filepath.Join(dir, name))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// unixDirs mirrors the lookup order documented by Discord for Linux/macOS.
func unixDirs() []string {
	var dirs []string
	for _, env := range []string{"XDG_RUNTIME_DIR", "TMPDIR", "TMP", "TEMP"} {
		if v := os.Getenv(env); v != "" {
			dirs = append(dirs, v)
		}
	}
	return append(dirs, "/tmp")
}
