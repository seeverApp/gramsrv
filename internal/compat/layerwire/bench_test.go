package layerwire

import (
	"testing"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/tg"
)

func benchEncode(o bin.Encoder) []byte {
	var b bin.Buffer
	if err := o.Encode(&b); err != nil {
		panic(err)
	}
	return b.Copy()
}

// BenchmarkTranscodeOutbound measures the outbound seam: the 227 passthrough
// (the overwhelmingly common case) vs a real message downgrade to 220.
func BenchmarkTranscodeOutbound(b *testing.B) {
	richMessage := canonicalCorpus()[1]
	raw := benchEncode(richMessage)

	b.Run("identity_227", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := Transcode(raw, 227); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("downgrade_220_message", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := Transcode(raw, 220); err != nil {
				b.Fatal(err)
			}
		}
	})

	dialogs := benchEncode(canonicalCorpus()[11]) // messages.dialogs
	b.Run("downgrade_220_dialogs", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := Transcode(dialogs, 220); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkUpgradeInbound measures the inbound seam: a 227 client (miss, the
// common case), a CRC-swap drift, and a body-transform drift.
func BenchmarkUpgradeInbound(b *testing.B) {
	b.Run("miss_227", func(b *testing.B) {
		// A canonical method id that needs no upgrade.
		body := benchEncode(&tg.HelpGetConfigRequest{})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			in := &bin.Buffer{Buf: body}
			id, _ := in.PeekID()
			if _, ok, _ := UpgradeInbound(id, in); ok {
				b.Fatal("unexpected upgrade")
			}
		}
	})

	// uploadMedia body transform (peer+media -> flags+peer+media).
	var um bin.Buffer
	um.PutID(0x519bc2b1)
	_ = (&tg.InputPeerSelf{}).Encode(&um)
	_ = (&tg.InputMediaUploadedPhoto{File: &tg.InputFile{ID: 10, Parts: 1, Name: "a.jpg"}}).Encode(&um)
	umRaw := um.Copy()
	b.Run("drift_uploadMedia", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			in := &bin.Buffer{Buf: append([]byte(nil), umRaw...)}
			if _, ok, err := UpgradeInbound(0x519bc2b1, in); !ok || err != nil {
				b.Fatal(ok, err)
			}
		}
	})
}
