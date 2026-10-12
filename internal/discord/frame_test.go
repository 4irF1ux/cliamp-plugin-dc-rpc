package discord

import (
	"bytes"
	"testing"
)

func TestWriteFrameHeaderIsLittleEndian(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte(`{"v":1}`)
	if err := writeFrame(&buf, opHandshake, payload); err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 0, 0, 0, 7, 0, 0, 0}
	want = append(want, payload...)
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("frame = %x, want %x", buf.Bytes(), want)
	}
}

func TestReadFrameRoundTripAndSizeLimit(t *testing.T) {
	var buf bytes.Buffer
	if err := writeFrame(&buf, opFrame, []byte(`{"cmd":"DISPATCH"}`)); err != nil {
		t.Fatal(err)
	}
	op, body, err := readFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if op != opFrame || string(body) != `{"cmd":"DISPATCH"}` {
		t.Fatalf("got op=%d body=%q", op, body)
	}

	// Length 0x100001 = maxFrame+1 must be rejected before allocating.
	hdr := []byte{1, 0, 0, 0, 0x01, 0x00, 0x10, 0x00}
	if _, _, err := readFrame(bytes.NewReader(hdr)); err == nil {
		t.Fatal("expected error for oversized frame")
	}
}
