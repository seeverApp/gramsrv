package layerwire

import (
	"testing"

	"github.com/gotd/td/bin"
)

func mustEncode(t *testing.T, o bin.Encoder) []byte {
	t.Helper()
	var b bin.Buffer
	if err := o.Encode(&b); err != nil {
		t.Fatalf("encode %T: %v", o, err)
	}
	return b.Copy()
}

// TestWalkConsumesCanonicalObjects encodes a diverse corpus of canonical (gotd,
// Layer 227) objects and asserts the generic walker consumes every byte. Full
// consumption proves the layout handles each field's wire kind (flags,
// multi-flags, conditionals, vectors, nested boxed/bare objects) exactly as gotd
// encoded them.
func TestWalkConsumesCanonicalObjects(t *testing.T) {
	for _, o := range canonicalCorpus() {
		raw := mustEncode(t, o)
		b := &bin.Buffer{Buf: append([]byte(nil), raw...)}
		if err := canonical.skipObject(b); err != nil {
			t.Errorf("%T: walk error: %v", o, err)
			continue
		}
		if b.Len() != 0 {
			t.Errorf("%T: %d/%d bytes left after walk", o, b.Len(), len(raw))
		}
	}
}
