package mtprotoedge

import (
	"bytes"
	"testing"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/tg"
)

// TestConnDowngradedClone verifies the outbound seam downgrades a canonical
// (227) object to the connection's negotiated layer, is a no-op for 227, and
// — critically for push fan-out — never mutates the shared input message (one
// pre-encoded update is reused across many connections of differing layers).
func TestConnDowngradedClone(t *testing.T) {
	const (
		message227CRC = 0x7600b9d3
		message220CRC = 0xb92f76cf
	)
	msg := &tg.Message{
		ID:      2,
		FromID:  &tg.PeerUser{UserID: 3},
		PeerID:  &tg.PeerUser{UserID: 3},
		Date:    1,
		Message: "hi",
	}

	// layer 220: returns a NEW message rewritten to the 220 constructor id,
	// leaving the shared input untouched (227).
	enc, err := encodeOutboundMessage(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	c := &Conn{metrics: NopMetrics{}}
	c.SetClientLayer(220)
	out := c.downgradedClone(enc)

	if id, _ := (&bin.Buffer{Buf: out.body}).PeekID(); id != message220CRC {
		t.Fatalf("downgraded id = %#08x, want %#08x", id, message220CRC)
	}
	if out.typeID != message220CRC {
		t.Fatalf("downgraded typeID = %#08x, want %#08x", out.typeID, message220CRC)
	}
	// Input must be unmodified — this is what makes shared push fan-out safe.
	if id, _ := (&bin.Buffer{Buf: enc.body}).PeekID(); id != message227CRC {
		t.Fatalf("input message was mutated: id now %#08x, want 227 %#08x", id, message227CRC)
	}

	// Two connections sharing one pre-encoded message get independent results.
	encShared, _ := encodeOutboundMessage(msg)
	c220 := &Conn{metrics: NopMetrics{}}
	c220.SetClientLayer(220)
	c227 := &Conn{metrics: NopMetrics{}} // ClientLayer() defaults to 227
	out220 := c220.downgradedClone(encShared)
	out227 := c227.downgradedClone(encShared)
	if id, _ := (&bin.Buffer{Buf: out220.body}).PeekID(); id != message220CRC {
		t.Fatalf("shared->220 id = %#08x, want %#08x", id, message220CRC)
	}
	if out227 != encShared {
		t.Errorf("227 connection should pass the shared message through unchanged (same pointer)")
	}
	if !bytes.Equal(encShared.body, out227.body) {
		t.Errorf("227 passthrough altered bytes")
	}
}
